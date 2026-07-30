package cli

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/flidai/leapview/internal/project/devloop"
)

type devCommandClient struct {
	resolved cliapi.Credentials
}

func (client *devCommandClient) Resolve(
	_ context.Context,
	credentials cliapi.Credentials,
) (cliapi.Credentials, error) {
	if credentials.Target != "prod" {
		return cliapi.Credentials{}, io.ErrUnexpectedEOF
	}
	client.resolved = cliapi.Credentials{
		Target: "https://prod.example.com",
		Token:  "ephemeral",
	}
	return client.resolved, nil
}

func (*devCommandClient) Environment(
	context.Context,
	cliapi.Credentials,
	string,
) (string, error) {
	return "", nil
}

func (*devCommandClient) Transport(
	context.Context,
	cliapi.Credentials,
) (apigenclient.Transport, error) {
	return nil, nil
}

type devRemoteFactory struct {
	credentials cliapi.Credentials
	concurrency int
}

func (factory *devRemoteFactory) Remote(
	_ context.Context,
	credentials cliapi.Credentials,
	concurrency int,
) (devloop.Remote, error) {
	factory.credentials = credentials
	factory.concurrency = concurrency
	return devCommandRemote{}, nil
}

type devCommandRemote struct{}

func (devCommandRemote) Synchronize(
	_ context.Context,
	request devloop.SyncRequest,
) (devloop.Candidate, error) {
	return devloop.Candidate{
		ID:               "cand_1",
		ProjectID:        request.Snapshot.ProjectID,
		OwnerID:          "principal_ci",
		ArtifactDigest:   request.Snapshot.Digest,
		PreviewURL:       "https://prod.example.com/candidates/cand_1",
		TargetID:         "lvinst_prod",
		Environment:      "production",
		ProvenanceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Revision:         7,
	}, nil
}

func TestDevCommandOwnsOneAuthenticatedRemoteWorkflow(t *testing.T) {
	projectPath := filepath.Join("..", "..", "..", "dashboards", "leapview.yaml")
	checkpoints := NewCandidateCheckpointStore(
		filepath.Join(t.TempDir(), "authoring.json"),
	)
	client := &devCommandClient{}
	remotes := &devRemoteFactory{}
	command := DevCommand(t.Context(), client, checkpoints, remotes)
	var output strings.Builder
	command.SetOut(&output)
	command.SetErr(io.Discard)
	command.SetArgs([]string{
		"--once",
		"--project", projectPath,
		"--target", "prod",
		"--upload-concurrency", "3",
		"--candidate-key", "github:pull/42",
		"--source-revision", "commit-a",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if remotes.credentials != client.resolved || remotes.concurrency != 3 {
		t.Fatalf(
			"remote credentials=%+v concurrency=%d",
			remotes.credentials,
			remotes.concurrency,
		)
	}
	checkpoint, err := checkpoints.LoadCandidate(
		projectPath,
		client.resolved.Target,
		"github:pull/42",
	)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.TargetID != "lvinst_prod" ||
		checkpoint.CandidateID != "cand_1" ||
		checkpoint.CandidateKey != "github:pull/42" ||
		checkpoint.CandidateRevision != 7 {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
	if !strings.Contains(output.String(), "synchronized sha256:") ||
		!strings.Contains(
			output.String(),
			"candidate cand_1 revision 7 target lvinst_prod environment production principal principal_ci",
		) ||
		!strings.Contains(
			output.String(),
			"preview https://prod.example.com/candidates/cand_1",
		) {
		t.Fatalf("output = %q", output.String())
	}
	for _, flag := range []string{
		"project",
		"target",
		"token",
		"upload-concurrency",
		"once",
	} {
		if command.Flags().Lookup(flag) == nil {
			t.Errorf("dev command is missing --%s", flag)
		}
	}
	for _, forbidden := range []string{
		"local-server",
		"production",
		"workspace",
	} {
		if command.Flags().Lookup(forbidden) != nil {
			t.Errorf("dev command exposes alternate workflow --%s", forbidden)
		}
	}
}
