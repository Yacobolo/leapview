package module

import (
	dashboardui "github.com/Yacobolo/leapview/internal/dashboard/ui"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
	uisignals "github.com/Yacobolo/leapview/internal/workspace/ui/signals"
)

func ChatChromeDecorators(signal ui.ChatSignal) []dashboardui.ChromeDecorator {
	return []dashboardui.ChromeDecorator{
		func(chrome *uisignals.ChromeSignal) {
			uisignals.AttachChatSidebar(&chrome.Sidebar, signal)
		},
	}
}
