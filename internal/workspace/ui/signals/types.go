package signals

import (
	"fmt"
	"net/url"
	"strings"

	adminview "github.com/Yacobolo/leapview/internal/admin/view"
	"github.com/Yacobolo/leapview/internal/agent"
	visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"
	workspaceview "github.com/Yacobolo/leapview/internal/workspace"
	"github.com/Yacobolo/leapview/internal/workspace/navigation"
)

const (
	RouteDashboard       RouteKind = "dashboard"
	RouteCatalog         RouteKind = "catalog"
	RouteChat            RouteKind = "chat"
	RouteWorkspace       RouteKind = "workspace"
	RouteWorkspaceAsset  RouteKind = "workspace_asset"
	RouteConnections     RouteKind = "connections"
	RouteConnectionAsset RouteKind = "connection_asset"
	RouteData            RouteKind = "data"
	RouteAdmin           RouteKind = "admin"
	RouteLogin           RouteKind = "login"
)

type AdminStorageData = adminview.AdminStorageData
type AdminStorageDatabase = adminview.AdminStorageDatabase
type AdminStorageTable = adminview.AdminStorageTable
type AdminStorageColumn = adminview.AdminStorageColumn
type AdminStorageFile = adminview.AdminStorageFile
type AdminStorageTableHistory = adminview.AdminStorageTableHistory
type AdminStorageSnapshot = adminview.AdminStorageSnapshot
type AdminStorageServingState = adminview.AdminStorageServingState

type WorkspaceAccessResponse struct {
	Workspace    workspaceview.WorkspaceView     `json:"workspace"`
	ObjectType   string                          `json:"objectType,omitempty"`
	ObjectID     string                          `json:"objectId,omitempty"`
	ObjectTitle  string                          `json:"objectTitle,omitempty"`
	Mode         string                          `json:"mode,omitempty"`
	Roles        []workspaceview.RoleView        `json:"roles"`
	Bindings     []workspaceview.RoleBindingView `json:"bindings"`
	Candidates   []WorkspaceAccessCandidate      `json:"candidates"`
	CanManage    bool                            `json:"canManage"`
	Search       string                          `json:"search"`
	SearchStatus WorkspaceAccessSearchStatus     `json:"searchStatus"`
	Status       WorkspaceAccessStatus           `json:"status"`
}

type ChatViewState struct {
	Agent   ChatSignal
	Visuals map[string]visualizationir.VisualizationEnvelope
}

func ChatTranscriptItems(items []agent.ChatTranscriptItem) []ChatTranscriptItemSignal {
	out := make([]ChatTranscriptItemSignal, 0, len(items))
	for _, item := range items {
		out = append(out, ChatTranscriptItem(item))
	}
	return out
}

func ChatTranscriptItem(item agent.ChatTranscriptItem) ChatTranscriptItemSignal {
	out := ChatTranscriptItemSignal{
		ID:             item.ID,
		Kind:           item.Kind,
		Text:           optionalValue(item.Text),
		Markdown:       optionalValue(item.Markdown),
		ToolCallID:     optionalValue(item.ToolCallID),
		Name:           optionalValue(item.Name),
		Title:          optionalValue(item.Title),
		Status:         optionalValue(item.Status),
		Summary:        optionalValue(item.Summary),
		ResultSummary:  optionalValue(item.ResultSummary),
		InputJSON:      optionalValue(item.InputJSON),
		InputFormat:    optionalValue(item.InputFormat),
		ArgumentsJSON:  optionalValue(item.ArgumentsJSON),
		ResultJSON:     optionalValue(item.ResultJSON),
		ResultFormat:   optionalValue(item.ResultFormat),
		Error:          optionalValue(item.Error),
		ConversationID: optionalValue(item.ConversationID),
		RunID:          optionalValue(item.RunID),
		CreatedAt:      optionalValue(item.CreatedAt),
	}
	if len(item.References) > 0 {
		references := make([]AgentReferenceSignal, 0, len(item.References))
		for _, reference := range item.References {
			references = append(references, agentReferenceSignalFromTurn(reference))
		}
		out.References = &references
	}
	if item.Artifact != nil {
		out.Artifact = &ChatArtifactSignal{
			Type:    item.Artifact.Type,
			ID:      item.Artifact.ID,
			Summary: optionalValue(item.Artifact.Summary),
		}
	}
	return out
}

