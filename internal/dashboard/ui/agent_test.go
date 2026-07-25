package ui

import (
	"testing"

	uisignals "github.com/Yacobolo/leapview/internal/dashboard/ui/signals"
)

func TestAgentChromeDecoratorBuildsDashboardOwnedHistory(t *testing.T) {
	decorate := AgentChromeDecorator(AgentChrome{
		ActiveConversationID: "active",
		Conversations: []AgentConversation{
			{ID: "active", Title: "", TitlePending: uisignals.Pointer(true)},
			{ID: "other", Title: "Other"},
		},
	})
	chrome := uisignals.ChromeSignal{}

	decorate(&chrome)

	if chrome.Sidebar.PrimaryAction == nil || chrome.Sidebar.PrimaryAction.Href != "/chats/new" {
		t.Fatalf("primary action = %#v", chrome.Sidebar.PrimaryAction)
	}
	if chrome.Sidebar.History == nil || len(chrome.Sidebar.History.Items) != 2 {
		t.Fatalf("history = %#v", chrome.Sidebar.History)
	}
	active := chrome.Sidebar.History.Items[0]
	if active.Title != "Conversation" || active.Href != "/chats/active" || !active.Active || active.Pending == nil || !*active.Pending {
		t.Fatalf("active history item = %#v", active)
	}
}
