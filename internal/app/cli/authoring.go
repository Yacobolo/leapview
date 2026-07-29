package cli

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	deploymentgen "github.com/flidai/leapview/internal/deployment/api/gen"
	"github.com/flidai/leapview/internal/platform/cliapi"
	projectdevloop "github.com/flidai/leapview/internal/project/devloop"
	configschema "github.com/flidai/leapview/internal/project/schema"
	"github.com/spf13/cobra"
)

type candidateSynchronizationTransport struct {
	client *deploymentgen.GenClient
}

type devOptions struct {
	project           string
	target            string
	token             string
	uploadConcurrency int
}

func devCommand(ctx context.Context) *cobra.Command {
	options := &devOptions{uploadConcurrency: 4}
	command := &cobra.Command{
		Use:   "dev [project]",
		Short: "Synchronize a project into your private target candidate",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 1 {
				if command.Flags().Changed("project") {
					return fmt.Errorf("choose either --project or positional project, not both")
				}
				options.project = args[0]
			}
			return runDev(ctx, options, command.OutOrStdout(), command.ErrOrStderr())
		},
	}
	command.Flags().StringVar(
		&options.project, "project", filepath.Join("dashboards", "leapview.yaml"),
		"project manifest path",
	)
	command.Flags().StringVar(
		&options.target, "target", "",
		"authenticated target profile or LeapView target URL",
	)
	command.Flags().StringVar(
		&options.token, "token", "",
		"ephemeral API token compatibility path",
	)
	command.Flags().IntVar(
		&options.uploadConcurrency, "upload-concurrency", options.uploadConcurrency,
		"maximum parallel content-addressed source uploads (1-16)",
	)
	return command
}

func runDev(ctx context.Context, options *devOptions, out, errOut io.Writer) error {
	if options == nil {
		return fmt.Errorf("dev options are required")
	}
	client := capabilityAPIClient{
		httpClient: authoringRefreshingHTTPClient(http.DefaultClient),
	}
	generic, err := client.Transport(ctx, cliapi.Credentials{
		Target: options.target, Token: options.token,
	})
	if err != nil {
		return err
	}
	remote, err := projectdevloop.NewTransportRemote(
		newCandidateSynchronizationTransport(deploymentgen.NewGenClient(generic)),
		options.uploadConcurrency,
	)
	if err != nil {
		return err
	}
	service, err := projectdevloop.New(
		projectdevloop.FilesystemBuilder{ProjectPath: options.project},
		remote,
	)
	if err != nil {
		return err
	}
	watcher, err := projectdevloop.NewWatcher(options.project, service)
	if err != nil {
		return err
	}
	signalContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	lastPreviewURL := ""
	return watcher.Run(signalContext, func(update projectdevloop.Update) {
		if update.Err != nil {
			for _, diagnostic := range configschema.Diagnostics(update.Err) {
				fmt.Fprintln(errOut, diagnostic.String())
			}
			return
		}
		if update.Result.Status != projectdevloop.StatusSynchronized {
			return
		}
		candidate := update.Result.Candidate
		fmt.Fprintf(out, "synchronized %s\n", candidate.ArtifactDigest)
		if candidate.PreviewURL != "" && candidate.PreviewURL != lastPreviewURL {
			fmt.Fprintf(out, "preview %s\n", candidate.PreviewURL)
			lastPreviewURL = candidate.PreviewURL
		}
	})
}

func newCandidateSynchronizationTransport(
	client *deploymentgen.GenClient,
) *candidateSynchronizationTransport {
	return &candidateSynchronizationTransport{client: client}
}

