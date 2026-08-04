package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	platformci "github.com/flidai/leapview/internal/platform/ci"
)

type githubRun struct {
	ID         int64     `json:"id"`
	Event      string    `json:"event"`
	Attempt    int       `json:"run_attempt"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Conclusion string    `json:"conclusion"`
}

type githubJob struct {
	Name       string     `json:"name"`
	Conclusion string     `json:"conclusion"`
	StartedAt  *time.Time `json:"started_at"`
}

type githubArtifact struct {
	Name               string `json:"name"`
	ArchiveDownloadURL string `json:"archive_download_url"`
	Expired            bool   `json:"expired"`
}

type client struct {
	http  *http.Client
	token string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	repo := flag.String("repo", os.Getenv("GITHUB_REPOSITORY"), "owner/repository")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub token")
	days := flag.Int("days", 7, "reporting window in days")
	output := flag.String("output", "ci-health.json", "health report JSON")
	summary := flag.String("summary", "", "Markdown summary output")
	flag.Parse()
	if *repo == "" || *token == "" {
		return errors.New("--repo and --token are required")
	}
	if *days < 1 || *days > 30 {
		return errors.New("--days must be between 1 and 30")
	}

	since := time.Now().UTC().Add(-time.Duration(*days) * 24 * time.Hour)
	api := &client{http: &http.Client{Timeout: 30 * time.Second}, token: *token}
	runs, err := api.healthRuns(context.Background(), *repo, since)
	if err != nil {
		return err
	}
	report := platformci.AnalyzeHealth(runs)
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*output, append(reportJSON, '\n'), 0o644); err != nil {
		return err
	}
	markdown := renderMarkdown(report, *days)
	if *summary != "" {
		if err := os.WriteFile(*summary, []byte(markdown), 0o644); err != nil {
			return err
		}
	}
	fmt.Print(markdown)
	return nil
}

func (c *client) healthRuns(ctx context.Context, repo string, since time.Time) ([]platformci.HealthRun, error) {
	var listedRuns []githubRun
	for _, workflow := range []string{"ci.yml", "merge-validation.yml"} {
		for page := 1; page <= 10; page++ {
			endpoint := fmt.Sprintf(
				"https://api.github.com/repos/%s/actions/workflows/%s/runs?per_page=100&page=%d&created=%%3E%%3D%s",
				repo,
				workflow,
				page,
				since.Format("2006-01-02"),
			)
			var response struct {
				Runs []githubRun `json:"workflow_runs"`
			}
			if err := c.getJSON(ctx, endpoint, &response); err != nil {
				return nil, fmt.Errorf("list %s runs page %d: %w", workflow, page, err)
			}
			listedRuns = append(listedRuns, response.Runs...)
			if len(response.Runs) < 100 {
				break
			}
		}
	}

	var candidates []githubRun
	for _, run := range listedRuns {
		if run.Conclusion == "" || run.CreatedAt.Before(since) {
			continue
		}
		candidates = append(candidates, run)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	work := make(chan githubRun)
	var (
		runs     []platformci.HealthRun
		firstErr error
		mutex    sync.Mutex
		group    sync.WaitGroup
	)
	workers := 8
	if len(candidates) < workers {
		workers = len(candidates)
	}
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for run := range work {
				healthRun, err := c.healthRun(ctx, repo, run)
				mutex.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
					cancel()
				}
				if err == nil {
					runs = append(runs, healthRun)
				}
				mutex.Unlock()
			}
		}()
	}
	for _, run := range candidates {
		select {
		case work <- run:
		case <-ctx.Done():
			break
		}
	}
	close(work)
	group.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].ID < runs[j].ID })
	return runs, nil
}

func (c *client) healthRun(ctx context.Context, repo string, run githubRun) (platformci.HealthRun, error) {
	jobs, err := c.jobs(ctx, repo, run.ID)
	if err != nil {
		return platformci.HealthRun{}, err
	}
	results := jobResults(jobs)
	plan, err := c.plan(ctx, repo, run.ID)
	if err != nil {
		return platformci.HealthRun{}, err
	}
	if plan.Version == 0 {
		plan = inferPlan(results)
	}
	queue := int64(-1)
	if started := earliestStart(jobs); run.Attempt <= 1 && !started.IsZero() && started.After(run.CreatedAt) {
		queue = int64(started.Sub(run.CreatedAt).Seconds())
	}
	return platformci.HealthRun{
		ID:              run.ID,
		Event:           run.Event,
		Attempt:         run.Attempt,
		Conclusion:      run.Conclusion,
		DurationSeconds: int64(run.UpdatedAt.Sub(run.CreatedAt).Seconds()),
		QueueSeconds:    queue,
		Deferred:        deferredStackRun(jobs),
		Plan:            plan,
		Results:         results,
	}, nil
}

func deferredStackRun(jobs []githubJob) bool {
	conclusions := make(map[string]string, len(jobs))
	for _, job := range jobs {
		conclusions[job.Name] = job.Conclusion
	}
	return conclusions["Autback preflight"] == "skipped" &&
		conclusions["GitHub CI (external pull request)"] == "skipped" &&
		conclusions["CI gate"] == "success"
}

func (c *client) jobs(ctx context.Context, repo string, runID int64) ([]githubJob, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d/jobs?per_page=100", repo, runID)
	var response struct {
		Jobs []githubJob `json:"jobs"`
	}
	if err := c.getJSON(ctx, endpoint, &response); err != nil {
		return nil, fmt.Errorf("list jobs for run %d: %w", runID, err)
	}
	return response.Jobs, nil
}

func (c *client) plan(ctx context.Context, repo string, runID int64) (platformci.Plan, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d/artifacts?per_page=100", repo, runID)
	var response struct {
		Artifacts []githubArtifact `json:"artifacts"`
	}
	if err := c.getJSON(ctx, endpoint, &response); err != nil {
		return platformci.Plan{}, fmt.Errorf("list artifacts for run %d: %w", runID, err)
	}
	for _, artifact := range response.Artifacts {
		if artifact.Name != "ci-plan" || artifact.Expired {
			continue
		}
		data, err := c.get(ctx, artifact.ArchiveDownloadURL)
		if err != nil {
			return platformci.Plan{}, fmt.Errorf("download plan for run %d: %w", runID, err)
		}
		return decodePlanArchive(data)
	}
	return platformci.Plan{}, nil
}

func (c *client) getJSON(ctx context.Context, endpoint string, destination any) error {
	data, err := c.get(ctx, endpoint)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, destination)
}

func (c *client) get(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %s: %s", endpoint, response.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func decodePlanArchive(data []byte) (platformci.Plan, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return platformci.Plan{}, err
	}
	for _, file := range archive.File {
		if file.Name != "ci-plan.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return platformci.Plan{}, err
		}
		defer reader.Close()
		var plan platformci.Plan
		if err := json.NewDecoder(reader).Decode(&plan); err != nil {
			return platformci.Plan{}, err
		}
		return plan, nil
	}
	return platformci.Plan{}, errors.New("ci-plan.json missing from artifact")
}

func jobResults(jobs []githubJob) map[string]string {
	results := map[string]string{}
	for _, job := range jobs {
		name := normalizedJobName(job.Name)
		if name == "" {
			continue
		}
		results[name] = combineConclusion(results[name], job.Conclusion)
	}
	return results
}

func normalizedJobName(name string) string {
	switch {
	case name == "Prepare generated assets":
		return "prepare"
	case name == "Prepare frontend assets":
		return "frontend-prepare"
	case name == "Documentation and public site":
		return "docs"
	case strings.HasPrefix(name, "Go tests ("):
		return "go-tests"
	case name == "Public site image", name == "Public site image (external pull request)":
		return "site-image"
	case strings.HasPrefix(name, "Frontend tests ("):
		return "frontend-tests"
	case name == "Go static and race analysis":
		return "go-analysis"
	case name == "UI route QA":
		return "ui-route-qa"
	case name == "JavaScript dependency audit":
		return "node-audit"
	case name == "Go vulnerability scan":
		return "go-vuln"
	case name == "Production image", name == "Production image (external pull request)":
		return "production-image"
	case name == "Deployment contracts":
		return "deployment-contracts"
	default:
		return ""
	}
}

func combineConclusion(current, next string) string {
	rank := map[string]int{"": 0, "skipped": 1, "success": 2, "cancelled": 3, "failure": 4}
	if rank[next] > rank[current] {
		return next
	}
	return current
}

func inferPlan(results map[string]string) platformci.Plan {
	jobs := platformci.FullJobs()
	return platformci.Plan{Version: platformci.PlanVersion, Reason: "legacy full-CI run", Nominal: jobs, Effective: jobs}
}

func earliestStart(jobs []githubJob) time.Time {
	var earliest time.Time
	for _, job := range jobs {
		if job.StartedAt == nil {
			continue
		}
		if earliest.IsZero() || job.StartedAt.Before(earliest) {
			earliest = *job.StartedAt
		}
	}
	return earliest
}

func renderMarkdown(report platformci.HealthReport, days int) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# CI health — trailing %d days\n\n", days)
	fmt.Fprintf(&output, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(&output, "| Runs | %d |\n", report.RunCount)
	fmt.Fprintf(&output, "| Success / failure / cancelled | %d / %d / %d |\n", report.Successes, report.Failures, report.Cancellations)
	fmt.Fprintf(&output, "| Deferred stack layers | %d |\n", report.Deferred)
	fmt.Fprintf(&output, "| Full CI p50 / p95 | %s / %s |\n", formatSeconds(report.Full.P50Seconds), formatSeconds(report.Full.P95Seconds))
	fmt.Fprintf(&output, "| Selective PR p50 / p95 | %s / %s |\n", formatSeconds(report.Selective.P50Seconds), formatSeconds(report.Selective.P95Seconds))
	fmt.Fprintf(&output, "| Queue p50 / p95 | %s / %s |\n", formatSeconds(report.Queue.P50Seconds), formatSeconds(report.Queue.P95Seconds))
	fmt.Fprintf(&output, "| Reruns | %.1f%% |\n", report.RerunPercent)
	fmt.Fprintf(&output, "| Selection audit misses | %d |\n", report.AuditMisses)
	output.WriteString("\n")

	output.WriteString("## Job selection\n\n| Job | Selected | Rate |\n|---|---:|---:|\n")
	names := make([]string, 0, len(report.Selection))
	for name := range report.Selection {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		metric := report.Selection[name]
		fmt.Fprintf(&output, "| %s | %d | %.1f%% |\n", name, metric.Selected, metric.Percent)
	}
	if len(report.Alerts) > 0 {
		output.WriteString("\n## Alerts\n\n")
		for _, alert := range report.Alerts {
			fmt.Fprintf(&output, "- %s\n", alert)
		}
	} else {
		output.WriteString("\nNo CI health thresholds were exceeded.\n")
	}
	return output.String()
}

func formatSeconds(seconds int64) string {
	return (time.Duration(seconds) * time.Second).String()
}
