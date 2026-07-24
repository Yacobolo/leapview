package ui

import (
	"testing"

	"github.com/Yacobolo/leapview/internal/agent"
	"github.com/Yacobolo/leapview/internal/workspace/navigation"
)

func TestChatTranscriptItemsPreserveAgentOwnedWireState(t *testing.T) {
	items := ChatTranscriptItems([]agent.ChatTranscriptItem{{
		ID:   "message-1",
		Kind: "assistant",
		Text: "Hello",
		Artifact: &agent.ChatArtifact{
			Type:    "visualization",
			ID:      "visual-1",
			Summary: "A chart",
		},
		References: []agent.TurnReference{{
			Reference: agent.TurnReferenceKey{WorkspaceID: "sales", Type: "measure", ID: "revenue"},
			Name:      "Revenue",
			Workspace: agent.TurnReferenceWorkspace{ID: "sales", Name: "Sales"},
		}},
	}})

	if len(items) != 1 || items[0].Text == nil || *items[0].Text != "Hello" {
		t.Fatalf("transcript items = %#v", items)
	}
	if items[0].Artifact == nil || items[0].Artifact.ID != "visual-1" {
		t.Fatalf("artifact = %#v", items[0].Artifact)
	}
	if items[0].References == nil || len(*items[0].References) != 1 {
		t.Fatalf("references = %#v", items[0].References)
	}
}

func TestChatBootstrapSignalsAreOwnedByAgent(t *testing.T) {
	state := ChatViewState{Agent: ChatSignal{
		Conversations: []ChatConversationSummary{{ID: "conversation-1", Title: "Revenue"}},
		Transcript:    []ChatTranscriptItemSignal{},
		Status:        ChatStatus{Enabled: true},
	}}

	signals := ChatBootstrapSignals(navigation.Catalog{}, "", "viewer", "list", state)

	runtime, ok := signals["runtime"].(RouteRuntimeSignal)
	if !ok || runtime.Kind != RouteChat {
		t.Fatalf("runtime = %#v", signals["runtime"])
	}
	chrome, ok := signals["chrome"].(ChromeSignal)
	if !ok || chrome.Sidebar.History == nil || len(chrome.Sidebar.History.Items) != 1 {
		t.Fatalf("chrome = %#v", signals["chrome"])
	}
	if chrome.Sidebar.History.Items[0].Href != "/chats/conversation-1" {
		t.Fatalf("history item = %#v", chrome.Sidebar.History.Items[0])
	}
}
