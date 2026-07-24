package ui

import (
	"net/url"
	"strings"

	"github.com/Yacobolo/leapview/internal/agent"
	visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"
	"github.com/Yacobolo/leapview/internal/workspace/navigation"
)

type RouteKind string

const RouteChat RouteKind = "chat"

type RouteRuntimeSignal struct {
	ClientID         *string   `json:"clientId,omitempty"`
	DashboardID      *string   `json:"dashboardId,omitempty"`
	Kind             RouteKind `json:"kind"`
	ModelID          *string   `json:"modelId,omitempty"`
	PageID           *string   `json:"pageId,omitempty"`
	StreamInstanceID *string   `json:"streamInstanceId,omitempty"`
	WorkspaceID      *string   `json:"workspaceId,omitempty"`
}

type AgentReferenceKeySignal struct {
	WorkspaceID string `json:"workspaceId"`
	Type        string `json:"type"`
	ID          string `json:"id"`
}

type AgentReferenceLocationSignal struct {
	DashboardID   *string `json:"dashboardId,omitempty"`
	DashboardName *string `json:"dashboardName,omitempty"`
	PageID        *string `json:"pageId,omitempty"`
	PageName      *string `json:"pageName,omitempty"`
	Href          string  `json:"href"`
}

type AgentReferenceSearchSignal struct {
	Query     string                 `json:"query"`
	RequestID int64                  `json:"requestId"`
	Results   []AgentReferenceSignal `json:"results"`
}

type AgentReferenceSignal struct {
	Reference   AgentReferenceKeySignal        `json:"reference"`
	Name        string                         `json:"name"`
	Description *string                        `json:"description,omitempty"`
	VisualType  *string                        `json:"visualType,omitempty"`
	Workspace   AgentReferenceWorkspaceSignal  `json:"workspace"`
	Hierarchy   []string                       `json:"hierarchy"`
	Href        string                         `json:"href"`
	Locations   []AgentReferenceLocationSignal `json:"locations"`
	Context     []string                       `json:"context"`
}

type AgentReferenceWorkspaceSignal struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ChatArtifactSignal struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Summary *string `json:"summary,omitempty"`
}

