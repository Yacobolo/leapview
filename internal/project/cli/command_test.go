package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/flidai/leapview/internal/workspace"
)

type fakeActiveWorkspaceGraphLoader struct {
	credentials cliapi.Credentials
	environment string
	workspaceID string
	graph       workspace.AssetGraph
}

func (loader *fakeActiveWorkspaceGraphLoader) LoadActiveWorkspaceGraph(
	_ context.Context,
	credentials cliapi.Credentials,
	environment string,
	workspaceID string,
) (workspace.AssetGraph, error) {
	loader.credentials = credentials
	loader.environment = environment
	loader.workspaceID = workspaceID
	return loader.graph, nil
}

func TestValidateCommandOwnsProjectArgumentRules(t *testing.T) {
	command := ValidateCommand(context.Background())
	command.SetArgs([]string{"project.yaml", "--project", "other.yaml"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "choose either --project or positional project") {
		t.Fatalf("error = %v", err)
	}
}

func TestSchemaCommandRejectsUnknownFormats(t *testing.T) {
	command := SchemaCommand()
	command.SetArgs([]string{"export", "--format", "yaml"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), `unsupported schema format "yaml"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestActiveWorkspaceGraphUsesCompositionSuppliedLoader(t *testing.T) {
	want := workspace.AssetGraph{Assets: []workspace.Asset{{Key: "orders"}}}
	loader := &fakeActiveWorkspaceGraphLoader{graph: want}
	options := &options{
		remote:      cliapi.RemoteOptions{Target: "https://example.test", Token: "secret"},
		environment: "production",
	}

	got, err := fetchActiveWorkspaceGraphFor(context.Background(), loader, options, "sales")
	if err != nil {
		t.Fatalf("fetch active graph: %v", err)
	}
	if len(got.Assets) != 1 || got.Assets[0].Key != "orders" {
		t.Fatalf("graph = %#v", got)
	}
	if loader.credentials.Target != "https://example.test" ||
		loader.credentials.Token != "secret" ||
		loader.environment != "production" ||
		loader.workspaceID != "sales" {
		t.Fatalf("loader request = %#v", loader)
	}
}
