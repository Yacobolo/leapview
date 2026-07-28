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

func (client *fakeClient) Environment(_ context.Context, _ cliapi.Credentials, asserted string) (string, error) {
	return asserted, nil
}

func (client *fakeClient) DoJSON(_ context.Context, _ cliapi.Credentials, request cliapi.Request, out any) error {
	client.requests = append(client.requests, request)
	return nil
}

func TestCommandOwnsSemanticQuery(t *testing.T) {
	client := &fakeClient{}
	command := Command(context.Background(), client, "sales")
	command.SetArgs([]string{
		"query", "orders",
		"--target", "https://example.test", "--token", "secret",
		"--body-json", `{"dimensions":[{"field":"state"}]}`,
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.requests))
	}
	request := client.requests[0]
	if request.OperationID != "querySemanticModel" ||
		request.PathParams["workspace"] != "sales" ||
		request.PathParams["model"] != "orders" {
		t.Fatalf("request = %#v", request)
	}
	body := request.Body.(map[string]any)
	if len(body["dimensions"].([]any)) != 1 {
		t.Fatalf("body = %#v", body)
	}
}
