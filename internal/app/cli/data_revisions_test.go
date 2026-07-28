package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	manageddataapi "github.com/Yacobolo/leapview/internal/manageddata/api"
)

func TestDataRevisionsListAndCurrent(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q", got)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/revisions"):
			writeJSONTest(t, w, http.StatusOK, manageddataapi.ManagedDataRevisionListResponse{Items: []manageddataapi.ManagedDataRevisionSummaryResponse{{Id: digest, Status: manageddataapi.ManagedDataRevisionStatusAvailable, FileCount: 2, Size: 12, CreatedAt: "2026-01-01T00:00:00Z", UploadSessionId: "upload-1"}}, Page: manageddataapi.PageInfo{}})
		case strings.HasSuffix(r.URL.Path, "/active-revision"):
			writeJSONTest(t, w, http.StatusOK, manageddataapi.ManagedDataActiveRevisionResponse{Revision: &manageddataapi.ManagedDataRevisionSummaryResponse{Id: digest, Status: manageddataapi.ManagedDataRevisionStatusAvailable, FileCount: 2, Size: 12, CreatedAt: "2026-01-01T00:00:00Z", UploadSessionId: "upload-1"}})
		default:
			t.Fatalf("request path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{target: server.URL, token: "token"}
	client := newManagedDataCLIClient(server.Client(), server.URL, "token")

	var listOut bytes.Buffer
	if err := runDataRevisionsList(context.Background(), opts, dataRevisionOptions{project: "demo", connection: "orders"}, client, &listOut); err != nil {
		t.Fatalf("runDataRevisionsList() error = %v", err)
	}
	if !strings.Contains(listOut.String(), digest) || !strings.Contains(listOut.String(), "AVAILABLE") {
		t.Fatalf("list output = %q", listOut.String())
	}

	var currentOut bytes.Buffer
	if err := runDataRevisionCurrent(context.Background(), opts, dataRevisionOptions{project: "demo", connection: "orders"}, client, &currentOut); err != nil {
		t.Fatalf("runDataRevisionCurrent() error = %v", err)
	}
	if got, want := strings.TrimSpace(currentOut.String()), digest; got != want {
		t.Fatalf("current output = %q, want %q", got, want)
	}
}

func TestDataRevisionCurrentPrintsNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONTest(t, w, http.StatusOK, manageddataapi.ManagedDataActiveRevisionResponse{})
	}))
	defer server.Close()
	var out bytes.Buffer
	client := newManagedDataCLIClient(server.Client(), server.URL, "token")
	if err := runDataRevisionCurrent(context.Background(), &rootOptions{}, dataRevisionOptions{project: "demo", connection: "orders"}, client, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "none\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestDataRevisionCurrentIncludesProblemDetailsInHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSONTest(t, w, http.StatusForbidden, map[string]string{
			"code":   "AUTHORIZATION_DENIED",
			"detail": "publisher token cannot inspect this revision",
		})
	}))
	defer server.Close()

	client := newManagedDataCLIClient(server.Client(), server.URL, "token")
	err := runDataRevisionCurrent(
		context.Background(),
		&rootOptions{},
		dataRevisionOptions{project: "demo", connection: "orders"},
		client,
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("runDataRevisionCurrent() error = nil")
	}
	for _, expected := range []string{"HTTP 403", "AUTHORIZATION_DENIED", "publisher token cannot inspect this revision"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("runDataRevisionCurrent() error = %q, want %q", err, expected)
		}
	}
}
