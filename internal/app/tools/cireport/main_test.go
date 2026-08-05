package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	platformci "github.com/flidai/leapview/internal/platform/ci"
)

func TestJobResultsAggregatesMatrixFailures(t *testing.T) {
	t.Parallel()

	got := jobResults([]githubJob{
		{Name: "Go tests (packages)", Conclusion: "success"},
		{Name: "Go tests (app 1/4)", Conclusion: "failure"},
		{Name: "Frontend tests (core)", Conclusion: "success"},
		{Name: "CI gate", Conclusion: "failure"},
	})
	want := map[string]string{
		"go-tests":       "failure",
		"frontend-tests": "success",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results = %#v, want %#v", got, want)
	}
}

func TestDeferredStackRunIsExcludedFromExecutionMetrics(t *testing.T) {
	t.Parallel()

	if !deferredStackRun([]githubJob{
		{Name: "Self-hosted PR validation", Conclusion: "skipped"},
		{Name: "GitHub CI (external pull request)", Conclusion: "skipped"},
		{Name: "CI gate", Conclusion: "success"},
	}) {
		t.Fatal("non-top stack gate was not recognized as deferred")
	}
	if deferredStackRun([]githubJob{
		{Name: "Self-hosted PR validation", Conclusion: "success"},
		{Name: "GitHub CI (external pull request)", Conclusion: "skipped"},
		{Name: "CI gate", Conclusion: "success"},
	}) {
		t.Fatal("executed top-stack preflight was classified as deferred")
	}
}

func TestDecodePlanArchive(t *testing.T) {
	t.Parallel()

	want := platformci.Plan{
		Version:   platformci.PlanVersion,
		Reason:    "test",
		Nominal:   platformci.Jobs{Docs: true},
		Effective: platformci.Jobs{Docs: true},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("ci-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := decodePlanArchive(archive.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %#v, want %#v", got, want)
	}
}
