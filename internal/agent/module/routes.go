package module

import (
	"net/http"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/go-chi/chi/v5"
)

type RouteGuard struct {
	Protect       func(access.Privilege, http.HandlerFunc) http.HandlerFunc
	ProtectGlobal func(access.Privilege, http.HandlerFunc) http.HandlerFunc
}

func (m *Module) MountAuthenticated(r chi.Router, guard RouteGuard) {
	if m == nil || m.handler == nil {
		return
	}
	h := m.handler
	r.Get("/chats", guard.ProtectGlobal(access.PrivilegeViewAgent, h.Chat))
	r.Get("/chats/new", guard.ProtectGlobal(access.PrivilegeViewAgent, h.ChatNew))
	r.Get("/chats/references/search", guard.ProtectGlobal(access.PrivilegeViewItem, h.ChatReferenceSearch))
	r.Get("/chats/restore", guard.ProtectGlobal(access.PrivilegeViewAgent, h.ChatRestore))
	r.Get("/chats/{conversation}", guard.ProtectGlobal(access.PrivilegeViewAgent, h.ChatConversation))
	r.Post("/chats/turns", guard.ProtectGlobal(access.PrivilegeUseAgent, h.ChatTurn))
	r.Patch("/admin/agent/config", guard.Protect(access.PrivilegeManageGrants, h.UpdateAdminConfig))
}
