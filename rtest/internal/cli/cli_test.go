package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/cli"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestStatusPrintsHumanReadableJob(t *testing.T) {
	job := protocol.Job{
		ID: "job-1", Repository: "example/service", Suite: "integration", Runner: "standard",
		Command: []string{"go", "test", "./..."}, Status: protocol.StatusRunning,
		WorkerID: "worker-1", CreatedAt: time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC), TimeoutSeconds: 900,
	}
	server := authenticatedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/jobs/job-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(job)
	})
	configure(t, server.URL)
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"status", "job-1"}, cli.IO{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"job-1", "running", "example/service", "integration", "worker-1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output %q missing %q", stdout.String(), want)
		}
	}
}

func TestListAndCancelUseJobAPI(t *testing.T) {
	cancelled := false
	server := authenticatedServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/jobs":
			if r.URL.Query().Get("repository") != "example/service" || r.URL.Query().Get("limit") != "5" {
				t.Errorf("query = %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]protocol.Job{{ID: "job-2", Repository: "example/service", Suite: "test", Status: protocol.StatusSucceeded, CreatedAt: time.Now()}})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/jobs/job-2":
			cancelled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	configure(t, server.URL)
	var stdout, stderr bytes.Buffer
	if code := cli.Run(context.Background(), []string{"list", "--repository", "example/service", "--limit", "5"}, cli.IO{Stdout: &stdout, Stderr: &stderr}); code != 0 {
		t.Fatalf("list code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "job-2") || !strings.Contains(stdout.String(), "succeeded") {
		t.Fatalf("list output = %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := cli.Run(context.Background(), []string{"cancel", "job-2"}, cli.IO{Stdout: &stdout, Stderr: &stderr}); code != 0 {
		t.Fatalf("cancel code=%d stderr=%q", code, stderr.String())
	}
	if !cancelled || !strings.Contains(stdout.String(), "cancellation requested") {
		t.Fatalf("cancelled=%v output=%q", cancelled, stdout.String())
	}
}

func authenticatedServer(t *testing.T, next http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}))
}

func configure(t *testing.T, url string) {
	t.Helper()
	t.Setenv("RTEST_CONFIG", t.TempDir()+"/missing.json")
	t.Setenv("RTEST_URL", url)
	t.Setenv("RTEST_TOKEN", "test-token")
}