func agentReferenceSignalFromTurn(reference agent.TurnReference) AgentReferenceSignal {
	locations := make([]AgentReferenceLocationSignal, 0, len(reference.Locations))
	for _, location := range reference.Locations {
		locations = append(locations, AgentReferenceLocationSignal{
			DashboardID:   optionalValue(location.DashboardID),
			DashboardName: optionalValue(location.DashboardName),
			PageID:        optionalValue(location.PageID),
			PageName:      optionalValue(location.PageName),
			Href:          location.Href,
		})
	}
	hierarchy := append([]string(nil), reference.Hierarchy...)
	if len(hierarchy) == 0 {
		hierarchy = referenceHierarchyFromTurn(reference)
	}
	return AgentReferenceSignal{
		Reference: AgentReferenceKeySignal{
			WorkspaceID: reference.Reference.WorkspaceID,
			Type:        reference.Reference.Type,
			ID:          reference.Reference.ID,
		},
		Name:        reference.Name,
		Description: optionalValue(reference.Description),
		VisualType:  optionalValue(reference.VisualType),
		Workspace:   AgentReferenceWorkspaceSignal{ID: reference.Workspace.ID, Name: reference.Workspace.Name},
		Hierarchy:   hierarchy,
		Href:        reference.Href,
		Locations:   locations,
		Context:     append([]string(nil), reference.Context...),
	}
}

func referenceHierarchyFromTurn(reference agent.TurnReference) []string {
	hierarchy := make([]string, 0, 3)
	appendUnique := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && (len(hierarchy) == 0 || hierarchy[len(hierarchy)-1] != value) {
			hierarchy = append(hierarchy, value)
		}
	}
	appendUnique(reference.Workspace.Name)
	if len(reference.Locations) > 0 {
		location := reference.Locations[0]
		if reference.Reference.Type == "page" || reference.Reference.Type == "visual" {
			appendUnique(location.DashboardName)
		}
		if reference.Reference.Type == "visual" {
			appendUnique(location.PageName)
		}
	}
	return hierarchy
}

func ChatInitialEnvelope(catalog navigation.Catalog, workspaceID, roleLabel, view string, state ChatViewState) ChatEnvelope {
	chrome := ChromeSignal{Sidebar: SidebarConfigForChat(catalog, workspaceID, roleLabel, view)}
	AttachChatSidebar(&chrome.Sidebar, state.Agent)
	return ChatEnvelope{
		Chrome:  chrome,
		Page:    ChatPage(workspaceID, view, state.Agent),
		Runtime: RouteRuntimeSignal{Kind: RouteChat, WorkspaceID: optionalValue(workspaceID)},
		Agent:   state.Agent,
		AgentContext: AgentContextSignal{
			Surface:        "chat",
			WorkspaceID:    workspaceID,
			Filters:        DashboardFilters{Controls: map[string]DashboardFilterControl{}, Selections: []DashboardInteractionSelection{}},
			ReferenceLimit: agent.MaxTurnReferences,
			References:     []AgentReferenceSignal{},
		},
		AgentReferenceSearch: AgentReferenceSearchSignal{Results: []AgentReferenceSignal{}},
		Visuals:              state.Visuals,
	}
}

func ChatPage(workspaceID, view string, agent ChatSignal) ChatPageSignal {
	if strings.TrimSpace(view) == "" {
		view = "conversation"
	}
	return ChatPageSignal{
		Kind:        RouteChat,
		View:        view,
		Title:       "Chats",
		Description: "Ask read-only questions about dashboards, semantic models, measures, and fields.",
	}
}

func AttachChatSidebar(sidebar *SidebarSignal, agent ChatSignal) {
	if sidebar == nil {
		return
	}
	sidebar.PrimaryAction = &SidebarActionSignal{Label: "New chat", Href: chatPath("new"), Icon: "plus"}
	sidebar.History = &SidebarHistorySignal{
		Label:     "Chats",
		EmptyText: optionalValue("No chats yet."),
		Items:     ChatHistoryItems(agent),
	}
}

func ChatHistoryItems(agent ChatSignal) []SidebarHistoryItemSignal {
	items := make([]SidebarHistoryItemSignal, 0, len(agent.Conversations))
	for _, conversation := range agent.Conversations {
		title := conversation.Title
		if title == "" {
			title = "Conversation"
		}
		items = append(items, SidebarHistoryItemSignal{
			ID:      conversation.ID,
			Title:   title,
			Href:    chatPath(conversation.ID),
			Active:  conversation.ID == agent.ActiveConversationID,
			Pending: conversation.TitlePending,
		})
	}
	return items
}

