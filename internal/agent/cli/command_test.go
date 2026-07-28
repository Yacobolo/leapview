package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/agent/api"
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
	return client.do(request, out)
}

func TestCommandRunsAgentConversationWithoutApplicationProcess(t *testing.T) {
	client := &fakeClient{}
	client.do = func(request cliapi.Request, out any) error {
		switch request.OperationID {
		case "createAgentConversation":
			*out.(*api.AgentConversationResponse) = api.AgentConversationResponse{ID: "conv_1"}
		case "createAgentRun":
			*out.(*api.AgentRunResponse) = api.AgentRunResponse{ID: "run_1", Status: "completed", StopReason: "complete"}
		case "listAgentMessages":
			*out.(*listResponse[api.AgentMessageResponse]) = listResponse[api.AgentMessageResponse]{
				Items: []api.AgentMessageResponse{{RunID: "run_1", Role: "assistant", ContentText: "Answer"}},
			}
		default:
			t.Fatalf("unexpected operation %q", request.OperationID)
		}
		return nil
	}
	command := Command(context.Background(), Dependencies{Client: client})
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"ask", "Question", "--target", "https://example.test", "--token", "secret"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, "Answer") || !strings.Contains(got, "conversation=conv_1 run=run_1") {
		t.Fatalf("output = %q", got)
	}
	if len(client.requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(client.requests))
	}
	if client.requests[0].Headers["Idempotency-Key"] == "" || client.requests[1].Headers["Idempotency-Key"] == "" {
		t.Fatal("mutating Agent requests must carry idempotency keys")
	}
}

func TestCommandOwnsConversationEnvelopePresentation(t *testing.T) {
	client := &fakeClient{}
	client.do = func(request cliapi.Request, out any) error {
		if request.OperationID != "listAgentConversations" {
			t.Fatalf("operation = %q", request.OperationID)
		}
		*out.(*listResponse[api.AgentConversationResponse]) = listResponse[api.AgentConversationResponse]{
			Items: []api.AgentConversationResponse{{ID: "conv_1", Title: "Ask", Status: "active"}},
		}
		return nil
	}
	command := Command(context.Background(), Dependencies{Client: client})
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"conversations", "--json", "--target", "https://example.test", "--token", "secret", "--limit", "7", "--page-token", "cursor"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var rows []api.AgentConversationResponse
	if err := json.Unmarshal([]byte(output.String()), &rows); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "conv_1" {
		t.Fatalf("rows = %#v", rows)
	}
	query := client.requests[0].Query
	if query.Get("limit") != "7" || query.Get("pageToken") != "cursor" {
		t.Fatalf("query = %s", query.Encode())
	}
}
