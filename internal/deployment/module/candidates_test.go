package module

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/deployment"
	deploymenthttp "github.com/flidai/leapview/internal/deployment/http"
	deploymentsqlite "github.com/flidai/leapview/internal/deployment/sqlite"
	"github.com/flidai/leapview/internal/platform"
	"github.com/flidai/leapview/internal/project"
)

func TestCandidateSynchronizationPlansUploadsAndCommitsOwnedCandidate(t *testing.T) {
	module := testCandidateModule(t, "principal_1")
	digest := "sha256:" + strings.Repeat("a", 64)
	blobDigest := "sha256:" + strings.Repeat("b", 64)
	sources := &candidateSourceSynchronizerStub{missing: []string{blobDigest}}
	module.candidateSources = sources
	body := `{"projectFile":"leapview.yaml","artifactDigest":"` + digest +
		`","artifacts":[{"path":"leapview.yaml","digest":"` + blobDigest + `"}]}`

	planned := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidate-sync/plan", body, func(w http.ResponseWriter, r *http.Request) {
		module.PlanProjectCandidateSynchronization(w, r, "finance")
	})
	if planned.Code != http.StatusOK || !strings.Contains(planned.Body.String(), blobDigest) {
		t.Fatalf("plan response = %d %s", planned.Code, planned.Body.String())
	}
	contentDigest := standardContentDigest(t, blobDigest)
	uploaded := callCandidateAPI(t, http.MethodPut, "/api/v1/projects/finance/candidate-sync/blobs/"+blobDigest, "blob", func(w http.ResponseWriter, r *http.Request) {
		module.UploadProjectCandidateSourceBlob(w, r, "finance", blobDigest, "application/octet-stream", contentDigest)
	})
	if uploaded.Code != http.StatusCreated || string(sources.uploaded) != "blob" {
		t.Fatalf("upload response = %d %s bytes=%q", uploaded.Code, uploaded.Body.String(), sources.uploaded)
	}
	committed := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidate-sync/commit", body, func(w http.ResponseWriter, r *http.Request) {
		module.CommitProjectCandidateSynchronization(w, r, "finance", "commit-1")
	})
	var candidate candidateAPIResponse
	decodeCandidateResponse(t, committed, &candidate)
	if committed.Code != http.StatusOK || candidate.ID == "" ||
		candidate.ArtifactDigest != digest || sources.commits != 1 {
		t.Fatalf("commit response = %d candidate=%#v commits=%d", committed.Code, candidate, sources.commits)
	}

	nextDigest := "sha256:" + strings.Repeat("c", 64)
	replacementBody := `{"projectFile":"leapview.yaml","artifactDigest":"` + nextDigest +
		`","expectedCandidateId":"` + candidate.ID + `","expectedArtifactDigest":"` + digest +
		`","artifacts":[{"path":"leapview.yaml","digest":"` + blobDigest + `"}]}`
	replaced := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidate-sync/commit", replacementBody, func(w http.ResponseWriter, r *http.Request) {
		module.CommitProjectCandidateSynchronization(w, r, "finance", "commit-2")
	})
	var replacement candidateAPIResponse
	decodeCandidateResponse(t, replaced, &replacement)
	if replaced.Code != http.StatusOK || replacement.ID != candidate.ID ||
		replacement.ArtifactDigest != nextDigest || replacement.Revision != candidate.Revision+1 {
		t.Fatalf("replacement response = %d candidate=%#v", replaced.Code, replacement)
	}
}

func TestCandidateSynchronizationRejectsBlobHeaderMismatchBeforeStorage(t *testing.T) {
	module := testCandidateModule(t, "principal_1")
	sources := &candidateSourceSynchronizerStub{}
	module.candidateSources = sources
	digest := "sha256:" + strings.Repeat("a", 64)
	response := callCandidateAPI(t, http.MethodPut, "/api/v1/projects/finance/candidate-sync/blobs/"+digest, "blob", func(w http.ResponseWriter, r *http.Request) {
		module.UploadProjectCandidateSourceBlob(w, r, "finance", digest, "application/octet-stream", "sha-256=:wrong:")
	})
	if response.Code != http.StatusUnprocessableEntity || len(sources.uploaded) != 0 {
		t.Fatalf("mismatched upload response = %d %s", response.Code, response.Body.String())
	}
}

