package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
	"github.com/Yacobolo/libredash/internal/manageddata"
	"github.com/Yacobolo/libredash/internal/manageddata/control"
	managedhttp "github.com/Yacobolo/libredash/internal/manageddata/http"
)

const (
	digestA   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestC   = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	revisionA = "sha256:" + digestA
	revisionB = "sha256:" + digestB
	revisionC = "sha256:" + digestC
)

func TestRevisionOperationsAreScopedAndPaginated(t *testing.T) {
	repo := metadataFixture()
	handler := newHandler(repo, nil, nil, nil)

	recorder := call(t, ``, func(w http.ResponseWriter, r *http.Request) {
		handler.GetManagedDataEnvironmentRevision(w, r, "project-a", "orders", "prod")
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("environment status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var current apigenapi.ManagedDataEnvironmentRevisionResponse
	decodeResponse(t, recorder, &current)
	if current.Revision == nil || current.Revision.Id != revisionA || current.Revision.UploadSessionId != "upload-a" || current.RolloutId == nil || *current.RolloutId != "rollout-a" {
		t.Fatalf("environment response = %#v", current)
	}

	limit := int32(1)
	recorder = call(t, ``, func(w http.ResponseWriter, r *http.Request) {
		handler.ListManagedDataRevisions(w, r, "project-a", "orders", apigenapi.GenListManagedDataRevisionsParams{Limit: &limit})
	})
	var first apigenapi.ManagedDataRevisionListResponse
	decodeResponse(t, recorder, &first)
	if len(first.Items) != 1 || first.Items[0].Id != revisionB || first.Page.NextCursor == nil || *first.Page.NextCursor == "" {
		t.Fatalf("first revision page = %#v", first)
	}
	recorder = call(t, ``, func(w http.ResponseWriter, r *http.Request) {
		handler.ListManagedDataRevisions(w, r, "project-a", "orders", apigenapi.GenListManagedDataRevisionsParams{Limit: &limit, PageToken: first.Page.NextCursor})
	})
	var second apigenapi.ManagedDataRevisionListResponse
	decodeResponse(t, recorder, &second)
	if len(second.Items) != 1 || second.Items[0].Id != revisionA {
		t.Fatalf("second revision page = %#v", second)
	}

	recorder = call(t, ``, func(w http.ResponseWriter, r *http.Request) {
		handler.GetManagedDataRevision(w, r, "project-a", "orders", revisionA)
	})
	var revision apigenapi.ManagedDataRevisionResponse
	decodeResponse(t, recorder, &revision)
	if revision.Id != revisionA || len(revision.Manifest.Files) != 1 || revision.Manifest.Files[0].Path != "orders.csv" {
		t.Fatalf("revision response = %#v", revision)
	}

	recorder = call(t, ``, func(w http.ResponseWriter, r *http.Request) {
		handler.GetManagedDataRevision(w, r, "project-a", "orders", revisionC)
	})
	assertPublicError(t, recorder, http.StatusNotFound, "unrelated-secret")
}

func TestUploadSessionOperationsUseControlServiceAndPrincipal(t *testing.T) {
	uploads := &fakeUploads{result: uploadFixture()}
	handler := newHandler(metadataFixture(), uploads, nil, nil)

	created := call(t, `{"manifest":{"files":[{"path":"orders.csv","size":3,"sha256":"`+digestA+`"}]}}`, func(w http.ResponseWriter, r *http.Request) {
		handler.CreateManagedDataUploadSession(w, r, "project-a", "orders", apigenapi.GenCreateManagedDataUploadSessionHeaders{IdempotencyKey: "create-key"})
	})
	if created.Code != http.StatusCreated || uploads.begin.Actor != "principal-a" || uploads.begin.IdempotencyKey != "create-key" {
		t.Fatalf("create = %d %s, request = %#v", created.Code, created.Body.String(), uploads.begin)
	}
	var session apigenapi.ManagedDataUploadSessionResponse
	decodeResponse(t, created, &session)
	if session.RevisionId != uploads.result.Manifest.RevisionID() || session.Files[0].Negotiation.Tus == nil || session.Files[0].Negotiation.Tus.Endpoint != "/api/v1/managed-data/tus" {
		t.Fatalf("created session = %#v", session)
	}

	tests := []struct {
		name string
		want int
		call func(http.ResponseWriter, *http.Request)
	}{
		{"get", http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.GetManagedDataUploadSession(w, r, "project-a", "orders", "upload-a")
		}},
		{"abort", http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.AbortManagedDataUploadSession(w, r, "project-a", "orders", "upload-a", apigenapi.GenAbortManagedDataUploadSessionHeaders{IdempotencyKey: "abort-key"})
		}},
		{"finalize", http.StatusAccepted, func(w http.ResponseWriter, r *http.Request) {
			handler.FinalizeManagedDataUploadSession(w, r, "project-a", "orders", "upload-a", apigenapi.GenFinalizeManagedDataUploadSessionHeaders{IdempotencyKey: "finalize-key"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := call(t, ``, test.call)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if uploads.recoverCalls != 1 || uploads.abortCalls != 1 || uploads.finalizeCalls != 1 {
		t.Fatalf("upload calls = recover %d, abort %d, finalize %d", uploads.recoverCalls, uploads.abortCalls, uploads.finalizeCalls)
	}
}

func TestMultipartOperationsAreSDKFreeAndScopedToUpload(t *testing.T) {
	uploadResult := uploadFixture()
	uploadResult.Files[0].Transport = control.TransportDescription{Protocol: control.ProtocolS3Multipart, S3Multipart: &control.S3MultipartDescription{CreateEndpoint: "/multipart", MinimumPartSize: 1, MaximumPartSize: 1024, MaximumParts: 100}}
	uploads := &fakeUploads{result: uploadResult}
	multipart := &fakeMultipart{upload: managedhttp.MultipartUpload{
		ID: "multipart-a", UploadSessionID: "upload-a", File: manageddata.File{Path: "orders.csv", Size: 3, SHA256: digestA},
		Status: managedhttp.MultipartStatusOpen, CreatedAt: "2026-01-01T00:00:00Z", ExpiresAt: "2026-01-01T01:00:00Z",
	}}
	handler := newHandler(metadataFixture(), uploads, multipart, nil)

	tests := []struct {
		name string
		body string
		want int
		call func(http.ResponseWriter, *http.Request)
	}{
		{"create", `{"path":"orders.csv"}`, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
			handler.CreateManagedDataS3MultipartUpload(w, r, "project-a", "orders", "upload-a", apigenapi.GenCreateManagedDataS3MultipartUploadHeaders{IdempotencyKey: "create-key"})
		}},
		{"sign", `{"size":3,"sha256":"` + digestA + `"}`, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.SignManagedDataS3MultipartPart(w, r, "project-a", "orders", "upload-a", "multipart-a", 1)
		}},
		{"complete", `{"parts":[{"partNumber":1,"etag":"etag-a","sha256":"` + digestA + `"}]}`, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.CompleteManagedDataS3MultipartUpload(w, r, "project-a", "orders", "upload-a", "multipart-a", apigenapi.GenCompleteManagedDataS3MultipartUploadHeaders{IdempotencyKey: "complete-key"})
		}},
		{"abort", ``, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.AbortManagedDataS3MultipartUpload(w, r, "project-a", "orders", "upload-a", "multipart-a", apigenapi.GenAbortManagedDataS3MultipartUploadHeaders{IdempotencyKey: "abort-key"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := call(t, test.body, test.call)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if multipart.create.Actor != "principal-a" || multipart.create.File.Path != "orders.csv" || multipart.sign.PartNumber != 1 || multipart.complete.Parts[0].ETag != "etag-a" {
		t.Fatalf("multipart requests = create %#v sign %#v complete %#v", multipart.create, multipart.sign, multipart.complete)
	}

	var signed apigenapi.ManagedDataS3MultipartSignedPartResponse
	signedRecorder := call(t, `{"size":3}`, func(w http.ResponseWriter, r *http.Request) {
		handler.SignManagedDataS3MultipartPart(w, r, "project-a", "orders", "upload-a", "multipart-a", 2)
	})
	decodeResponse(t, signedRecorder, &signed)
	if signed.Url != "https://signed.example/part" || len(signed.Headers) != 1 || signed.Headers[0].Name != "x-checksum" {
		t.Fatalf("signed response = %#v", signed)
	}
}

func TestRolloutOperationsPropagateScopeActorAndFilters(t *testing.T) {
	rollouts := &fakeRollouts{result: rolloutFixture()}
	handler := newHandler(metadataFixture(), nil, nil, rollouts)
	status := apigenapi.ManagedDataRolloutStatusDraft
	environment := "prod"
	limit := int32(10)

	tests := []struct {
		name string
		body string
		want int
		call func(http.ResponseWriter, *http.Request)
	}{
		{"list", ``, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.ListManagedDataRollouts(w, r, "project-a", "orders", apigenapi.GenListManagedDataRolloutsParams{Environment: &environment, Status: &status, Limit: &limit})
		}},
		{"create", `{"revisionId":"` + revisionA + `","environment":"prod","targets":[{"workspace":"workspace-a","servingStateId":"state-a"}]}`, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
			handler.CreateManagedDataRollout(w, r, "project-a", "orders", apigenapi.GenCreateManagedDataRolloutHeaders{IdempotencyKey: "create-key"})
		}},
		{"get", ``, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
			handler.GetManagedDataRollout(w, r, "project-a", "orders", "rollout-a")
		}},
		{"activate", ``, http.StatusAccepted, func(w http.ResponseWriter, r *http.Request) {
			handler.ActivateManagedDataRollout(w, r, "project-a", "orders", "rollout-a", apigenapi.GenActivateManagedDataRolloutHeaders{IdempotencyKey: "activate-key"})
		}},
		{"rollback", `{"reason":"bad data"}`, http.StatusAccepted, func(w http.ResponseWriter, r *http.Request) {
			handler.RollbackManagedDataRollout(w, r, "project-a", "orders", "rollout-a", apigenapi.GenRollbackManagedDataRolloutHeaders{IdempotencyKey: "rollback-key"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := call(t, test.body, test.call)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if rollouts.list.Environment != "prod" || rollouts.list.Status != managedhttp.RolloutStatusDraft || rollouts.create.Actor != "principal-a" || rollouts.activate.IdempotencyKey != "activate-key" || rollouts.rollback.Reason != "bad data" {
		t.Fatalf("rollout requests = list %#v create %#v activate %#v rollback %#v", rollouts.list, rollouts.create, rollouts.activate, rollouts.rollback)
	}
}

func TestStrictDecodingErrorMappingAndSanitization(t *testing.T) {
	t.Run("unknown JSON field", func(t *testing.T) {
		handler := newHandler(metadataFixture(), &fakeUploads{result: uploadFixture()}, nil, nil)
		recorder := call(t, `{"manifest":{"files":[]},"secret":"credential"}`, func(w http.ResponseWriter, r *http.Request) {
			handler.CreateManagedDataUploadSession(w, r, "project-a", "orders", apigenapi.GenCreateManagedDataUploadSessionHeaders{IdempotencyKey: "key"})
		})
		assertPublicError(t, recorder, http.StatusBadRequest, "credential")
	})

	t.Run("oversized JSON", func(t *testing.T) {
		options := handlerOptions(metadataFixture(), &fakeUploads{result: uploadFixture()}, nil, nil)
		options.MaxJSONBodyBytes = 32
		handler := managedhttp.NewHandler(options)
		recorder := call(t, `{"manifest":{"files":[{"path":"orders.csv"}]}}`, func(w http.ResponseWriter, r *http.Request) {
			handler.CreateManagedDataUploadSession(w, r, "project-a", "orders", apigenapi.GenCreateManagedDataUploadSessionHeaders{IdempotencyKey: "key"})
		})
		assertPublicError(t, recorder, http.StatusRequestEntityTooLarge, "orders.csv")
	})

	tests := []struct {
		err  error
		want int
	}{
		{managedhttp.ErrInvalid, http.StatusBadRequest},
		{manageddata.ErrNotFound, http.StatusNotFound},
		{control.ErrConflict, http.StatusConflict},
		{managedhttp.ErrTooLarge, http.StatusRequestEntityTooLarge},
		{errors.New("storage key s3://private credentials=secret signed=https://signed.example"), http.StatusInternalServerError},
		{control.ErrBackend, http.StatusBadGateway},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.want), func(t *testing.T) {
			repo := metadataFixture()
			repo.revisionErr = test.err
			handler := newHandler(repo, nil, nil, nil)
			recorder := call(t, ``, func(w http.ResponseWriter, r *http.Request) {
				handler.GetManagedDataRevision(w, r, "project-a", "orders", revisionA)
			})
			assertPublicError(t, recorder, test.want, "secret")
		})
	}

	handler := newHandler(metadataFixture(), nil, nil, nil)
	recorder := call(t, ``, func(w http.ResponseWriter, r *http.Request) {
		handler.CreateManagedDataS3MultipartUpload(w, r, "project-a", "orders", "upload-a", apigenapi.GenCreateManagedDataS3MultipartUploadHeaders{IdempotencyKey: "key"})
	})
	assertPublicError(t, recorder, http.StatusServiceUnavailable, "secret")
}

func TestMutationResponsesCannotCrossRequestedIDs(t *testing.T) {
	t.Run("upload session", func(t *testing.T) {
		uploads := &fakeUploads{result: uploadFixture()}
		uploads.result.ID = "upload-other"
		handler := newHandler(metadataFixture(), uploads, nil, nil)
		recorder := call(t, ``, func(w http.ResponseWriter, r *http.Request) {
			handler.AbortManagedDataUploadSession(w, r, "project-a", "orders", "upload-a", apigenapi.GenAbortManagedDataUploadSessionHeaders{IdempotencyKey: "key"})
		})
		assertPublicError(t, recorder, http.StatusNotFound, "orders.csv")
	})

	t.Run("multipart upload", func(t *testing.T) {
		uploads := &fakeUploads{result: uploadFixture()}
		multipart := &fakeMultipart{upload: managedhttp.MultipartUpload{ID: "multipart-other", UploadSessionID: "upload-a", File: manageddata.File{Path: "orders.csv", Size: 3, SHA256: digestA}, Status: managedhttp.MultipartStatusCompleted, CreatedAt: "2026-01-01T00:00:00Z"}}
		handler := newHandler(metadataFixture(), uploads, multipart, nil)
		recorder := call(t, `{"parts":[{"partNumber":1,"etag":"etag-a"}]}`, func(w http.ResponseWriter, r *http.Request) {
			handler.CompleteManagedDataS3MultipartUpload(w, r, "project-a", "orders", "upload-a", "multipart-a", apigenapi.GenCompleteManagedDataS3MultipartUploadHeaders{IdempotencyKey: "key"})
		})
		assertPublicError(t, recorder, http.StatusNotFound, "orders.csv")
	})

	t.Run("rollout revision", func(t *testing.T) {
		rollouts := &fakeRollouts{result: rolloutFixture()}
		rollouts.result.RevisionID = revisionB
		handler := newHandler(metadataFixture(), nil, nil, rollouts)
		recorder := call(t, `{"revisionId":"`+revisionA+`","environment":"prod","targets":[{"workspace":"workspace-a","servingStateId":"state-a"}]}`, func(w http.ResponseWriter, r *http.Request) {
			handler.CreateManagedDataRollout(w, r, "project-a", "orders", apigenapi.GenCreateManagedDataRolloutHeaders{IdempotencyKey: "key"})
		})
		assertPublicError(t, recorder, http.StatusNotFound, "state-a")
	})
}

type fakeRepository struct {
	collection  manageddata.Collection
	revisions   map[string]managedhttp.RevisionMetadata
	pointer     manageddata.EnvironmentPointer
	revisionErr error
}

func metadataFixture() *fakeRepository {
	collection := manageddata.Collection{ID: "collection-a", ProjectID: "project-a", ConnectionName: "orders", Status: manageddata.CollectionStatusActive}
	return &fakeRepository{
		collection: collection,
		revisions: map[string]managedhttp.RevisionMetadata{
			revisionA: {Revision: manageddata.Revision{ID: revisionA, CollectionID: collection.ID, Status: manageddata.RevisionStatusReady, ManifestJSON: `{"files":[{"path":"orders.csv","size":3,"sha256":"` + digestA + `"}]}`, FileCount: 1, SizeBytes: 3, CreatedAt: "2026-01-01T00:00:00Z"}, UploadSessionID: "upload-a"},
			revisionB: {Revision: manageddata.Revision{ID: revisionB, CollectionID: collection.ID, Status: manageddata.RevisionStatusReady, ManifestJSON: `{"files":[{"path":"customers.csv","size":4,"sha256":"` + digestB + `"}]}`, FileCount: 1, SizeBytes: 4, CreatedAt: "2026-01-02T00:00:00Z"}, UploadSessionID: "upload-b"},
			revisionC: {Revision: manageddata.Revision{ID: revisionC, CollectionID: "collection-other", Status: manageddata.RevisionStatusReady, ManifestJSON: `{"files":[{"path":"unrelated-secret.csv","size":4,"sha256":"` + digestC + `"}]}`}, UploadSessionID: "upload-secret"},
		},
		pointer: manageddata.EnvironmentPointer{CollectionID: collection.ID, Environment: "prod", RevisionID: revisionA, RolloutID: "rollout-a", UpdatedAt: "2026-01-03T00:00:00Z"},
	}
}

func (r *fakeRepository) CollectionByProjectConnection(_ context.Context, project, connection string) (manageddata.Collection, error) {
	if project != r.collection.ProjectID || connection != r.collection.ConnectionName {
		return manageddata.Collection{}, manageddata.ErrNotFound
	}
	return r.collection, nil
}

func (r *fakeRepository) RevisionByID(_ context.Context, id string) (managedhttp.RevisionMetadata, error) {
	if r.revisionErr != nil {
		return managedhttp.RevisionMetadata{}, r.revisionErr
	}
	revision, ok := r.revisions[id]
	if !ok {
		return managedhttp.RevisionMetadata{}, manageddata.ErrNotFound
	}
	return revision, nil
}

func (r *fakeRepository) ListRevisions(context.Context, string) ([]managedhttp.RevisionMetadata, error) {
	return []managedhttp.RevisionMetadata{r.revisions[revisionA], r.revisions[revisionB]}, nil
}

func (r *fakeRepository) EnvironmentPointer(context.Context, string, manageddata.Environment) (manageddata.EnvironmentPointer, error) {
	return r.pointer, nil
}

type fakeUploads struct {
	result        control.UploadResult
	begin         control.BeginUploadRequest
	recoverCalls  int
	abortCalls    int
	finalizeCalls int
}

func (u *fakeUploads) BeginUpload(_ context.Context, request control.BeginUploadRequest) (control.UploadResult, error) {
	u.begin = request
	return u.result, nil
}

func (u *fakeUploads) RecoverUpload(context.Context, control.UploadRequest) (control.UploadResult, error) {
	u.recoverCalls++
	return u.result, nil
}

func (u *fakeUploads) AbortUpload(context.Context, control.UploadRequest) (control.UploadResult, error) {
	u.abortCalls++
	return u.result, nil
}

func (u *fakeUploads) FinalizeUpload(context.Context, control.UploadRequest) (control.FinalizeResult, error) {
	u.finalizeCalls++
	return control.FinalizeResult{Upload: u.result}, nil
}

func uploadFixture() control.UploadResult {
	file := manageddata.File{Path: "orders.csv", Size: 3, SHA256: digestA}
	return control.UploadResult{
		ID: "upload-a", RevisionID: "internal-revision-a", Collection: control.CollectionResult{ID: "collection-a", Project: "project-a", Connection: "orders"},
		Status: manageddata.UploadStatusOpen, Manifest: manageddata.Manifest{Files: []manageddata.File{file}}, CreatedAt: "2026-01-01T00:00:00Z", ExpiresAt: "2026-01-01T01:00:00Z",
		Files: []control.UploadFile{{File: file, Status: control.FileStatusPending, Transport: control.TransportDescription{Protocol: control.ProtocolTus, Tus: &control.TusDescription{Endpoint: "/api/v1/managed-data/tus", UploadID: "tus-a", Offset: 0, ExpiresAt: "2026-01-01T01:00:00Z", Metadata: map[string]string{"secret": "do-not-return"}}}}},
	}
}

type fakeMultipart struct {
	upload   managedhttp.MultipartUpload
	create   managedhttp.MultipartCreateRequest
	sign     managedhttp.MultipartSignPartRequest
	complete managedhttp.MultipartCompleteRequest
}

func (m *fakeMultipart) Create(_ context.Context, request managedhttp.MultipartCreateRequest) (managedhttp.MultipartUpload, error) {
	m.create = request
	return m.upload, nil
}

func (m *fakeMultipart) SignPart(_ context.Context, request managedhttp.MultipartSignPartRequest) (managedhttp.MultipartSignedPart, error) {
	m.sign = request
	return managedhttp.MultipartSignedPart{UploadSessionID: request.UploadSessionID, MultipartUploadID: request.MultipartUploadID, PartNumber: request.PartNumber, URL: "https://signed.example/part", Headers: []managedhttp.HTTPHeader{{Name: "x-checksum", Value: "secret-signature"}}, ExpiresAt: "2026-01-01T00:15:00Z"}, nil
}

func (m *fakeMultipart) Complete(_ context.Context, request managedhttp.MultipartCompleteRequest) (managedhttp.MultipartUpload, error) {
	m.complete = request
	result := m.upload
	result.Status = managedhttp.MultipartStatusCompleted
	return result, nil
}

func (m *fakeMultipart) Abort(context.Context, managedhttp.MultipartRequest) (managedhttp.MultipartUpload, error) {
	result := m.upload
	result.Status = managedhttp.MultipartStatusAborted
	return result, nil
}

type fakeRollouts struct {
	result   managedhttp.Rollout
	list     managedhttp.RolloutListRequest
	create   managedhttp.RolloutCreateRequest
	activate managedhttp.RolloutRequest
	rollback managedhttp.RolloutRollbackRequest
}

func (r *fakeRollouts) List(_ context.Context, request managedhttp.RolloutListRequest) ([]managedhttp.Rollout, error) {
	r.list = request
	return []managedhttp.Rollout{r.result}, nil
}

func (r *fakeRollouts) Get(context.Context, managedhttp.RolloutRequest) (managedhttp.Rollout, error) {
	return r.result, nil
}

func (r *fakeRollouts) Create(_ context.Context, request managedhttp.RolloutCreateRequest) (managedhttp.Rollout, error) {
	r.create = request
	return r.result, nil
}

func (r *fakeRollouts) Activate(_ context.Context, request managedhttp.RolloutRequest) (managedhttp.Rollout, error) {
	r.activate = request
	return r.result, nil
}

func (r *fakeRollouts) Rollback(_ context.Context, request managedhttp.RolloutRollbackRequest) (managedhttp.Rollout, error) {
	r.rollback = request
	return r.result, nil
}

func rolloutFixture() managedhttp.Rollout {
	return managedhttp.Rollout{
		ID: "rollout-a", CollectionID: "collection-a", RevisionID: revisionA, Environment: "prod", Status: managedhttp.RolloutStatusDraft,
		CreatedAt: "2026-01-01T00:00:00Z", Targets: []managedhttp.RolloutTarget{{Workspace: "workspace-a", ServingStateID: "state-a", Status: managedhttp.RolloutTargetStatusPending}},
	}
}

func newHandler(repo managedhttp.Repository, uploads managedhttp.UploadCoordinator, multipart managedhttp.MultipartCoordinator, rollouts managedhttp.RolloutCoordinator) *managedhttp.Handler {
	return managedhttp.NewHandler(handlerOptions(repo, uploads, multipart, rollouts))
}

func handlerOptions(repo managedhttp.Repository, uploads managedhttp.UploadCoordinator, multipart managedhttp.MultipartCoordinator, rollouts managedhttp.RolloutCoordinator) managedhttp.Options {
	return managedhttp.Options{
		Repository: repo, Uploads: uploads, Multipart: multipart, Rollouts: rollouts,
		CurrentPrincipal: func(*http.Request) (managedhttp.Principal, bool) {
			return managedhttp.Principal{ID: "principal-a"}, true
		},
	}
}

func call(t *testing.T, body string, invoke func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	invoke(recorder, request)
	return recorder
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if recorder.Code < 200 || recorder.Code >= 300 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, recorder.Body.String())
	}
}

func assertPublicError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, forbidden string) {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, wantStatus, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), forbidden) || strings.Contains(recorder.Body.String(), "signed.example") || strings.Contains(recorder.Body.String(), "s3://") {
		t.Fatalf("error response leaked sensitive value: %s", recorder.Body.String())
	}
	var response apigenapi.Error
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Code != int32(wantStatus) || response.Message == "" {
		t.Fatalf("error response = %#v, error = %v", response, err)
	}
}
