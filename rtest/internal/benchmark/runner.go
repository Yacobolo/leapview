// Package benchmark runs controlled, repeatable command comparisons.
package benchmark

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
)

type Spec struct {
	SchemaVersion   int         `json:"schema_version"`
	Name            string      `json:"name"`
	ProjectDir      string      `json:"project_dir"`
	WarmupRuns      int         `json:"warmup_runs"`
	MeasuredRuns    int         `json:"measured_runs"`
	RequireClean    bool        `json:"require_clean"`
	IsolateWorktree bool        `json:"isolate_worktree"`
	Arguments       []string    `json:"arguments"`
	Candidates      []Candidate `json:"candidates"`
}

type Candidate struct {
	Name           string   `json:"name"`
	Prefix         []string `json:"prefix"`
	VersionCommand []string `json:"version_command,omitempty"`
	RequiredEnv    []string `json:"required_env,omitempty"`
	Required       bool     `json:"required,omitempty"`
}

type Summary struct {
	SchemaVersion       int               `json:"schema_version"`
	Name                string            `json:"name"`
	Commit              string            `json:"commit"`
	WorktreeFingerprint string            `json:"worktree_fingerprint"`
	Clean               bool              `json:"clean"`
	CompletedAt         string            `json:"completed_at"`
	Host                Host              `json:"host"`
	WarmupRuns          int               `json:"warmup_runs"`
	MeasuredRuns        int               `json:"measured_runs"`
	IsolatedWorktrees   bool              `json:"isolated_worktrees"`
	Arguments           []string          `json:"arguments"`
	Candidates          []CandidateResult `json:"candidates"`
}

type Host struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type CandidateResult struct {
	Name        string       `json:"name"`
	Status      string       `json:"status"`
	Reason      string       `json:"reason,omitempty"`
	Command     []string     `json:"command"`
	Version     string       `json:"version,omitempty"`
	Runs        []RunResult  `json:"runs"`
	WallSeconds Distribution `json:"wall_seconds"`
}

type RunResult struct {
	Phase       string  `json:"phase"`
	Index       int     `json:"index"`
	WallSeconds float64 `json:"wall_seconds"`
	ExitCode    int     `json:"exit_code"`
	Log         string  `json:"log"`
}

type Distribution struct {
	Values []float64 `json:"values"`
	Min    float64   `json:"min"`
	Median float64   `json:"median"`
	P95    float64   `json:"p95"`
	Max    float64   `json:"max"`
}

var candidateName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func Run(ctx context.Context, spec Spec, outputDirectory string) (Summary, error) {
	return RunWithProgress(ctx, spec, outputDirectory, io.Discard)
}