func WorkspaceAccessSignals(access WorkspaceAccessResponse) WorkspaceAccessSignal {
	roles := make([]any, len(access.Roles))
	for index := range access.Roles {
		roles[index] = access.Roles[index]
	}
	bindings := make([]any, len(access.Bindings))
	for index := range access.Bindings {
		bindings[index] = access.Bindings[index]
	}
	candidates := access.Candidates
	if candidates == nil {
		candidates = []WorkspaceAccessCandidate{}
	}
	return WorkspaceAccessSignal{
		Workspace:    access.Workspace,
		ObjectType:   optionalValue(access.ObjectType),
		ObjectID:     optionalValue(access.ObjectID),
		ObjectTitle:  optionalValue(access.ObjectTitle),
		Mode:         optionalValue(access.Mode),
		Roles:        roles,
		Bindings:     bindings,
		Candidates:   candidates,
		CanManage:    access.CanManage,
		Status:       access.Status,
		Command:      WorkspaceAccessCommand{},
		Search:       access.Search,
		SearchStatus: access.SearchStatus,
	}
}

func SidebarConfigForCatalog(catalog navigation.Catalog) SidebarSignal {
	modelID, modelTitle := "", ""
	if len(catalog.Models) > 0 {
		modelID = catalog.Models[0].ID
		modelTitle = catalog.Models[0].Title
	}
	return sidebarConfig(catalog, "dashboards", "", "LeapView", "Dashboards", "Discovery", modelID, modelTitle, false, "", false)
}

func SidebarConfigForWorkspace(catalog navigation.Catalog, active, roleLabel string) SidebarSignal {
	return sidebarConfig(catalog, active, "", workspaceDisplayTitle(catalog), "Workspace", "Published assets", "", "", false, roleLabel, strings.TrimSpace(catalog.Workspace.ID) != "")
}

func SidebarConfigForChat(catalog navigation.Catalog, workspaceID, roleLabel, view string) SidebarSignal {
	if strings.TrimSpace(workspaceID) != "" {
		catalog.Workspace.ID = workspaceID
	}
	active := ""
	if strings.TrimSpace(view) == "list" {
		active = "chat"
	}
	config := SidebarConfigForWorkspace(catalog, active, roleLabel)
	return config
}

func sidebarConfig(catalog navigation.Catalog, active, dashboardID, workspaceTitle, dashboardTitle, pageTitle, modelID, modelTitle string, compact bool, roleLabel string, includeWorkspaceScoped bool) SidebarSignal {
	return SidebarSignal{
		WorkspaceTitle: workspaceTitle,
		Active:         active,
		DashboardID:    optionalValue(dashboardID),
		DashboardTitle: dashboardTitle,
		PageTitle:      pageTitle,
		ModelID:        optionalValue(modelID),
		ModelTitle:     optionalValue(modelTitle),
		Compact:        compact,
		UserRole:       optionalValue(roleLabel),
		Groups:         sidebarGroups(catalog, includeWorkspaceScoped),
	}
}

func ValidateChatEnvelope(envelope ChatEnvelope) error {
	if envelope.Page.Kind != RouteChat {
		return fmt.Errorf("chat envelope page kind = %q", envelope.Page.Kind)
	}
	if envelope.Runtime.Kind != RouteChat {
		return fmt.Errorf("chat envelope runtime kind = %q", envelope.Runtime.Kind)
	}
	if envelope.Page.Title == "" {
		return fmt.Errorf("chat envelope requires page title")
	}
	return nil
}

func sidebarGroups(catalog navigation.Catalog, includeWorkspaceScoped bool) []SidebarGroupSignal {
	return []SidebarGroupSignal{
		{
			Label: "Navigation",
			Items: []SidebarItemSignal{
				{ID: "dashboards", Label: "Dashboards", Href: "/", Icon: "dashboard", Meta: optionalValue("Reports")},
				{ID: "chat", Label: "Chats", Href: chatPath(), Icon: "chat", Meta: optionalValue("Agent interface")},
				{ID: "workspaces", Label: "Workspaces", Href: "/workspaces", Icon: "catalog", Meta: optionalValue("Published assets")},
				{ID: "data", Label: "Data", Href: "/data", Icon: "cache", Meta: optionalValue("Inspect rows")},
				{ID: "connections", Label: "Connections", Href: "/connections", Icon: "data", Meta: optionalValue("Data access")},
				{ID: "admin", Label: "Admin", Href: "/admin", Icon: "settings", Meta: optionalValue("Read-only administration")},
			},
		},
	}
}

func chatPath(parts ...string) string {
	path := "/chats"
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		path += "/" + url.PathEscape(part)
	}
	return path
}

func workspaceDisplayTitle(catalog navigation.Catalog) string {
	if strings.TrimSpace(catalog.Workspace.Title) != "" {
		return catalog.Workspace.Title
	}
	if strings.TrimSpace(catalog.Workspace.ID) != "" {
		return catalog.Workspace.ID
	}
	return "LeapView"
}
