package cli

import (
	"context"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform/cliapi"
)

type fakeClient struct {
	requests []cliapi.Request
}

func (client *fakeClient) Resolve(_ context.Context, credentials cliapi.Credentials) (cliapi.Credentials, error) {
	return credentials, nil
}

func (client *fakeClient) DoJSON(_ context.Context, _ cliapi.Credentials, request cliapi.Request, out any) error {
	client.requests = append(client.requests, request)
	return nil
}

func TestCommandOwnsDashboardVisualQuery(t *testing.T) {
	client := &fakeClient{}
	command := Command(context.Background(), client, "sales")
	command.SetArgs([]string{
		"visual-data", "executive", "overview", "orders",
		"--target", "https://example.test", "--token", "secret",
		"--count", "7", "--filter-state-json", `{"version":"typed_v1"}`,
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.requests))
	}
	request := client.requests[0]
	if request.OperationID != "queryDashboardVisualData" {
		t.Fatalf("operation = %q", request.OperationID)
	}
	if request.PathParams["workspace"] != "sales" || request.PathParams["dashboard"] != "executive" ||
		request.PathParams["page"] != "overview" || request.PathParams["visual"] != "orders" {
		t.Fatalf("path params = %#v", request.PathParams)
	}
	body := request.Body.(map[string]any)
	if body["limit"] != 7 {
		t.Fatalf("body = %#v", body)
	}
	filterState := body["filterState"].(map[string]any)
	if filterState["version"] != "typed_v1" {
		t.Fatalf("filter state = %#v", filterState)
	}
}