type ChatConversationSummary struct {
	ArchivedAt      *string `json:"archivedAt,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	ID              string  `json:"id"`
	LastMessageText *string `json:"lastMessageText,omitempty"`
	MessageCount    int64   `json:"messageCount"`
	PrincipalID     string  `json:"principalId"`
	Status          string  `json:"status"`
	Title           string  `json:"title"`
	TitlePending    *bool   `json:"titlePending,omitempty"`
	UpdatedAt       string  `json:"updatedAt"`
}

type ChatSignal struct {
	ActiveConversationID string                     `json:"activeConversationId"`
	Composer             ComposerSignal             `json:"composer"`
	Conversations        []ChatConversationSummary  `json:"conversations"`
	Status               ChatStatus                 `json:"status"`
	Transcript           []ChatTranscriptItemSignal `json:"transcript"`
}

type ChatStatus struct {
	Enabled bool    `json:"enabled"`
	Error   *string `json:"error,omitempty"`
	Running bool    `json:"running"`
}

type ChatTranscriptItemSignal struct {
	ArgumentsJSON  *string                 `json:"argumentsJson,omitempty"`
	Artifact       *ChatArtifactSignal     `json:"artifact,omitempty"`
	ConversationID *string                 `json:"conversationId,omitempty"`
	CreatedAt      *string                 `json:"createdAt,omitempty"`
	Error          *string                 `json:"error,omitempty"`
	ID             string                  `json:"id"`
	InputFormat    *string                 `json:"inputFormat,omitempty"`
	InputJSON      *string                 `json:"inputJson,omitempty"`
	Kind           string                  `json:"kind"`
	Markdown       *string                 `json:"markdown,omitempty"`
	Name           *string                 `json:"name,omitempty"`
	References     *[]AgentReferenceSignal `json:"references,omitempty"`
	ResultFormat   *string                 `json:"resultFormat,omitempty"`
	ResultJSON     *string                 `json:"resultJson,omitempty"`
	ResultSummary  *string                 `json:"resultSummary,omitempty"`
	RunID          *string                 `json:"runId,omitempty"`
	Status         *string                 `json:"status,omitempty"`
	Summary        *string                 `json:"summary,omitempty"`
	Text           *string                 `json:"text,omitempty"`
	Title          *string                 `json:"title,omitempty"`
	ToolCallID     *string                 `json:"toolCallId,omitempty"`
}

type ComposerSignal struct {
	Disabled    bool   `json:"disabled"`
	Placeholder string `json:"placeholder"`
	Value       string `json:"value"`
}

type ChatViewState struct {
	Agent   ChatSignal
	Visuals map[string]visualizationir.VisualizationEnvelope
}

type AgentContextSignal struct {
	Surface        string                 `json:"surface"`
	WorkspaceID    string                 `json:"workspaceId"`
	DashboardID    string                 `json:"dashboardId"`
	DashboardTitle string                 `json:"dashboardTitle"`
	PageID         string                 `json:"pageId"`
	PageTitle      string                 `json:"pageTitle"`
	ModelID        string                 `json:"modelId"`
	Generation     int64                  `json:"generation"`
	Filters        DashboardFilters       `json:"filters"`
	ReferenceLimit int32                  `json:"referenceLimit"`
	References     []AgentReferenceSignal `json:"references"`
}

type DashboardFilters struct {
	Controls          map[string]any `json:"controls"`
	Selections        []any          `json:"selections"`
	SpatialSelections []any          `json:"spatialSelections"`
}

type ChatPageSignal struct {
	Description string    `json:"description"`
	Kind        RouteKind `json:"kind"`
	Title       string    `json:"title"`
	View        string    `json:"view"`
}

type ChromeSignal struct {
	Sidebar SidebarSignal `json:"sidebar"`
}

type SidebarActionSignal struct {
	Href  string `json:"href"`
	Icon  string `json:"icon"`
	Label string `json:"label"`
}

type SidebarGroupSignal struct {
	Items []SidebarItemSignal `json:"items"`
	Label string              `json:"label"`
}

type SidebarHistoryItemSignal struct {
	Active  bool   `json:"active"`
	Href    string `json:"href"`
	ID      string `json:"id"`
	Pending *bool  `json:"pending,omitempty"`
	Title   string `json:"title"`
}

type SidebarHistorySignal struct {
	EmptyText *string                    `json:"emptyText,omitempty"`
	Items     []SidebarHistoryItemSignal `json:"items"`
	Label     string                     `json:"label"`
}

type SidebarItemSignal struct {
	Active *bool   `json:"active,omitempty"`
	Href   string  `json:"href"`
	Icon   string  `json:"icon"`
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Meta   *string `json:"meta,omitempty"`
}

type SidebarSignal struct {
	Active         string                `json:"active"`
	Compact        bool                  `json:"compact"`
	DashboardID    *string               `json:"dashboardId,omitempty"`
	DashboardTitle string                `json:"dashboardTitle"`
	Groups         []SidebarGroupSignal  `json:"groups"`
	History        *SidebarHistorySignal `json:"history,omitempty"`
	ModelID        *string               `json:"modelId,omitempty"`
	ModelTitle     *string               `json:"modelTitle,omitempty"`
	PageTitle      string                `json:"pageTitle"`
	PrimaryAction  *SidebarActionSignal  `json:"primaryAction,omitempty"`
	UserRole       *string               `json:"userRole,omitempty"`
	WorkspaceTitle string                `json:"workspaceTitle"`
}

func Optional[T comparable](value T) *T {
	var zero T
	if value == zero {
		return nil
	}
	return &value
}

func Pointer[T any](value T) *T {
	return &value
}

func ChatTranscriptItems(items []agent.ChatTranscriptItem) []ChatTranscriptItemSignal {
	out := make([]ChatTranscriptItemSignal, 0, len(items))
	for _, item := range items {
		out = append(out, chatTranscriptItem(item))
	}
	return out
}

func chatTranscriptItem(item agent.ChatTranscriptItem) ChatTranscriptItemSignal {
	out := ChatTranscriptItemSignal{
		ID: item.ID, Kind: item.Kind, Text: Optional(item.Text), Markdown: Optional(item.Markdown),
		ToolCallID: Optional(item.ToolCallID), Name: Optional(item.Name), Title: Optional(item.Title),
		Status: Optional(item.Status), Summary: Optional(item.Summary), ResultSummary: Optional(item.ResultSummary),
		InputJSON: Optional(item.InputJSON), InputFormat: Optional(item.InputFormat), ArgumentsJSON: Optional(item.ArgumentsJSON),
		ResultJSON: Optional(item.ResultJSON), ResultFormat: Optional(item.ResultFormat), Error: Optional(item.Error),
		ConversationID: Optional(item.ConversationID), RunID: Optional(item.RunID), CreatedAt: Optional(item.CreatedAt),
	}
	if len(item.References) > 0 {
		references := make([]AgentReferenceSignal, 0, len(item.References))
		for _, reference := range item.References {
			references = append(references, referenceSignalFromTurn(reference))
		}
		out.References = &references
	}
	if item.Artifact != nil {
		out.Artifact = &ChatArtifactSignal{Type: item.Artifact.Type, ID: item.Artifact.ID, Summary: Optional(item.Artifact.Summary)}
	}
	return out
}

func referenceSignalFromTurn(reference agent.TurnReference) AgentReferenceSignal {
	locations := make([]AgentReferenceLocationSignal, 0, len(reference.Locations))
	for _, location := range reference.Locations {
		locations = append(locations, AgentReferenceLocationSignal{
			DashboardID: Optional(location.DashboardID), DashboardName: Optional(location.DashboardName),
			PageID: Optional(location.PageID), PageName: Optional(location.PageName), Href: location.Href,
		})
	}
	hierarchy := append([]string(nil), reference.Hierarchy...)
	if len(hierarchy) == 0 {
		appendUnique := func(value string) {
			value = strings.TrimSpace(value)
			if value != "" && (len(hierarchy) == 0 || hierarchy[len(hierarchy)-1] != value) {
				hierarchy = append(hierarchy, value)
			}
		}
		appendUnique(reference.Workspace.Name)
		if len(reference.Locations) > 0 {
			if reference.Reference.Type == "page" || reference.Reference.Type == "visual" {
				appendUnique(reference.Locations[0].DashboardName)
			}
			if reference.Reference.Type == "visual" {
				appendUnique(reference.Locations[0].PageName)
			}
		}
	}
	return AgentReferenceSignal{
		Reference: AgentReferenceKeySignal{WorkspaceID: reference.Reference.WorkspaceID, Type: reference.Reference.Type, ID: reference.Reference.ID},
		Name:      reference.Name, Description: Optional(reference.Description), VisualType: Optional(reference.VisualType),
		Workspace: AgentReferenceWorkspaceSignal{ID: reference.Workspace.ID, Name: reference.Workspace.Name},
		Hierarchy: hierarchy, Href: reference.Href, Locations: locations, Context: append([]string(nil), reference.Context...),
	}
}

func chatInitialSignals(catalog navigation.Catalog, workspaceID, roleLabel, view string, state ChatViewState) map[string]any {
	sidebar := sidebarConfigForChat(catalog, workspaceID, roleLabel, view)
	attachChatSidebar(&sidebar, state.Agent)
	return map[string]any{
		"chrome": ChromeSignal{Sidebar: sidebar},
		"page": ChatPageSignal{
			Kind: RouteChat, View: normalizedView(view), Title: "Chats",
			Description: "Ask read-only questions about dashboards, semantic models, measures, and fields.",
		},
		"runtime": RouteRuntimeSignal{Kind: RouteChat, WorkspaceID: Optional(workspaceID)},
		"agent":   state.Agent,
		"agentContext": AgentContextSignal{
			Surface: "chat", WorkspaceID: workspaceID,
			Filters:        DashboardFilters{Controls: map[string]any{}, Selections: []any{}, SpatialSelections: []any{}},
			ReferenceLimit: agent.MaxTurnReferences, References: []AgentReferenceSignal{},
		},
		"agentReferenceSearch": AgentReferenceSearchSignal{Results: []AgentReferenceSignal{}},
		"visuals":              state.Visuals,
	}
}

func sidebarConfigForChat(catalog navigation.Catalog, workspaceID, roleLabel, view string) SidebarSignal {
	if strings.TrimSpace(workspaceID) != "" {
		catalog.Workspace.ID = workspaceID
	}
	workspaceTitle := strings.TrimSpace(catalog.Workspace.Title)
	if workspaceTitle == "" {
		workspaceTitle = strings.TrimSpace(catalog.Workspace.ID)
	}
	if workspaceTitle == "" {
		workspaceTitle = "LeapView"
	}
	active := ""
	if strings.TrimSpace(view) == "list" {
		active = "chat"
	}
	return SidebarSignal{
		WorkspaceTitle: workspaceTitle, Active: active, DashboardTitle: "Workspace", PageTitle: "Published assets",
		UserRole: Optional(roleLabel), Groups: []SidebarGroupSignal{{
			Label: "Navigation",
			Items: []SidebarItemSignal{
				{ID: "dashboards", Label: "Dashboards", Href: "/", Icon: "dashboard", Meta: Optional("Reports")},
				{ID: "chat", Label: "Chats", Href: "/chats", Icon: "chat", Meta: Optional("Agent interface")},
				{ID: "workspaces", Label: "Workspaces", Href: "/workspaces", Icon: "catalog", Meta: Optional("Published assets")},
				{ID: "data", Label: "Data", Href: "/data", Icon: "cache", Meta: Optional("Inspect rows")},
				{ID: "connections", Label: "Connections", Href: "/connections", Icon: "data", Meta: Optional("Data access")},
				{ID: "admin", Label: "Admin", Href: "/admin", Icon: "settings", Meta: Optional("Read-only administration")},
			},
		}},
	}
}

func attachChatSidebar(sidebar *SidebarSignal, state ChatSignal) {
	sidebar.PrimaryAction = &SidebarActionSignal{Label: "New chat", Href: "/chats/new", Icon: "plus"}
	sidebar.History = &SidebarHistorySignal{Label: "Chats", EmptyText: Optional("No chats yet."), Items: chatHistoryItems(state)}
}

func chatHistoryItems(state ChatSignal) []SidebarHistoryItemSignal {
	items := make([]SidebarHistoryItemSignal, 0, len(state.Conversations))
	for _, conversation := range state.Conversations {
		title := conversation.Title
		if title == "" {
			title = "Conversation"
		}
		items = append(items, SidebarHistoryItemSignal{
			ID: conversation.ID, Title: title, Href: chatPath(conversation.ID),
			Active: conversation.ID == state.ActiveConversationID, Pending: conversation.TitlePending,
		})
	}
	return items
}

func chatPath(parts ...string) string {
	path := "/chats"
	for _, part := range parts {
		if part = strings.Trim(part, "/"); part != "" {
			path += "/" + url.PathEscape(part)
		}
	}
	return path
}

func normalizedView(view string) string {
	if strings.TrimSpace(view) == "" {
		return "conversation"
	}
	return view
}
