package app

import (
	"encoding/json"
	"reflect"
	"testing"

	agentui "github.com/flidai/leapview/internal/agent/ui"
)

func TestDashboardChatAdapterPreservesBrowserContract(t *testing.T) {
	source := agentui.ChatSignal{
		ActiveConversationID: "conversation-1",
		Composer: agentui.ComposerSignal{
			Disabled: true, Placeholder: "Ask", Value: "Question",
		},
		Conversations: []agentui.ChatConversationSummary{{
			ArchivedAt: agentui.Optional("2026-07-24T00:00:00Z"), CreatedAt: "2026-07-23T00:00:00Z",
			ID: "conversation-1", LastMessageText: agentui.Optional("Answer"), MessageCount: 2,
			PrincipalID: "principal-1", Status: "ready", Title: "Analysis",
			TitlePending: agentui.Pointer(true), UpdatedAt: "2026-07-25T00:00:00Z",
		}},
		Status: agentui.ChatStatus{
			Enabled: true, Error: agentui.Optional("warning"), Running: true,
		},
		Transcript: []agentui.ChatTranscriptItemSignal{{
			ID: "turn-1", Kind: "assistant", Text: agentui.Optional("Answer"),
			Artifact: &agentui.ChatArtifactSignal{
				ID: "visual-1", Type: "visual", Summary: agentui.Optional("Chart"),
			},
			References: &[]agentui.AgentReferenceSignal{{
				Reference: agentui.AgentReferenceKeySignal{
					WorkspaceID: "workspace-1", Type: "visual", ID: "visual-1",
				},
				Name: "Revenue", Description: agentui.Optional("Revenue chart"), VisualType: agentui.Optional("bar"),
				Workspace: agentui.AgentReferenceWorkspaceSignal{ID: "workspace-1", Name: "Sales"},
				Hierarchy: []string{"Sales", "Executive"}, Href: "/visual-1",
				Locations: []agentui.AgentReferenceLocationSignal{{
					DashboardID: agentui.Optional("dashboard-1"), DashboardName: agentui.Optional("Executive"),
					PageID: agentui.Optional("page-1"), PageName: agentui.Optional("Overview"), Href: "/visual-1",
				}},
				Context: []string{"revenue"},
			}},
		}},
	}

	got := dashboardChatSignal(source)

	var sourceJSON any
	sourceBytes, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(sourceBytes, &sourceJSON); err != nil {
		t.Fatal(err)
	}
	var gotJSON any
	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(gotBytes, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, sourceJSON) {
		t.Fatalf("dashboard adapter changed browser contract:\nsource=%s\ngot=%s", sourceBytes, gotBytes)
	}
}
