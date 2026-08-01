package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flidai/leapview/rtest/internal/coordinator/httpapi"
	"github.com/flidai/leapview/rtest/internal/coordinator/sqlite"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestAuthenticatedJobLifecycleThroughAPI(t *testing.T) {
	server, store := newServer(t)
	source := []byte("snapshot bytes")
	digestBytes := sha256.Sum256(source)
	digest := "sha256:" + hex.EncodeToString(digestBytes[:])
	manifest := protocol.SubmitManifest{
		Repository: "example/repo", Runner: "standard", Command: []string{"go", "test", "./..."},
		SourceDigest: digest, SourceSize: int64(len(source)), TimeoutSeconds: 60,
	}

	request := multipartRequest(t, server.URL+"/v1/jobs", manifest, source)
	request.Header.Set("Authorization", "Bearer test-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("submit status = %d: %s", response.StatusCode, readBody(t, response))
	}
	var submitted protocol.Job
	decode(t, response, &submitted)
	if submitted.Status != protocol.StatusQueued || submitted.SourceDigest != digest {
		t.Fatalf("submitted = %#v", submitted)
	}

	claimBody := strings.NewReader(`{"worker_id":"worker-1"}`)
	response = do(t, http.MethodPost, server.URL+"/v1/worker/claim", claimBody, "test-token")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("claim status = %d: %s", response.StatusCode, readBody(t, response))
	}
	var claimed protocol.Job
	decode(t, response, &claimed)
	if claimed.ID != submitted.ID || claimed.WorkerID != "worker-1" {
		t.Fatalf("claimed = %#v", claimed)
	}

	response = do(t, http.MethodGet, server.URL+"/v1/worker/jobs/"+submitted.ID+"/source", nil, "test-token")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("source status = %d", response.StatusCode)
	}
	if got := readBody(t, response); got != string(source) {
		t.Fatalf("source = %q", got)
	}

	response = do(t, http.MethodPost, server.URL+"/v1/worker/jobs/"+submitted.ID+"/logs", strings.NewReader("remote output\n"), "test-token")
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("append log status = %d: %s", response.StatusCode, readBody(t, response))
	}
	log, next, err := store.ReadLog(context.Background(), submitted.ID, 0, 1024)
	if err != nil || string(log) != "remote output\n" || next != int64(len(log)) {
		t.Fatalf("log = %q next=%d err=%v", log, next, err)
	}

	finish := strings.NewReader(`{"status":"succeeded","exit_code":0}`)
	response = do(t, http.MethodPost, server.URL+"/v1/worker/jobs/"+submitted.ID+"/finish", finish, "test-token")
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("finish status = %d: %s", response.StatusCode, readBody(t, response))
	}
	response = do(t, http.MethodGet, server.URL+"/v1/jobs/"+submitted.ID, nil, "test-token")
	var finished protocol.Job
	decode(t, response, &finished)
	if finished.Status != protocol.StatusSucceeded || finished.ExitCode == nil || *finished.ExitCode != 0 {
		t.Fatalf("finished = %#v", finished)
	}
}

func TestAPIRejectsMissingAuthenticationAndBadSnapshot(t *testing.T) {
	server, _ := newServer(t)
	response := do(t, http.MethodGet, server.URL+"/v1/jobs/does-not-exist", nil, "")
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.StatusCode)
	}

	manifest := protocol.SubmitManifest{
		Repository: "example/repo", Runner: "standard", Command: []string{"true"},
		SourceDigest: "sha256:" + strings.Repeat("0", 64), SourceSize: 3, TimeoutSeconds: 60,
	}
	request := multipartRequest(t, server.URL+"/v1/jobs", manifest, []byte("bad"))
	request.Header.Set("Authorization", "Bearer test-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad snapshot status = %d: %s", response.StatusCode, readBody(t, response))
	}
}

func TestQueuedJobCanBeCancelled(t *testing.T) {
	server, _ := newServer(t)
	source := []byte("x")
	digestBytes := sha256.Sum256(source)
	manifest := protocol.SubmitManifest{
		Repository: "example/repo", Runner: "standard", Command: []string{"sleep", "10"},
		SourceDigest: "sha256:" + hex.EncodeToString(digestBytes[:]), SourceSize: 1, TimeoutSeconds: 60,
	}
	request := multipartRequest(t, server.URL+"/v1/jobs", manifest, source)
	request.Header.Set("Authorization", "Bearer test-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var job protocol.Job
	decode(t, response, &job)
	response = do(t, http.MethodDelete, server.URL+"/v1/jobs/"+job.ID, nil, "test-token")
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel status = %d", response.StatusCode)
	}
	response = do(t, http.MethodGet, server.URL+"/v1/jobs/"+job.ID, nil, "test-token")
	decode(t, response, &job)
	if !job.CancelRequested {
		t.Fatalf("job = %#v", job)
	}
}

func TestJobsCanBeListedByRepository(t *testing.T) {
	server, _ := newServer(t)
	for _, repository := range []string{"example/one", "example/two", "example/one"} {
		source := []byte(repository)
		digestBytes := sha256.Sum256(source)
		manifest := protocol.SubmitManifest{
			Repository: repository, Runner: "standard", Command: []string{"true"},
			SourceDigest: "sha256:" + hex.EncodeToString(digestBytes[:]), SourceSize: int64(len(source)), TimeoutSeconds: 60,
		}
		request := multipartRequest(t, server.URL+"/v1/jobs", manifest, source)
		request.Header.Set("Authorization", "Bearer test-token")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("submit status = %d: %s", response.StatusCode, readBody(t, response))
		}
		_ = response.Body.Close()
	}

	response := do(t, http.MethodGet, server.URL+"/v1/jobs?repository=example%2Fone&limit=1", nil, "test-token")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d: %s", response.StatusCode, readBody(t, response))
	}
	var jobs []protocol.Job
	decode(t, response, &jobs)
	if len(jobs) != 1 || jobs[0].Repository != "example/one" {
		t.Fatalf("jobs = %#v", jobs)
	}
}

func newServer(t *testing.T) (*httptest.Server, *sqlite.Store) {
	t.Helper()
	root := t.TempDir()
	store, err := sqlite.Open(filepath.Join(root, "state", "rtest.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	handler, err := httpapi.New(httpapi.Config{Token: "test-token", PayloadDir: filepath.Join(root, "payloads")}, store)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, store
}

func multipartRequest(t *testing.T, url string, manifest protocol.SubmitManifest, source []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormField("manifest")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(part).Encode(manifest); err != nil {
		t.Fatal(err)
	}
	part, err = writer.CreateFormFile("source", "source.tar.zst")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(source); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func do(t *testing.T, method, url string, body io.Reader, token string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decode(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