func TestCandidateSynchronizationMapsProjectSourceErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{project.ErrCandidateSourceUnavailable, http.StatusServiceUnavailable, "CANDIDATE_SERVICE_UNAVAILABLE"},
		{project.ErrCandidateSourceConflict, http.StatusConflict, "CANDIDATE_CONFLICT"},
		{project.ErrCandidateSourceInvalid, http.StatusUnprocessableEntity, "INVALID_CANDIDATE"},
	}
	for _, test := range tests {
		module := testCandidateModule(t, "principal_1")
		module.candidateSources = &candidateSourceSynchronizerStub{planErr: test.err}
		response := callCandidateAPI(
			t,
			http.MethodPost,
			"/api/v1/projects/finance/candidate-sync/plan",
			`{"projectFile":"leapview.yaml","artifactDigest":"sha256:`+strings.Repeat("a", 64)+`","artifacts":[]}`,
			func(w http.ResponseWriter, r *http.Request) {
				module.PlanProjectCandidateSynchronization(w, r, "finance")
			},
		)
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
			t.Fatalf("error %v response = %d %s", test.err, response.Code, response.Body.String())
		}
	}
}

func TestCandidateAPIStartsResumesUpdatesAndCancelsOwnedSession(t *testing.T) {
	module := testCandidateModule(t, "principal_1")
	digest := "sha256:" + strings.Repeat("a", 64)
	started := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidates", `{"artifactDigest":"`+digest+`"}`, func(w http.ResponseWriter, r *http.Request) {
		module.StartProjectCandidate(w, r, "finance", "start-1")
	})
	if started.Code != http.StatusCreated {
		t.Fatalf("start status = %d body=%s", started.Code, started.Body.String())
	}
	var created candidateAPIResponse
	decodeCandidateResponse(t, started, &created)
	if created.ID == "" || created.BaseGeneration != deployment.CandidateBaseGenerationEmpty || created.Status != string(deployment.CandidatePreparing) {
		t.Fatalf("created candidate = %#v", created)
	}
	if want := "https://prod.leapview.example/candidates/" + created.ID; created.PreviewURL != want {
		t.Fatalf("preview URL = %q, want %q", created.PreviewURL, want)
	}
	if got := started.Header().Get("Location"); got != "/api/v1/projects/finance/candidates/"+created.ID {
		t.Fatalf("Location = %q", got)
	}

	resumed := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidates", `{"artifactDigest":"`+digest+`"}`, func(w http.ResponseWriter, r *http.Request) {
		module.StartProjectCandidate(w, r, "finance", "start-retry")
	})
	var replay candidateAPIResponse
	decodeCandidateResponse(t, resumed, &replay)
	if resumed.Code != http.StatusCreated || replay.ID != created.ID || !replay.Resumed {
		t.Fatalf("resumed status=%d candidate=%#v", resumed.Code, replay)
	}

	nextDigest := "sha256:" + strings.Repeat("b", 64)
	replaced := callCandidateAPI(t, http.MethodPut, "/api/v1/projects/finance/candidates/"+created.ID+"/artifact",
		`{"expectedArtifactDigest":"`+digest+`","artifactDigest":"`+nextDigest+`"}`,
		func(w http.ResponseWriter, r *http.Request) {
			module.ReplaceProjectCandidateArtifact(w, r, "finance", created.ID, "replace-1")
		})
	var updated candidateAPIResponse
	decodeCandidateResponse(t, replaced, &updated)
	if replaced.Code != http.StatusOK || updated.ArtifactDigest != nextDigest || updated.Revision != created.Revision+1 {
		t.Fatalf("replacement status=%d candidate=%#v", replaced.Code, updated)
	}

	cancelled := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidates/"+created.ID+"/cancel", "", func(w http.ResponseWriter, r *http.Request) {
		module.CancelProjectCandidate(w, r, "finance", created.ID, "cancel-1")
	})
	var stopped candidateAPIResponse
	decodeCandidateResponse(t, cancelled, &stopped)
	if cancelled.Code != http.StatusOK || stopped.Status != string(deployment.CandidateCancelled) {
		t.Fatalf("cancel status=%d candidate=%#v", cancelled.Code, stopped)
	}
}

