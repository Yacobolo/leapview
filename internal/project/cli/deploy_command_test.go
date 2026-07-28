package cli

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform/cliapi"
)

type deployClient struct{}

func (deployClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	return credentials, nil
}
func (deployClient) Environment(_ context.Context, _ cliapi.Credentials, asserted string) (string, error) {
	return asserted, nil
}
func (deployClient) DoJSON(context.Context, cliapi.Credentials, cliapi.Request, any) error {
	return nil
}

type deployOperations struct {
	options DeployOptions
}

func (operations *deployOperations) Deploy(_ context.Context, options DeployOptions, _ io.Writer) error {
	operations.options = options
	return nil
}

func TestDeployCommandOwnsRevisionParsing(t *testing.T) {
	operations := &deployOperations{}
	command := DeployCommand(context.Background(), deployClient{}, operations, "")
	command.SetArgs([]string{
		"--target", "https://example.test", "--token", "secret",
		"--revision", "orders=sha256:" + strings.Repeat("a", 64),
		"--auto-approve",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if operations.options.Revisions["orders"] == "" || !operations.options.AutoApprove {
		t.Fatalf("options = %#v", operations.options)
	}
}
