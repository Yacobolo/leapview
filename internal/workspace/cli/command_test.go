package cli

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform/cliapi"
)

type fakeClient struct {
	requests []cliapi.Request
	do       func(cliapi.Request, any) error
}

func (client *fakeClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	return credentials, nil
}

func (client *fakeClient) Environment(_ context.Context, _ cliapi.Credentials, asserted string) (string, error) {
	return asserted, nil
}

func (client *fakeClient) DoJSON(_ context.Context, _ cliapi.Credentials, request cliapi.Request, out any) error {
	client.requests = append(client.requests, request)
	if client.do != nil {
		return client.do(request, out)
	}
	return nil
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
	if len(client.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.requests))
	}
	request := client.requests[0]
	if request.OperationID != "listWorkspaces" || request.Query.Get("limit") != "7" || request.Query.Get("pageToken") != "cursor" {
		t.Fatalf("request = %#v", request)
	}
}

func TestSearchCommandOwnsFiltersAndPresentation(t *testing.T) {
	client := &fakeClient{do: func(request cliapi.Request, out any) error {
		*out.(*searchResponse) = searchResponse{
			Items: []searchResult{{
				Reference: searchReference{WorkspaceID: "sales", Type: "visual", ID: "orders"},
				Name:      "Orders",
			}},
		}
		return nil
	}}
	command := SearchCommand(context.Background(), client)
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"orders", "--target", "https://example.test", "--token", "secret", "--workspace", "sales,finance", "--type", "visual", "--json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	request := client.requests[0]
	if request.OperationID != "search" || request.Query.Get("q") != "orders" {
		t.Fatalf("request = %#v", request)
	}
	if got := request.Query["workspace"]; !equalStrings(got, []string{"sales", "finance"}) {
		t.Fatalf("workspace filters = %#v", got)
	}
	var response searchResponse
	if err := json.Unmarshal([]byte(output.String()), &response); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].Reference.ID != "orders" {
		t.Fatalf("response = %#v", response)
	}
}

func equalStrings(got, want []string) bool {
	return url.Values{"got": got}.Encode() == url.Values{"got": want}.Encode()
}