func TestCandidateAPIConcealsForeignOwnershipAndMapsValidation(t *testing.T) {
	owner := testCandidateModule(t, "principal_1")
	digest := "sha256:" + strings.Repeat("a", 64)
	started := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidates", `{"artifactDigest":"`+digest+`"}`, func(w http.ResponseWriter, r *http.Request) {
		owner.StartProjectCandidate(w, r, "finance", "start-1")
	})
	var created candidateAPIResponse
	decodeCandidateResponse(t, started, &created)

	foreign := *owner
	foreign.handler = deploymenthttp.NewHandler(deploymenthttp.Options{
		CurrentPrincipal: func(*http.Request) (deploymenthttp.Principal, bool) {
			return deploymenthttp.Principal{ID: "principal_2"}, true
		},
		InstanceEnvironment: "prod",
	})
	hidden := callCandidateAPI(t, http.MethodGet, "/api/v1/projects/finance/candidates/"+created.ID, "", func(w http.ResponseWriter, r *http.Request) {
		foreign.GetProjectCandidate(w, r, "finance", created.ID)
	})
	if hidden.Code != http.StatusNotFound || !strings.Contains(hidden.Body.String(), "CANDIDATE_NOT_FOUND") {
		t.Fatalf("foreign response = %d %s", hidden.Code, hidden.Body.String())
	}

	invalid := callCandidateAPI(t, http.MethodPost, "/api/v1/projects/finance/candidates", `{"artifactDigest":"not-a-digest"}`, func(w http.ResponseWriter, r *http.Request) {
		owner.StartProjectCandidate(w, r, "finance", "invalid-1")
	})
	if invalid.Code != http.StatusUnprocessableEntity || !strings.Contains(invalid.Body.String(), "INVALID_CANDIDATE") {
		t.Fatalf("invalid response = %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestCandidatePreviewMapsLifecycleAndConcealsRuntimeDetails(t *testing.T) {
	start := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		status deployment.CandidateStatus
		code   int
	}{
		{status: deployment.CandidatePreparing, code: http.StatusAccepted},
		{status: deployment.CandidateReady, code: http.StatusOK},
		{status: deployment.CandidateFailed, code: http.StatusConflict},
		{status: deployment.CandidateCancelled, code: http.StatusGone},
		{status: deployment.CandidateExpired, code: http.StatusGone},
	} {
		t.Run(string(test.status), func(t *testing.T) {
			now := start
			module := testCandidateModuleWithClock(t, "principal_1", func() time.Time { return now }, time.Minute)
			digest := "sha256:" + strings.Repeat("a", 64)
			started, err := module.candidates.Start(context.Background(), deployment.StartCandidateRequest{
				ProjectID: "finance", OwnerID: "principal_1", ArtifactDigest: digest,
			})
			if err != nil {
				t.Fatal(err)
			}
			scope := deployment.CandidateScope{
				ProjectID: "finance", CandidateID: started.Candidate.ID, OwnerID: "principal_1",
			}
			now = now.Add(30 * time.Second)
			switch test.status {
			case deployment.CandidateReady:
				_, err = module.candidates.MarkReady(context.Background(), scope, digest)
			case deployment.CandidateFailed:
				_, err = module.candidates.MarkFailed(context.Background(), scope, digest, "RUNTIME_PREPARATION_FAILED")
			case deployment.CandidateCancelled:
				_, err = module.candidates.Cancel(context.Background(), scope)
			case deployment.CandidateExpired:
				now = start.Add(2 * time.Minute)
			}
			if err != nil {
				t.Fatal(err)
			}

			request := httptest.NewRequest(http.MethodGet, "/candidates/"+started.Candidate.ID, nil)
			response := httptest.NewRecorder()
			module.ServeCandidatePreview(response, request, started.Candidate.ID, "principal_1", nil)
			if response.Code != test.code {
				t.Fatalf("status = %d, want %d body=%s", response.Code, test.code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
			for _, forbidden := range []string{
				started.Candidate.ArtifactDigest, started.Candidate.OwnerID,
				started.Candidate.ProjectID, started.Candidate.TargetID,
			} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatalf("preview leaked %q: %s", forbidden, response.Body.String())
				}
			}
		})
	}
}

