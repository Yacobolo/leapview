package shell

import (
	"testing"

	webpage "github.com/Yacobolo/leapview/internal/platform/web/page"
)

func TestProviderOwnsGlobalNavigationAndAgentHistory(t *testing.T) {
	provider := Provider(Config{
		Presentation: webpage.Presentation{ProductName: "LeapView"},
		RoleLabel:    "Owner", ActiveConversationID: "chat-1",
		Conversations: []Conversation{{ID: "chat-1", Title: "Revenue"}},
	})
	layout := provider(webpage.Context{Active: "admin", ScopeTitle: "Sales", SectionTitle: "Workspace", PageTitle: "Published assets"})
	chrome, ok := layout.Signal.(Chrome)
	if !ok {
		t.Fatalf("signal = %T, want shell.Chrome", layout.Signal)
	}
	if len(chrome.Sidebar.Groups) != 1 || len(chrome.Sidebar.Groups[0].Items) != 6 {
		t.Fatalf("navigation = %#v", chrome.Sidebar.Groups)
	}
	if chrome.Sidebar.History == nil || len(chrome.Sidebar.History.Items) != 1 || !chrome.Sidebar.History.Items[0].Active {
		t.Fatalf("history = %#v", chrome.Sidebar.History)
	}
}

func TestRouteContextSelectsActiveHistoryWithoutOwningHistory(t *testing.T) {
	provider := Provider(Config{
		Presentation:  webpage.Presentation{ProductName: "LeapView"},
		Conversations: []Conversation{{ID: "chat-1", Title: "Revenue"}},
	})
	layout := provider(webpage.Context{HistoryID: "chat-1"})
	chrome := layout.Signal.(Chrome)
	if chrome.Sidebar.History == nil || !chrome.Sidebar.History.Items[0].Active {
		t.Fatalf("history = %#v", chrome.Sidebar.History)
	}
}