func (transport *candidateSynchronizationTransport) Plan(
	ctx context.Context,
	request projectdevloop.SynchronizationPlanRequest,
) (projectdevloop.SynchronizationPlan, error) {
	if transport == nil || transport.client == nil {
		return projectdevloop.SynchronizationPlan{}, fmt.Errorf("candidate synchronization client is not configured")
	}
	response, err := transport.client.PlanProjectCandidateSynchronization(
		ctx,
		deploymentgen.GenPlanProjectCandidateSynchronizationClientRequest{
			Project: request.ProjectID,
			Headers: deploymentgen.GenPlanProjectCandidateSynchronizationClientHeaders{
				IdempotencyKey: deploymentIdempotencyKey(
					"candidate-plan", request.ProjectID,
					request.ExpectedCandidateID, request.ArtifactDigest,
				),
			},
			Body: candidateSynchronizationBody(request),
		},
	)
	if err != nil {
		return projectdevloop.SynchronizationPlan{}, err
	}
	if response.Body.ArtifactDigest != request.ArtifactDigest {
		return projectdevloop.SynchronizationPlan{}, fmt.Errorf("target synchronization plan does not match requested artifact")
	}
	return projectdevloop.SynchronizationPlan{
		MissingDigests: append([]string(nil), response.Body.MissingDigests...),
	}, nil
}

func (transport *candidateSynchronizationTransport) Upload(
	ctx context.Context,
	request projectdevloop.SynchronizationPlanRequest,
	artifact projectdevloop.Artifact,
) error {
	if transport == nil || transport.client == nil {
		return fmt.Errorf("candidate synchronization client is not configured")
	}
	response, err := transport.client.UploadProjectCandidateSourceBlob(
		ctx,
		deploymentgen.GenUploadProjectCandidateSourceBlobClientRequest{
			Project: request.ProjectID, Digest: artifact.Digest,
			Headers: deploymentgen.GenUploadProjectCandidateSourceBlobClientHeaders{
				ContentType:   "application/octet-stream",
				ContentDigest: standardCandidateContentDigest(artifact.Digest),
			},
			Body: append([]byte(nil), artifact.Content...),
		},
	)
	if err != nil {
		return err
	}
	if response.Body.Digest != artifact.Digest ||
		response.Body.SizeBytes != int64(len(artifact.Content)) {
		return fmt.Errorf("target source upload acknowledgement does not match artifact")
	}
	return nil
}

func (transport *candidateSynchronizationTransport) Commit(
	ctx context.Context,
	request projectdevloop.SynchronizationPlanRequest,
) (projectdevloop.Candidate, error) {
	if transport == nil || transport.client == nil {
		return projectdevloop.Candidate{}, fmt.Errorf("candidate synchronization client is not configured")
	}
	response, err := transport.client.CommitProjectCandidateSynchronization(
		ctx,
		deploymentgen.GenCommitProjectCandidateSynchronizationClientRequest{
			Project: request.ProjectID,
			Headers: deploymentgen.GenCommitProjectCandidateSynchronizationClientHeaders{
				IdempotencyKey: deploymentIdempotencyKey(
					"candidate-sync", request.ProjectID,
					request.ExpectedCandidateID, request.ArtifactDigest,
				),
			},
			Body: candidateSynchronizationBody(request),
		},
	)
	if err != nil {
		return projectdevloop.Candidate{}, err
	}
	return projectdevloop.Candidate{
		ID: response.Body.Id, ProjectID: response.Body.ProjectId,
		ArtifactDigest: response.Body.ArtifactDigest,
		PreviewURL:     response.Body.PreviewUrl,
	}, nil
}

func candidateSynchronizationBody(
	request projectdevloop.SynchronizationPlanRequest,
) deploymentgen.CandidateSynchronizationRequest {
	body := deploymentgen.CandidateSynchronizationRequest{
		ProjectFile: request.ProjectFile, ArtifactDigest: request.ArtifactDigest,
		Artifacts: make([]deploymentgen.CandidateSourceArtifact, len(request.Artifacts)),
	}
	if request.ExpectedCandidateID != "" {
		value := request.ExpectedCandidateID
		body.ExpectedCandidateId = &value
	}
	if request.ExpectedArtifactDigest != "" {
		value := request.ExpectedArtifactDigest
		body.ExpectedArtifactDigest = &value
	}
	for index, artifact := range request.Artifacts {
		body.Artifacts[index] = deploymentgen.CandidateSourceArtifact{
			Path: artifact.Path, Digest: artifact.Digest,
		}
	}
	return body
}

func standardCandidateContentDigest(identity string) string {
	decoded, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(identity), "sha256:"))
	if err != nil || len(decoded) != 32 {
		return ""
	}
	return "sha-256=:" + base64.StdEncoding.EncodeToString(decoded) + ":"
}