func RunWithProgress(ctx context.Context, spec Spec, outputDirectory string, progress io.Writer) (Summary, error) {
	if err := validate(spec); err != nil {
		return Summary{}, err
	}
	project, err := filepath.Abs(spec.ProjectDir)
	if err != nil {
		return Summary{}, fmt.Errorf("resolve benchmark project: %w", err)
	}
	commit, err := gitOutput(ctx, project, "rev-parse", "HEAD")
	if err != nil {
		return Summary{}, err
	}
	status, err := gitBytes(ctx, project, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return Summary{}, err
	}
	clean := len(status) == 0
	if spec.RequireClean && !clean {
		return Summary{}, errors.New("benchmark requires a clean project worktree")
	}
	fingerprint, err := fingerprint(ctx, project)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		SchemaVersion:       1,
		Name:                spec.Name,
		Commit:              strings.TrimSpace(commit),
		WorktreeFingerprint: fingerprint,
		Clean:               clean,
		Host:                Host{OS: runtime.GOOS, Arch: runtime.GOARCH},
		WarmupRuns:          spec.WarmupRuns,
		MeasuredRuns:        spec.MeasuredRuns,
		IsolatedWorktrees:   spec.IsolateWorktree,
		Arguments:           expand(spec.Arguments),
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return Summary{}, fmt.Errorf("create benchmark output: %w", err)
	}

	for _, candidate := range spec.Candidates {
		result := CandidateResult{Name: candidate.Name, Status: "pending"}
		prefix := expand(candidate.Prefix)
		result.Command = append(slices.Clone(prefix), summary.Arguments...)
		reason := unavailable(candidate, prefix)
		if reason != "" {
			result.Status, result.Reason = "unavailable", reason
			summary.Candidates = append(summary.Candidates, result)
			if candidate.Required {
				summary.CompletedAt = time.Now().UTC().Format(time.RFC3339)
				_ = writeSummary(outputDirectory, summary)
				return summary, fmt.Errorf("required candidate %q is unavailable: %s", candidate.Name, reason)
			}
			continue
		}
		if len(candidate.VersionCommand) > 0 {
			version, versionErr := commandOutput(ctx, project, expand(candidate.VersionCommand))
			if versionErr != nil {
				result.Status, result.Reason = "unavailable", "version command failed: "+versionErr.Error()
				summary.Candidates = append(summary.Candidates, result)
				if candidate.Required {
					return summary, fmt.Errorf("required candidate %q version: %w", candidate.Name, versionErr)
				}
				continue
			}
			result.Version = strings.TrimSpace(version)
		}

		candidateDirectory := filepath.Join(outputDirectory, candidate.Name)
		if err := os.MkdirAll(candidateDirectory, 0o755); err != nil {
			return summary, err
		}
		for index := 1; index <= spec.WarmupRuns+spec.MeasuredRuns; index++ {
			phase, phaseIndex := "warmup", index
			if index > spec.WarmupRuns {
				phase, phaseIndex = "measured", index-spec.WarmupRuns
			}
			logName := fmt.Sprintf("%s-%d.log", phase, phaseIndex)
			logPath := filepath.Join(candidateDirectory, logName)
			fmt.Fprintf(progress, "%s %s %d: running\n", candidate.Name, phase, phaseIndex)
			runDirectory, cleanup, prepareErr := prepareRunDirectory(ctx, project, summary.Commit, spec.IsolateWorktree)
			if prepareErr != nil {
				return summary, prepareErr
			}
			run, runErr := runOnce(ctx, runDirectory, result.Command, logPath)
			cleanupErr := cleanup()
			if runErr == nil && cleanupErr != nil {
				runErr = cleanupErr
			}
			run.Phase, run.Index, run.Log = phase, phaseIndex, filepath.ToSlash(filepath.Join(candidate.Name, logName))
			fmt.Fprintf(progress, "%s %s %d: %.3fs\n", candidate.Name, phase, phaseIndex, run.WallSeconds)
			result.Runs = append(result.Runs, run)
			if runErr != nil {
				result.Status, result.Reason = "failed", runErr.Error()
				summary.Candidates = append(summary.Candidates, result)
				summary.CompletedAt = time.Now().UTC().Format(time.RFC3339)
				_ = writeSummary(outputDirectory, summary)
				return summary, fmt.Errorf("candidate %q %s %d failed: %w", candidate.Name, phase, phaseIndex, runErr)
			}
		}
		values := make([]float64, 0, spec.MeasuredRuns)
		for _, run := range result.Runs {
			if run.Phase == "measured" {
				values = append(values, run.WallSeconds)
			}
		}
		result.Status = "completed"
		result.WallSeconds = distribution(values)
		summary.Candidates = append(summary.Candidates, result)
	}
	summary.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeSummary(outputDirectory, summary); err != nil {
		return summary, err
	}
	return summary, nil
}

func prepareRunDirectory(ctx context.Context, project, commit string, isolate bool) (string, func() error, error) {
	if !isolate {
		return project, func() error { return nil }, nil
	}
	root, err := os.MkdirTemp("", "rtest-benchmark-worktree-")
	if err != nil {
		return "", nil, err
	}
	worktree := filepath.Join(root, "worktree")
	command := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", "--quiet", worktree, commit)
	command.Dir = project
	if output, addErr := command.CombinedOutput(); addErr != nil {
		os.RemoveAll(root)
		return "", nil, fmt.Errorf("create isolated benchmark worktree: %w: %s", addErr, output)
	}
	cleanup := func() error {
		remove := exec.Command("git", "worktree", "remove", "--force", worktree)
		remove.Dir = project
		output, removeErr := remove.CombinedOutput()
		filesystemErr := os.RemoveAll(root)
		if removeErr != nil {
			return fmt.Errorf("remove isolated benchmark worktree: %w: %s", removeErr, output)
		}
		return filesystemErr
	}
	return worktree, cleanup, nil
}

