package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	deploymentgen "github.com/flidai/leapview/internal/deployment/api/gen"
	"github.com/flidai/leapview/internal/platform/cliapi"
	projectcli "github.com/flidai/leapview/internal/project/cli"
)

func TestProjectPublishOperationsUseGeneratedExactCandidateProtocol(t *testing.T) {
	transport := &publishTransportStub{pendingApproval: true}
	var output strings.Builder
	operations := projectPublishOperations{
		client: fixedTransportClient{transport: transport},
	}
	checkpoint := projectcli.CandidateCheckpoint{
		ProjectPath: "/work/leapview.yaml", TargetOrigin: "https://target.example",
		TargetID: "target_1", Environment: "production", ProjectID: "finance",
		CandidateID: "cand_1", CandidateRevision: 7,
		ArtifactDigest:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProvenanceDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	if err := operations.Publish(t.Context(), projectcli.PublishOptions{
		ProjectPath: checkpoint.ProjectPath,
		Credentials: cliapi.Credentials{Target: checkpoint.TargetOrigin, Token: "token"},
		Checkpoint:  checkpoint,
	}, &output); err != nil {
		t.Fatal(err)
	}
	if transport.request.OperationID != deploymentgen.GenOperationPublishProjectCandidate ||
		transport.request.PathParams["project"] != checkpoint.ProjectID ||
		transport.request.PathParams["candidate"] != checkpoint.CandidateID ||
		transport.request.Headers.Get("Idempotency-Key") == "" {
		t.Fatalf("request = %#v", transport.request)
	}
	body := transport.request.Body.(deploymentgen.CandidatePublishRequest)
	if body.ExpectedRevision != checkpoint.CandidateRevision ||
		body.ProvenanceDigest != checkpoint.ProvenanceDigest ||
		body.TargetId != checkpoint.TargetID {
		t.Fatalf("body = %#v", body)
	}
	if !strings.Contains(output.String(), "pending approval") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestProjectPublishOperationsRequireActivationForBootstrap(t *testing.T) {
	transport := &publishTransportStub{pendingApproval: true}
	operations := projectPublishOperations{
		client:        fixedTransportClient{transport: transport},
		requireActive: true,
	}
	checkpoint := projectcli.CandidateCheckpoint{
		ProjectPath: "/work/leapview.yaml", TargetOrigin: "https://target.example",
		TargetID: "target_1", Environment: "evaluation", ProjectID: "finance",
		CandidateID: "cand_1", CandidateRevision: 7,
		ArtifactDigest:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProvenanceDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	err := operations.Publish(t.Context(), projectcli.PublishOptions{
		ProjectPath: checkpoint.ProjectPath,
		Credentials: cliapi.Credentials{
			Target: checkpoint.TargetOrigin,
			Token:  "token",
		},
		Checkpoint: checkpoint,
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "pending approval") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestProjectPublishOperationsWaitForImmediateActivation(t *testing.T) {
	transport := &publishTransportStub{
		getStatuses: []deploymentgen.DeploymentStatus{
			deploymentgen.DeploymentStatusRunning,
			deploymentgen.DeploymentStatusActive,
		},
	}
	var output strings.Builder
	operations := projectPublishOperations{
		client: fixedTransportClient{transport: transport},
	}
	checkpoint := projectcli.CandidateCheckpoint{
		ProjectPath: "/work/leapview.yaml", TargetOrigin: "https://target.example",
		TargetID: "target_1", Environment: "development", ProjectID: "finance",
		CandidateID: "cand_1", CandidateRevision: 7,
		ArtifactDigest:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProvenanceDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	if err := operations.Publish(t.Context(), projectcli.PublishOptions{
		ProjectPath: checkpoint.ProjectPath,
		Credentials: cliapi.Credentials{
			Target: checkpoint.TargetOrigin,
			Token:  "token",
		},
		Checkpoint: checkpoint,
	}, &output); err != nil {
		t.Fatal(err)
	}
	if len(transport.requests) != 3 ||
		transport.requests[0].OperationID !=
			deploymentgen.GenOperationPublishProjectCandidate ||
		transport.requests[1].OperationID !=
			deploymentgen.GenOperationGetDeployment ||
		transport.requests[2].OperationID !=
			deploymentgen.GenOperationGetDeployment {
		t.Fatalf("requests = %#v", transport.requests)
	}
	if !strings.Contains(output.String(), "published release_1 deployment request_1") {
		t.Fatalf("output = %q", output.String())
	}
}

type fixedTransportClient struct {
	transport apigenclient.Transport
}

func (client fixedTransportClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	return credentials, nil
}

func (client fixedTransportClient) Environment(context.Context, cliapi.Credentials, string) (string, error) {
	return "", nil
}

func (client fixedTransportClient) Transport(context.Context, cliapi.Credentials) (apigenclient.Transport, error) {
	return client.transport, nil
}

type publishTransportStub struct {
	request         apigenclient.Request
	requests        []apigenclient.Request
	pendingApproval bool
	getStatuses     []deploymentgen.DeploymentStatus
}

func (stub *publishTransportStub) DoAPIGen(
	_ context.Context,
	request apigenclient.Request,
	out any,
) (apigenclient.Response, error) {
	stub.request = request
	stub.requests = append(stub.requests, request)
	response := deploymentgen.DeploymentResponse{
		Id: "request_1", ProjectId: "finance", ReleaseId: "release_1",
		Status: deploymentgen.DeploymentStatusQueued,
	}
	if stub.pendingApproval {
		response.Approval = &deploymentgen.DeploymentApprovalResponse{
			Status: deploymentgen.DeploymentApprovalStatusPending,
		}
	}
	if request.OperationID == deploymentgen.GenOperationGetDeployment {
		if len(stub.getStatuses) == 0 {
			return apigenclient.Response{}, context.DeadlineExceeded
		}
		response.Status = stub.getStatuses[0]
		stub.getStatuses = stub.getStatuses[1:]
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return apigenclient.Response{}, err
	}
	if err := json.Unmarshal(encoded, out); err != nil {
		return apigenclient.Response{}, err
	}
	return apigenclient.Response{
		StatusCode: http.StatusAccepted, Headers: http.Header{},
		ContentType: "application/json",
	}, nil
}
