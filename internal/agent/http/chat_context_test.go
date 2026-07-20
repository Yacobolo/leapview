package http

import (
	"testing"

	"github.com/Yacobolo/libredash/internal/ui"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
)

func TestChatSignalPatchKeepsEmbeddedArtifactsSeparateFromDashboardVisuals(t *testing.T) {
	state := ui.ChatViewState{
		Agent:   ui.ChatSignal{Conversations: []ui.ChatConversationSummary{}, Transcript: []ui.ChatTranscriptItemSignal{}},
		Visuals: map[string]uisignals.DashboardVisual{},
	}
	embedded := chatSignalPatch(state, true)
	if _, ok := embedded["agentVisuals"]; !ok {
		t.Fatalf("embedded patch = %#v, want agentVisuals", embedded)
	}
	if _, ok := embedded["visuals"]; ok {
		t.Fatalf("embedded patch = %#v, must not replace dashboard visuals", embedded)
	}
	standalone := chatSignalPatch(state, false)
	if _, ok := standalone["visuals"]; !ok {
		t.Fatalf("standalone patch = %#v, want visuals", standalone)
	}
}
