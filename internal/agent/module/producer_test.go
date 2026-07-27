package module

import (
	"context"
	"testing"

	"github.com/Yacobolo/leapview/internal/agent"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
)

type runJobStore struct {
	input jobs.EnqueueInput
	event jobs.Event
}

func (s *runJobStore) Enqueue(_ context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	s.input = input
	return jobs.Job{ID: input.ID}, nil
}

func (s *runJobStore) AppendEvent(_ context.Context, kind, id, eventType string, data []byte) (jobs.Event, error) {
	s.event = jobs.Event{ResourceKind: kind, ResourceID: id, EventType: eventType, Data: data}
	return s.event, nil
}

func (s *runJobStore) Cancel(context.Context, string) error { return nil }

func TestEnqueueRunOwnsWorkloadAndWorkspaceClassification(t *testing.T) {
	store := &runJobStore{}
	module, err := Build(t.Context(), Config{Jobs: store, DefaultWorkspaceID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	started := &agent.StartedPrompt{ConversationID: "conversation-1", RunID: "run-1", CorrelationID: "correlation-1"}
	scope := agent.Scope{Credential: agent.CredentialScope{WorkspaceID: "credential-workspace"}}
	if err := module.EnqueueRun(t.Context(), scope, started); err != nil {
		t.Fatal(err)
	}
	if store.input.WorkloadClass != "background" || store.input.WorkspaceID != "credential-workspace" {
		t.Fatalf("workload = %q/%q", store.input.WorkloadClass, store.input.WorkspaceID)
	}
	if store.input.Kind != RunJobKind || store.input.ID != "agent:run-1:run" {
		t.Fatalf("job = %#v", store.input)
	}
	if store.event.EventType != "agent_run.queued" {
		t.Fatalf("event = %#v", store.event)
	}
}

func TestBuildConstructsOwnedHTTPHandler(t *testing.T) {
	module, err := Build(t.Context(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if module.HTTP() == nil {
		t.Fatal("expected agent module to construct its HTTP handler")
	}
}

func TestRunWorkspaceFallsBackToDefaultThenGlobal(t *testing.T) {
	if got := runWorkspaceID(agent.Scope{WorkspaceID: "scope", Credential: agent.CredentialScope{WorkspaceID: "credential"}}, "default", "_global"); got != "scope" {
		t.Fatalf("scope workspace = %q", got)
	}
	if got := runWorkspaceID(agent.Scope{}, "default", "_global"); got != "default" {
		t.Fatalf("default workspace = %q", got)
	}
	if got := runWorkspaceID(agent.Scope{}, "", "_global"); got != "_global" {
		t.Fatalf("global workspace = %q", got)
	}
}