func TestCandidatePreviewConcealsForeignOwnership(t *testing.T) {
	module := testCandidateModule(t, "principal_1")
	digest := "sha256:" + strings.Repeat("a", 64)
	started, err := module.candidates.Start(context.Background(), deployment.StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1", ArtifactDigest: digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/candidates/"+started.Candidate.ID, nil)
	response := httptest.NewRecorder()
	module.ServeCandidatePreview(response, request, started.Candidate.ID, "principal_2", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("foreign preview status = %d body=%s", response.Code, response.Body.String())
	}
}

func testCandidateModule(t *testing.T, principalID string) *Module {
	t.Helper()
	return testCandidateModuleWithClock(
		t,
		principalID,
		func() time.Time { return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC) },
		0,
	)
}

func testCandidateModuleWithClock(t *testing.T, principalID string, now func() time.Time, lifetime time.Duration) *Module {
	t.Helper()
	store, err := platform.Open(context.Background(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, id := range []string{"principal_1", "principal_2"} {
		if _, err := store.SQLDB().ExecContext(context.Background(),
			`INSERT INTO principals (id, email, display_name) VALUES (?, ?, ?)`,
			id, id+"@example.test", id,
		); err != nil {
			t.Fatal(err)
		}
	}
	repository := deploymentsqlite.NewRepositoryWithHooks(store.SQLDB(), deploymentsqlite.ActivationHooks{})
	service, err := deployment.NewCandidateService(repository, deployment.CandidateServiceConfig{
		TargetID: "lvinst_prod", CanonicalOrigin: "https://prod.leapview.example", Environment: "prod",
		Lifetime: lifetime, Now: now, NewID: func() (string, error) { return "cand_opaque_1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Module{
		candidates: service,
		handler: deploymenthttp.NewHandler(deploymenthttp.Options{
			CurrentPrincipal: func(*http.Request) (deploymenthttp.Principal, bool) {
				return deploymenthttp.Principal{ID: principalID}, true
			},
			InstanceEnvironment: "prod",
		}),
	}
}

func callCandidateAPI(t *testing.T, method, target, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}

type candidateAPIResponse struct {
	ID             string `json:"id"`
	BaseGeneration string `json:"baseGeneration"`
	ArtifactDigest string `json:"artifactDigest"`
	Status         string `json:"status"`
	PreviewURL     string `json:"previewUrl"`
	Revision       int64  `json:"revision"`
	Resumed        bool   `json:"resumed"`
}

func decodeCandidateResponse(t *testing.T, response *httptest.ResponseRecorder, target *candidateAPIResponse) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode candidate response: %v body=%s", err, response.Body.String())
	}
}

type candidateSourceSynchronizerStub struct {
	missing  []string
	uploaded []byte
	commits  int
	planErr  error
}

func (stub *candidateSourceSynchronizerStub) Plan(
	context.Context,
	deployment.CandidateSourceScope,
	deployment.CandidateSynchronizationRequest,
) ([]string, error) {
	return append([]string(nil), stub.missing...), stub.planErr
}

func (stub *candidateSourceSynchronizerStub) Upload(
	_ context.Context,
	_ deployment.CandidateSourceScope,
	_ string,
	source io.Reader,
) error {
	bytes, err := io.ReadAll(source)
	stub.uploaded = bytes
	return err
}

func (stub *candidateSourceSynchronizerStub) Commit(
	context.Context,
	deployment.CandidateSourceScope,
	deployment.CandidateSynchronizationRequest,
) error {
	stub.commits++
	return nil
}

func standardContentDigest(t *testing.T, identity string) string {
	t.Helper()
	decoded, err := hex.DecodeString(strings.TrimPrefix(identity, "sha256:"))
	if err != nil {
		t.Fatal(err)
	}
	return "sha-256=:" + base64.StdEncoding.EncodeToString(decoded) + ":"
}
