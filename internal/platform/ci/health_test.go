package ci

import (
	"slices"
	"testing"
)

func TestAnalyzeHealth(t *testing.T) {
	t.Parallel()

	full := FullJobs()
	selective := Jobs{Docs: true, SiteImage: true}
	runs := []HealthRun{
		{
			Event: "push", DurationSeconds: 600, QueueSeconds: 20, Conclusion: "success",
			Plan:    Plan{Nominal: full, Effective: full},
			Results: successfulResults(full),
		},
		{
			Event: "push", DurationSeconds: 700, QueueSeconds: 30, Conclusion: "success",
			Plan:    Plan{Nominal: full, Effective: full},
			Results: successfulResults(full),
		},
		{
			Event: "push", DurationSeconds: 800, QueueSeconds: 140, Conclusion: "failure",
			Plan:    Plan{Nominal: full, Effective: full},
			Results: successfulResults(full),
		},
		{
			Event: "pull_request", DurationSeconds: 240, QueueSeconds: 10, Conclusion: "success",
			Plan:    Plan{Nominal: selective, Effective: selective},
			Results: successfulResults(selective),
		},
		{
			Event: "pull_request", DurationSeconds: 300, QueueSeconds: 15, Conclusion: "success", Attempt: 2,
			Plan: Plan{Nominal: selective, Effective: full, Audit: true},
			Results: func() map[string]string {
				results := successfulResults(full)
				results["production-image"] = "failure"
				return results
			}(),
		},
	}
	got := AnalyzeHealth(runs)
	if got.RunCount != 5 {
		t.Fatalf("run count = %d, want 5", got.RunCount)
	}
	if got.Full.P95Seconds != 800 {
		t.Fatalf("full p95 = %d, want 800", got.Full.P95Seconds)
	}
	if got.Selective.P95Seconds != 240 {
		t.Fatalf("selective p95 = %d, want 240", got.Selective.P95Seconds)
	}
	if got.Queue.P95Seconds != 140 {
		t.Fatalf("queue p95 = %d, want 140", got.Queue.P95Seconds)
	}
	if got.RerunPercent != 20 {
		t.Fatalf("rerun percentage = %.1f, want 20", got.RerunPercent)
	}
	if got.AuditMisses != 1 {
		t.Fatalf("audit misses = %d, want 1", got.AuditMisses)
	}
	for _, alert := range []string{
		"full CI p95 is 13m20s (limit 12m0s)",
		"queue p95 is 2m20s (limit 2m0s)",
		"rerun rate is 20.0% (limit 3.0%)",
		"selection audit detected 1 miss",
	} {
		if !slices.Contains(got.Alerts, alert) {
			t.Errorf("alerts %v do not contain %q", got.Alerts, alert)
		}
	}
	if got.Selection["docs"].Selected != 5 {
		t.Fatalf("docs selected count = %d, want 5", got.Selection["docs"].Selected)
	}
}

func TestAnalyzeHealthHealthyReportHasNoAlerts(t *testing.T) {
	t.Parallel()

	jobs := Jobs{Docs: true}
	got := AnalyzeHealth([]HealthRun{{
		Event:           "pull_request",
		DurationSeconds: 120,
		QueueSeconds:    5,
		Conclusion:      "success",
		Plan:            Plan{Nominal: jobs, Effective: jobs},
		Results:         successfulResults(jobs),
	}})
	if len(got.Alerts) != 0 {
		t.Fatalf("alerts = %v, want none", got.Alerts)
	}
}
