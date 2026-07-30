package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	deploymentgen "github.com/flidai/leapview/internal/deployment/api/gen"
	projectdevloop "github.com/flidai/leapview/internal/project/devloop"
)

func TestGeneratedCandidateSynchronizationTransportMapsTypedProtocol(t *testing.T) {
	generic := &candidateSyncTransportStub{}
	transport := newCandidateSynchronizationTransport(deploymentgen.NewGenClient(generic))
	request := projectdevloop.SynchronizationPlanRequest{
		ProjectID: "finance", ProjectFile: "leapview.yaml",
		ArtifactDigest:         "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedCandidateID:    "cand_1",
		ExpectedArtifactDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Artifacts: []projectdevloop.ArtifactReference{{
			Path: "leapview.yaml", Digest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		}},
	}

	plan, err := transport.Plan(t.Context(), request)
	if err != nil || len(plan.MissingDigests) != 1 ||
		plan.MissingDigests[0] != request.Artifacts[0].Digest {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	if err := transport.Upload(t.Context(), request, projectdevloop.Artifact{
		Path: request.Artifacts[0].Path, Digest: request.Artifacts[0].Digest, Content: []byte("source"),
	}); err != nil {
		t.Fatal(err)
	}
	candidate, err := transport.Commit(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.ID != "cand_1" || candidate.ProjectID != "finance" ||
		candidate.ArtifactDigest != request.ArtifactDigest ||
		candidate.TargetID != "target_1" ||
		candidate.Environment != "development" ||
		candidate.ProvenanceDigest != "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd" ||
		candidate.Revision != 7 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(generic.requests) != 3 ||
		generic.requests[0].Headers.Get("Idempotency-Key") == "" ||
		generic.requests[1].Headers.Get("Content-Digest") != standardCandidateContentDigest(request.Artifacts[0].Digest) ||
		generic.requests[2].Headers.Get("Idempotency-Key") == "" ||
		string(generic.requests[1].Body.([]byte)) != "source" {
		t.Fatalf("generated requests = %#v", generic.requests)
	}
}

func TestDevCommandExposesOneAuthenticatedRemoteWorkflow(t *testing.T) {
	command := devCommand(context.Background())
	if command.Name() != "dev" || !strings.Contains(strings.ToLower(command.Short), "private") {
		t.Fatalf("dev command = %q %q", command.Name(), command.Short)
	}
	for _, flag := range []string{"project", "target", "token", "upload-concurrency", "once"} {
		if command.Flags().Lookup(flag) == nil {
			t.Errorf("dev command is missing --%s", flag)
		}
	}
	for _, forbidden := range []string{"local-server", "production", "workspace"} {
		if command.Flags().Lookup(forbidden) != nil {
			t.Errorf("dev command exposes alternate workflow flag --%s", forbidden)
		}
	}
}

type candidateSyncTransportStub struct {
	requests []apigenclient.Request
}

func (stub *candidateSyncTransportStub) DoAPIGen(
	_ context.Context,
	request apigenclient.Request,
	out any,
) (apigenclient.Response, error) {
	stub.requests = append(stub.requests, request)
	var response any
	status := http.StatusOK
	switch request.OperationID {
	case deploymentgen.GenOperationPlanProjectCandidateSynchronization:
		body := request.Body.(deploymentgen.CandidateSynchronizationRequest)
		response = deploymentgen.CandidateSynchronizationPlanResponse{
			ArtifactDigest: body.ArtifactDigest,
			MissingDigests: []string{body.Artifacts[0].Digest},
		}
	case deploymentgen.GenOperationUploadProjectCandidateSourceBlob:
		status = http.StatusCreated
		response = deploymentgen.CandidateSourceBlobResponse{
			Digest: request.PathParams["digest"], SizeBytes: int64(len(request.Body.([]byte))),
		}
	case deploymentgen.GenOperationCommitProjectCandidateSynchronization:
		body := request.Body.(deploymentgen.CandidateSynchronizationRequest)
		response = deploymentgen.CandidateResponse{
			Id: "cand_1", ProjectId: "finance", ArtifactDigest: body.ArtifactDigest,
			PreviewUrl: "https://target.example/candidates/cand_1",
			TargetId:   "target_1", Environment: "development", Revision: 7,
			ProvenanceDigest: testPointer("sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"),
		}
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return apigenclient.Response{}, err
	}
	if err := json.Unmarshal(encoded, out); err != nil {
		return apigenclient.Response{}, err
	}
	return apigenclient.Response{
		StatusCode: status, Headers: make(http.Header), ContentType: "application/json",
	}, nil
}

func testPointer[T any](value T) *T {
	return &value
}
