package module

import (
	dashboardui "github.com/Yacobolo/leapview/internal/dashboard/ui"
	uisignals "github.com/Yacobolo/leapview/internal/dashboard/ui/signals"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
)

func ChatChromeDecorators(signal ui.ChatSignal) []dashboardui.ChromeDecorator {
	return []dashboardui.ChromeDecorator{
		func(chrome *uisignals.ChromeSignal) {
			items := make([]uisignals.SidebarHistoryItemSignal, 0, len(signal.Conversations))
			for _, conversation := range signal.Conversations {
				title := conversation.Title
				if title == "" {
					title = "Conversation"
				}
				items = append(items, uisignals.SidebarHistoryItemSignal{
					ID: conversation.ID, Title: title, Href: "/chats/" + conversation.ID,
					Active: conversation.ID == signal.ActiveConversationID, Pending: conversation.TitlePending,
				})
			}
			chrome.Sidebar.PrimaryAction = &uisignals.SidebarActionSignal{Label: "New chat", Href: "/chats/new", Icon: "plus"}
			chrome.Sidebar.History = &uisignals.SidebarHistorySignal{
				Label: "Chats", EmptyText: uisignals.Optional("No chats yet."), Items: items,
			}
		},
	}
}
