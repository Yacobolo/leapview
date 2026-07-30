package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	deploymentgen "github.com/flidai/leapview/internal/deployment/api/gen"
	"github.com/flidai/leapview/internal/platform/cliapi"
	projectcli "github.com/flidai/leapview/internal/project/cli"
	"github.com/spf13/cobra"
)

type projectPublishOperations struct {
	client cliapi.Client
}

func publishCommand(ctx context.Context) *cobra.Command {
	client := capabilityAPIClient{
		httpClient: authoringRefreshingHTTPClient(http.DefaultClient),
	}
	return projectcli.PublishCommand(
		ctx,
		client,
		projectcli.NewCandidateCheckpointStore(candidateCheckpointPath()),
		projectPublishOperations{client: client},
	)
}

func (operations projectPublishOperations) Publish(
	ctx context.Context,
	options projectcli.PublishOptions,
	out io.Writer,
) error {
	if operations.client == nil {
		return fmt.Errorf("Project publish API client is required")
	}
	transport, err := operations.client.Transport(ctx, options.Credentials)
	if err != nil {
		return err
	}
	checkpoint := options.Checkpoint
	response, err := deploymentgen.NewGenClient(transport).PublishProjectCandidate(
		ctx,
		deploymentgen.GenPublishProjectCandidateClientRequest{
			Project:   checkpoint.ProjectID,
			Candidate: checkpoint.CandidateID,
			Headers: deploymentgen.GenPublishProjectCandidateClientHeaders{
				IdempotencyKey: deploymentIdempotencyKey(
					"candidate-publish-v2",
					checkpoint.ProjectID,
					checkpoint.CandidateID,
					fmt.Sprintf("%d", checkpoint.CandidateRevision),
					checkpoint.ProvenanceDigest,
					checkpoint.TargetID,
				),
			},
			Body: deploymentgen.CandidatePublishRequest{
				ExpectedRevision: checkpoint.CandidateRevision,
				ProvenanceDigest: checkpoint.ProvenanceDigest,
				TargetId:         checkpoint.TargetID,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("publish candidate: %w", err)
	}
	if response.Body.Approval != nil &&
		response.Body.Approval.Status == deploymentgen.DeploymentApprovalStatusPending {
		fmt.Fprintf(out, "publish request %s pending approval\n", response.Body.Id)
		return nil
	}
	deployment, err := waitForCandidatePublish(
		ctx,
		deploymentgen.NewGenClient(transport),
		response.Body,
	)
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"published %s deployment %s\n",
		deployment.ReleaseId,
		deployment.Id,
	)
	return nil
}

func waitForCandidatePublish(
	ctx context.Context,
	client *deploymentgen.GenClient,
	deployment deploymentgen.DeploymentResponse,
) (deploymentgen.DeploymentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	projectID := deployment.ProjectId
	releaseID := deployment.ReleaseId
	deploymentID := deployment.Id
	if client == nil || projectID == "" || releaseID == "" ||
		deploymentID == "" {
		return deploymentgen.DeploymentResponse{}, fmt.Errorf(
			"publish candidate returned inconsistent deployment identity",
		)
	}
	for {
		if deployment.ProjectId != projectID ||
			deployment.ReleaseId != releaseID ||
			deployment.Id != deploymentID {
			return deploymentgen.DeploymentResponse{}, fmt.Errorf(
				"publish candidate returned inconsistent deployment scope",
			)
		}
		switch deployment.Status {
		case deploymentgen.DeploymentStatusActive:
			return deployment, nil
		case deploymentgen.DeploymentStatusQueued,
			deploymentgen.DeploymentStatusRunning:
		case deploymentgen.DeploymentStatusFailed,
			deploymentgen.DeploymentStatusCancelled,
			deploymentgen.DeploymentStatusSuperseded:
			detail := ""
			if deployment.Error != nil {
				detail = ": " + *deployment.Error
			}
			return deploymentgen.DeploymentResponse{}, fmt.Errorf(
				"publish candidate deployment %s%s",
				deployment.Status,
				detail,
			)
		default:
			return deploymentgen.DeploymentResponse{}, fmt.Errorf(
				"publish candidate returned unexpected deployment status %q",
				deployment.Status,
			)
		}
		select {
		case <-ctx.Done():
			return deploymentgen.DeploymentResponse{}, fmt.Errorf(
				"wait for candidate publication: %w",
				ctx.Err(),
			)
		case <-time.After(100 * time.Millisecond):
		}
		response, err := client.GetDeployment(
			ctx,
			deploymentgen.GenGetDeploymentClientRequest{
				Project: projectID, Deployment: deploymentID,
			},
		)
		if err != nil {
			return deploymentgen.DeploymentResponse{}, fmt.Errorf(
				"get candidate publication: %w",
				err,
			)
		}
		deployment = response.Body
	}
}

func candidateCheckpointPath() string {
	return filepath.Join(filepath.Dir(clientConfigPath()), "authoring.json")
}
