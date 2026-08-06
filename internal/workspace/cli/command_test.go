package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	"github.com/flidai/leapview/internal/platform/cliapi"
	workspacegen "github.com/flidai/leapview/internal/workspace/api/gen"
)

type fakeClient struct {
	transport fakeTransport
}

type fakeTransport struct {
	requests []apigenclient.Request
	do       func(apigenclient.Request, any) error
}

func (client *fakeClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	return credentials, nil
}

func (client *fakeClient) Environment(_ context.Context, _ cliapi.Credentials, asserted string) (string, error) {
	return asserted, nil
}

func (client *fakeClient) Transport(_ context.Context, _ cliapi.Credentials) (apigenclient.Transport, error) {
	return &client.transport, nil
}

func (transport *fakeTransport) DoAPIGen(_ context.Context, request apigenclient.Request, out any) (apigenclient.Response, error) {
	transport.requests = append(transport.requests, request)
	if transport.do != nil {
		if err := transport.do(request, out); err != nil {
			return apigenclient.Response{}, err
		}
	}
	return apigenclient.Response{StatusCode: 200}, nil
}

func TestWorkspacesCommandOwnsListRequest(t *testing.T) {
	client := &fakeClient{}
	command := WorkspacesCommand(context.Background(), client)
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"list", "--target", "https://example.test", "--token", "secret", "--limit", "7", "--page-token", "cursor"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(client.transport.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.transport.requests))
	}
	request := client.transport.requests[0]
	if request.OperationID != workspacegen.GenOperationListWorkspaces || request.Query.Get("limit") != "7" || request.Query.Get("pageToken") != "cursor" {
		t.Fatalf("request = %#v", request)
	}
}

func TestSearchCommandOwnsFiltersAndPresentation(t *testing.T) {
	client := &fakeClient{}
	client.transport.do = func(request apigenclient.Request, out any) error {
		return json.Unmarshal([]byte(`{"items":[{"context":[],"href":"/orders","locations":[],"name":"Orders","reference":{"workspaceId":"sales","type":"visual","id":"orders"},"workspace":{"id":"sales","name":"Sales"}}],"page":{"nextCursor":""}}`), out)
	}
	command := SearchCommand(context.Background(), client)
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"orders", "--target", "https://example.test", "--token", "secret", "--workspace", "sales,finance", "--type", "visual", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	request := client.transport.requests[0]
	if request.OperationID != workspacegen.GenOperationSearch || request.Query.Get("q") != "orders" {
		t.Fatalf("request = %#v", request)
	}
	if got := request.Query["workspace"]; len(got) != 2 || got[0] != "sales" || got[1] != "finance" {
		t.Fatalf("workspace filters = %#v", got)
	}
	if got := request.Query["type"]; len(got) != 1 || got[0] != "visual" {
		t.Fatalf("type filters = %#v", got)
	}
	var response workspacegen.GenSchemaSearchResponse
	if err := json.Unmarshal([]byte(output.String()), &response); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].Reference.Id != "orders" {
		t.Fatalf("response = %#v", response)
	}
}
