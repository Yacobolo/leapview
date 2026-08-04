package ci

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"
)

type HealthRun struct {
	ID              int64             `json:"id"`
	Event           string            `json:"event"`
	Attempt         int               `json:"attempt"`
	Conclusion      string            `json:"conclusion"`
	DurationSeconds int64             `json:"duration_seconds"`
	QueueSeconds    int64             `json:"queue_seconds"`
	Deferred        bool              `json:"deferred,omitempty"`
	Plan            Plan              `json:"plan"`
	Results         map[string]string `json:"results"`
}

type DurationMetric struct {
	Count      int   `json:"count"`
	P50Seconds int64 `json:"p50_seconds"`
	P95Seconds int64 `json:"p95_seconds"`
}

type SelectionMetric struct {
	Selected int     `json:"selected"`
	Percent  float64 `json:"percent"`
}

type HealthReport struct {
	GeneratedAt   time.Time                  `json:"generated_at"`
	RunCount      int                        `json:"run_count"`
	Successes     int                        `json:"successes"`
	Failures      int                        `json:"failures"`
	Cancellations int                        `json:"cancellations"`
	Deferred      int                        `json:"deferred"`
	Full          DurationMetric             `json:"full"`
	Selective     DurationMetric             `json:"selective"`
	Queue         DurationMetric             `json:"queue"`
	RerunPercent  float64                    `json:"rerun_percent"`
	AuditMisses   int                        `json:"audit_misses"`
	Selection     map[string]SelectionMetric `json:"selection"`
	Alerts        []string                   `json:"alerts"`
}

func AnalyzeHealth(runs []HealthRun) HealthReport {
	report := HealthReport{
		GeneratedAt: time.Now().UTC(),
		RunCount:    len(runs),
		Selection:   map[string]SelectionMetric{},
	}
	var fullDurations, selectiveDurations, queues []int64
	var reruns int
	selectedCounts := map[string]int{}
	for _, run := range runs {
		switch run.Conclusion {
		case "success":
			report.Successes++
		case "failure":
			report.Failures++
		case "cancelled":
			report.Cancellations++
		}
		if run.Deferred {
			report.Deferred++
			continue
		}
		if run.Attempt > 1 {
			reruns++
		}
		if run.QueueSeconds >= 0 {
			queues = append(queues, run.QueueSeconds)
		}
		if reflect.DeepEqual(run.Plan.Effective, FullJobs()) {
			fullDurations = append(fullDurations, run.DurationSeconds)
		} else if run.Event == "pull_request" && !run.Plan.Audit {
			selectiveDurations = append(selectiveDurations, run.DurationSeconds)
		}
		for job, selected := range run.Plan.Effective.Selected() {
			if selected {
				selectedCounts[job]++
			}
		}
		report.AuditMisses += len(EvaluatePlanGate(run.Plan, run.Results).AuditMisses)
	}
	report.Full = durationMetric(fullDurations)
	report.Selective = durationMetric(selectiveDurations)
	report.Queue = durationMetric(queues)
	executedRuns := len(runs) - report.Deferred
	if executedRuns > 0 {
		report.RerunPercent = float64(reruns) * 100 / float64(executedRuns)
	}
	for job, count := range selectedCounts {
		percent := 0.0
		if executedRuns > 0 {
			percent = float64(count) * 100 / float64(executedRuns)
		}
		report.Selection[job] = SelectionMetric{Selected: count, Percent: percent}
	}

	if report.Full.Count > 0 && report.Full.P95Seconds > int64((12*time.Minute).Seconds()) {
		report.Alerts = append(report.Alerts, fmt.Sprintf(
			"full CI p95 is %s (limit %s)",
			duration(report.Full.P95Seconds),
			12*time.Minute,
		))
	}
	if report.Selective.Count > 0 && report.Selective.P95Seconds > int64((6*time.Minute).Seconds()) {
		report.Alerts = append(report.Alerts, fmt.Sprintf(
			"selective PR p95 is %s (limit %s)",
			duration(report.Selective.P95Seconds),
			6*time.Minute,
		))
	}
	if report.Queue.Count > 0 && report.Queue.P95Seconds > int64((2*time.Minute).Seconds()) {
		report.Alerts = append(report.Alerts, fmt.Sprintf(
			"queue p95 is %s (limit %s)",
			duration(report.Queue.P95Seconds),
			2*time.Minute,
		))
	}
	if report.RerunPercent > 3 {
		report.Alerts = append(report.Alerts, fmt.Sprintf(
			"rerun rate is %.1f%% (limit 3.0%%)",
			report.RerunPercent,
		))
	}
	if report.AuditMisses > 0 {
		report.Alerts = append(report.Alerts, fmt.Sprintf(
			"selection audit detected %d miss%s",
			report.AuditMisses,
			plural(report.AuditMisses),
		))
	}
	return report
}

func durationMetric(values []int64) DurationMetric {
	return DurationMetric{
		Count:      len(values),
		P50Seconds: percentile(values, 0.50),
		P95Seconds: percentile(values, 0.95),
	}
}

func percentile(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}

func duration(seconds int64) time.Duration {
	return time.Duration(seconds) * time.Second
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "es"
}
