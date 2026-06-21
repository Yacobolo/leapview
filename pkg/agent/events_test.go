package agent

import (
	"context"
	"testing"
)

func TestStreamingDeltasOnlyEmitForTurnRequests(t *testing.T) {
	events := &recordingEvents{}
	model := ModelFunc(func(ctx context.Context, req ModelRequest, stream ModelStream) (ModelResponse, error) {
		if err := stream.Delta(ctx, "hello"); err != nil {
			return ModelResponse{}, err
		}
		return ModelResponse{Content: "hello", FinishReason: FinishReasonStop}, nil
	})
	a := mustAgent(t, Definition{
		Name:         "test",
		SystemPrompt: "x",
		Model:        model,
		Events:       events,
	})

	if _, err := a.Prompt(context.Background(), PromptRequest{Input: "go"}); err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}

	foundDelta := false
	for _, event := range events.events {
		if event.Type == EventTypeMessageDelta && event.Delta == "hello" {
			foundDelta = true
			if event.Severity != SeverityInfo {
				t.Fatalf("delta severity = %s, want info", event.Severity)
			}
		}
	}
	if !foundDelta {
		t.Fatalf("events = %s, want message_delta", eventTypes(events.events))
	}
}
