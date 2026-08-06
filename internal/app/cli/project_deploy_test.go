package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	"github.com/flidai/leapview/internal/platform/cliapi"
	projectcli "github.com/flidai/leapview/internal/project/cli"
	"github.com/stretchr/testify/require"
)

func TestDeployUsesExactCandidateSynchronizationAndPublicationLifecycle(t *testing.T) {
	client := &deployLifecycleClient{environment: "prod"}
	lifecycle := &deployLifecycleRecorder{}
	operations := projectDeployOperations{client: client, lifecycle: lifecycle}
	credentials := cliapi.Credentials{Target: "https://example.test", Token: "secret"}

	err := operations.Deploy(context.Background(), projectcli.DeployOptions{
		ProjectPath: "dashboards/leapview.yaml", Credentials: credentials, Environment: "prod",
	}, &bytes.Buffer{})
	require.NoError(t, err)
	require.Equal(t, []string{"synchronize", "publish"}, lifecycle.order)
	require.Equal(t, "prod", client.assertedEnvironment)
	require.Equal(t, projectDeploymentCandidateKey, lifecycle.dev.CandidateKey)
	require.Equal(t, projectDeploymentCandidateKey, lifecycle.publish.CandidateKey)
	require.Equal(t, credentials, lifecycle.dev.Credentials)
	require.Equal(t, credentials, lifecycle.publish.Credentials)
	require.True(t, lifecycle.dev.Once)
	require.True(t, lifecycle.dev.NoBrowser)
}

func TestDeployDoesNotPublishWhenCandidateSynchronizationFails(t *testing.T) {
	syncErr := errors.New("synchronization failed")
	lifecycle := &deployLifecycleRecorder{syncErr: syncErr}
	operations := projectDeployOperations{
		client: &deployLifecycleClient{environment: "prod"}, lifecycle: lifecycle,
	}

	err := operations.Deploy(context.Background(), projectcli.DeployOptions{
		ProjectPath: "dashboards/leapview.yaml",
		Credentials: cliapi.Credentials{Target: "https://example.test", Token: "secret"},
	}, &bytes.Buffer{})
	require.ErrorIs(t, err, syncErr)
	require.Equal(t, []string{"synchronize"}, lifecycle.order)
}

func TestDeployAdapterCannotBypassCandidatePublicationPipeline(t *testing.T) {
	source, err := os.ReadFile("deploy.go")
	require.NoError(t, err)
	body := string(source)
	for _, required := range []string{"projectcli.RunDev", "projectcli.RunPublish"} {
		require.Contains(t, body, required)
	}
	for _, forbidden := range []string{"createRelease(", "createDeployment("} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("deploy adapter bypasses candidate publication via %s", forbidden)
		}
	}
}

type deployLifecycleClient struct {
	environment         string
	assertedEnvironment string
}

func (client *deployLifecycleClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	return credentials, nil
}

func (client *deployLifecycleClient) Environment(_ context.Context, _ cliapi.Credentials, asserted string) (string, error) {
	client.assertedEnvironment = asserted
	return client.environment, nil
}

func (*deployLifecycleClient) Transport(context.Context, cliapi.Credentials) (apigenclient.Transport, error) {
	return nil, nil
}

type deployLifecycleRecorder struct {
	order      []string
	dev        projectcli.DevOptions
	publish    projectcli.PublishOptions
	syncErr    error
	publishErr error
}

func (lifecycle *deployLifecycleRecorder) Synchronize(
	_ context.Context,
	options projectcli.DevOptions,
	_, _ io.Writer,
) error {
	lifecycle.order = append(lifecycle.order, "synchronize")
	lifecycle.dev = options
	return lifecycle.syncErr
}

func (lifecycle *deployLifecycleRecorder) Publish(
	_ context.Context,
	options projectcli.PublishOptions,
	_ io.Writer,
) error {
	lifecycle.order = append(lifecycle.order, "publish")
	lifecycle.publish = options
	return lifecycle.publishErr
}
