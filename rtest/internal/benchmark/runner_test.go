package benchmark

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunMeasuresAvailableCandidatesAndLabelsMissingOnes(t *testing.T) {
	project := gitProject(t)
	output := filepath.Join(t.TempDir(), "results")
	spec := Spec{
		SchemaVersion: 1,
		Name:          "controlled",
		ProjectDir:    project,
		WarmupRuns:    1,
		MeasuredRuns:  2,
		Arguments:     []string{"measured"},
		Candidates: []Candidate{
			{Name: "local", Prefix: []string{"sh", "-c", "printf '%s\\n' \"$1\"", "benchmark"}},
			{Name: "missing", Prefix: []string{"rtest-benchmark-command-that-does-not-exist"}},
		},
	}

	summary, err := Run(t.Context(), spec, output)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Commit == "" || summary.WorktreeFingerprint == "" {
		t.Fatalf("summary does not identify exact source state: %#v", summary)
	}
	if got := summary.Candidates[0]; got.Status != "completed" || len(got.Runs) != 3 || got.WallSeconds.Median < 0 {
		t.Fatalf("available candidate = %#v", got)
	}
	if got := summary.Candidates[1]; got.Status != "unavailable" || got.Reason == "" {
		t.Fatalf("missing candidate = %#v", got)
	}
	data, err := os.ReadFile(filepath.Join(output, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted Summary
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Name != spec.Name {
		t.Fatalf("persisted name = %q, want %q", persisted.Name, spec.Name)
	}
	if _, err := os.Stat(filepath.Join(output, "local", "measured-2.log")); err != nil {
		t.Fatalf("raw log was not preserved: %v", err)
	}
}

func TestRunRejectsDirtyProjectWhenCleanSourceIsRequired(t *testing.T) {
	project := gitProject(t)
	if err := os.WriteFile(filepath.Join(project, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(t.Context(), Spec{
		SchemaVersion: 1,
		Name:          "clean-only",
		ProjectDir:    project,
		MeasuredRuns:  1,
		RequireClean:  true,
		Candidates:    []Candidate{{Name: "local", Prefix: []string{"true"}}},
	}, t.TempDir())
	if err == nil {
		t.Fatal("dirty benchmark project unexpectedly accepted")
	}
}

func TestRunCanIsolateEverySampleInAFreshDetachedWorktree(t *testing.T) {
	project := gitProject(t)
	summary, err := Run(t.Context(), Spec{
		SchemaVersion:   1,
		Name:            "isolated",
		ProjectDir:      project,
		WarmupRuns:      1,
		MeasuredRuns:    2,
		RequireClean:    true,
		IsolateWorktree: true,
		Candidates: []Candidate{{
			Name: "local",
			Prefix: []string{
				"sh", "-c", "test ! -e generated && touch generated",
			},
		}},
	}, filepath.Join(t.TempDir(), "results"))
	if err != nil {
		t.Fatal(err)
	}
	if !summary.IsolatedWorktrees {
		t.Fatal("summary does not record isolated worktrees")
	}
	if _, err := os.Stat(filepath.Join(project, "generated")); !os.IsNotExist(err) {
		t.Fatalf("benchmark mutated source project: %v", err)
	}
}

func TestRunFailsOnRequiredUnavailableCandidate(t *testing.T) {
	project := gitProject(t)
	_, err := Run(t.Context(), Spec{
		SchemaVersion: 1,
		Name:          "required",
		ProjectDir:    project,
		MeasuredRuns:  1,
		Candidates: []Candidate{{
			Name: "remote", Prefix: []string{"missing-required-command"}, Required: true,
		}},
	}, t.TempDir())
	if err == nil {
		t.Fatal("required unavailable candidate unexpectedly accepted")
	}
}

func TestDistributionUsesMedianAndNearestRankP95(t *testing.T) {
	got := distribution([]float64{5, 1, 4, 2, 3})
	if got.Min != 1 || got.Median != 3 || got.P95 != 5 || got.Max != 5 {
		t.Fatalf("distribution = %#v", got)
	}
}

func gitProject(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	runCommand(t, directory, "git", "init", "--quiet")
	runCommand(t, directory, "git", "config", "user.email", "benchmark@example.test")
	runCommand(t, directory, "git", "config", "user.name", "Benchmark Test")
	if err := os.WriteFile(filepath.Join(directory, "tracked.txt"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, directory, "git", "add", "tracked.txt")
	runCommand(t, directory, "git", "commit", "--quiet", "-m", "source")
	return directory
}

func runCommand(t *testing.T, directory, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%v failed: %v\n%s", command.Args, err, output)
	}
}