func validate(spec Spec) error {
	if spec.SchemaVersion != 1 {
		return fmt.Errorf("unsupported benchmark schema version %d", spec.SchemaVersion)
	}
	if !candidateName.MatchString(spec.Name) {
		return errors.New("benchmark name must contain only letters, numbers, dot, dash, and underscore")
	}
	if spec.ProjectDir == "" {
		return errors.New("benchmark project_dir is required")
	}
	if spec.WarmupRuns < 0 || spec.MeasuredRuns < 1 {
		return errors.New("benchmark needs non-negative warmup_runs and at least one measured_run")
	}
	if len(spec.Candidates) == 0 {
		return errors.New("benchmark needs at least one candidate")
	}
	seen := map[string]bool{}
	for _, candidate := range spec.Candidates {
		if !candidateName.MatchString(candidate.Name) || len(candidate.Prefix) == 0 {
			return fmt.Errorf("invalid benchmark candidate %q", candidate.Name)
		}
		if seen[candidate.Name] {
			return fmt.Errorf("duplicate benchmark candidate %q", candidate.Name)
		}
		seen[candidate.Name] = true
	}
	return nil
}

func unavailable(candidate Candidate, prefix []string) string {
	for _, name := range candidate.RequiredEnv {
		if value, ok := os.LookupEnv(name); !ok || value == "" {
			return "required environment variable " + name + " is not set"
		}
	}
	if _, err := exec.LookPath(prefix[0]); err != nil {
		return err.Error()
	}
	return ""
}

func expand(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = os.ExpandEnv(value)
	}
	return result
}

func runOnce(ctx context.Context, directory string, arguments []string, logPath string) (RunResult, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return RunResult{}, err
	}
	defer logFile.Close()
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Dir, command.Stdout, command.Stderr = directory, logFile, logFile
	started := time.Now()
	err = command.Run()
	elapsed := time.Since(started).Seconds()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			exitCode = exit.ExitCode()
		}
	}
	return RunResult{WallSeconds: elapsed, ExitCode: exitCode}, err
}

func commandOutput(ctx context.Context, directory string, arguments []string) (string, error) {
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	return string(output), err
}

func distribution(values []float64) Distribution {
	sorted := slices.Clone(values)
	sort.Float64s(sorted)
	result := Distribution{Values: slices.Clone(values)}
	if len(sorted) == 0 {
		return result
	}
	result.Min, result.Max = sorted[0], sorted[len(sorted)-1]
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		result.Median = (sorted[middle-1] + sorted[middle]) / 2
	} else {
		result.Median = sorted[middle]
	}
	p95Index := (len(sorted)*95 + 99) / 100
	result.P95 = sorted[p95Index-1]
	return result
}

func fingerprint(ctx context.Context, directory string) (string, error) {
	paths, err := gitBytes(ctx, directory, "ls-files", "-co", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	items := bytes.Split(bytes.TrimSuffix(paths, []byte{0}), []byte{0})
	slices.SortFunc(items, bytes.Compare)
	hash := sha256.New()
	for _, item := range items {
		if len(item) == 0 {
			continue
		}
		hash.Write(item)
		hash.Write([]byte{0})
		path := filepath.Join(directory, filepath.FromSlash(string(item)))
		info, statErr := os.Lstat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				hash.Write([]byte("<deleted>"))
				continue
			}
			return "", statErr
		}
		fmt.Fprintf(hash, "%s\x00", info.Mode())
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(path)
			if readErr != nil {
				return "", readErr
			}
			hash.Write([]byte(target))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return "", openErr
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func gitOutput(ctx context.Context, directory string, arguments ...string) (string, error) {
	output, err := gitBytes(ctx, directory, arguments...)
	return string(output), err
}

func gitBytes(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(arguments, " "), err)
	}
	return output, nil
}

func writeSummary(directory string, summary Summary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(directory, "summary-*.json")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(directory, "summary.json"))
}
