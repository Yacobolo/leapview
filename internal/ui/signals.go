package ui

import (
	"github.com/Yacobolo/libredash/internal/agentapp"
	"github.com/Yacobolo/libredash/internal/api"
)

type WorkspaceAccessResponse struct {
	Workspace api.WorkspaceResponse     `json:"workspace"`
	Roles     []api.RoleResponse        `json:"roles"`
	Bindings  []api.RoleBindingResponse `json:"bindings"`
	CanManage bool                      `json:"canManage"`
	Status    WorkspaceAccessStatus     `json:"status"`
}

type WorkspaceAccessStatus struct {
	Loading bool   `json:"loading"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type WorkspaceAccessCommand struct {
	Email       string `json:"email"`
	Role        string `json:"role"`
	PrincipalID string `json:"principalId"`
}

type ChatSignal struct {
	Conversations        []api.AgentConversationResponse `json:"conversations"`
	ActiveConversationID string                          `json:"activeConversationId"`
	Transcript           []agentapp.ChatTranscriptItem   `json:"transcript"`
	Status               ChatStatus                      `json:"status"`
	Composer             ComposerSignal                  `json:"composer"`
}

type ChatStatus struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

type ComposerSignal struct {
	Value       string `json:"value"`
	Disabled    bool   `json:"disabled"`
	Placeholder string `json:"placeholder"`
}
