package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	workspacegen "github.com/flidai/leapview/internal/workspace/api/gen"
)

func TestCapabilityAPITransportExecutesGeneratedTypedClient(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/workspaces" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("limit") != "7" || request.URL.Query().Get("pageToken") != "cursor" {
			t.Fatalf("query = %q", request.URL.RawQuery)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("accept = %q", request.Header.Get("Accept"))
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Request-ID", "request-1")
		_, _ = writer.Write([]byte(`{"items":[],"page":{"nextCursor":""}}`))
	}))
	defer server.Close()

	limit := int32(7)
	pageToken := "cursor"
	client := workspacegen.NewGenClient(capabilityAPITransport{
		target: server.URL,
		token:  "secret",
		client: server.Client(),
	})
	response, err := client.ListWorkspaces(context.Background(), workspacegen.GenListWorkspacesClientRequest{
		Params: workspacegen.GenListWorkspacesClientParams{
			Limit:     &limit,
			PageToken: &pageToken,
		},
	})
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if response.StatusCode != http.StatusOK || response.Headers.Get("X-Request-ID") != "request-1" {
		t.Fatalf("response metadata = %#v", response)
	}
	if len(response.Body.Items) != 0 {
		t.Fatalf("items = %#v", response.Body.Items)
	}
}
