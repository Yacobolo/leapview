package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Yacobolo/libredash/internal/dashboard"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
	workspaceview "github.com/Yacobolo/libredash/internal/workspace"
	g "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

type AdminData struct {
	Workspace         workspaceview.WorkspaceView
	AuthConfigured    bool
	RBACConfigured    bool
	RBACStatusLabel   string
	PrincipalCount    int
	GroupCount        int
	BindingCount      int
	RoleCount         int
	Principals        []AdminPrincipal
	SelectedPrincipal *AdminPrincipal
	Groups            []AdminGroup
	SelectedGroup     *AdminGroup
}

type AdminPrincipal struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   string
	UpdatedAt   string
	DirectRoles []string
	Groups      []AdminGroupRef
}

type AdminGroupRef struct {
	ID         string
	Name       string
	ExternalID string
}

type AdminGroup struct {
	ID         string
	Name       string
	Provider   string
	ExternalID string
	CreatedAt  string
	Roles      []string
	Members    []AdminPrincipalRef
}

type AdminPrincipalRef struct {
	ID          string
	Email       string
	DisplayName string
}

func AdminPage(catalog dashboard.Catalog, active, roleLabel string, data AdminData) g.Node {
	title := adminPageTitle(active)
	page := adminPageSignal(active, data)
	chrome := uisignals.ChromeSignal{Sidebar: uisignals.SidebarConfigForWorkspace(catalog, "admin", roleLabel)}
	signals := map[string]any{
		"chrome":  chrome,
		"page":    page,
		"runtime": uisignals.RouteRuntimeSignal{Kind: uisignals.RouteAdmin},
		"status":  dashboard.Status{},
	}
	return c.HTML5(c.HTML5Props{
		Title:    "Admin - " + title,
		Language: "en",
		HTMLAttrs: []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		},
		Head: pageHead(
			h.Script(h.Type("module"), h.Src(staticAsset("/static/app-shell.js"))),
			h.Script(h.Type("module"), h.Src(staticAsset("/static/admin-page.js"))),
			inspectorScript(),
			h.Script(h.Type("module"), h.Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.2/bundles/datastar.js")),
		),
		Body: []g.Node{
			h.Main(h.Class(appRootClass),
				ds.Signals(signals),
				g.El("ld-app-shell",
					g.Attr("chrome", jsonString(chrome)),
					g.Attr("data-attr:chrome", "JSON.stringify($chrome)"),
					g.El("ld-admin-page",
						g.Attr("slot", "page"),
						g.Attr("page", jsonString(page)),
						g.Attr("data-attr:page", "JSON.stringify($page)"),
					),
				),
				inspectorElement(),
			),
		},
	})
}

func adminPageSignal(active string, data AdminData) uisignals.AdminPageSignal {
	page := uisignals.AdminPageSignal{
		Kind:    uisignals.RouteAdmin,
		Title:   adminPageTitle(active),
		Active:  active,
		Sidebar: adminSidebarSignal(active),
	}
	switch active {
	case "principals":
		page.HeaderTitle = "Principals"
		page.HeaderDetail = "Users and service principals known to LibreDash."
		page.Sections = []uisignals.AdminContentSectionSignal{{Title: "Principals", Grid: adminPrincipalsGrid(data.Principals)}}
	case "principal-detail":
		page.HeaderTitle = "Principals"
		page.HeaderDetail = "Read-only principal access."
		if data.SelectedPrincipal == nil {
			page.Empty = "Principal not found."
			return page
		}
		principal := *data.SelectedPrincipal
		name := adminDisplayLabel(principal.DisplayName, principal.Email, principal.ID)
		page.HeaderTitle = "Principals / " + name
		page.HeaderDetail = "Read-only principal identity and group memberships."
		page.Metrics = []uisignals.AdminMetricSignal{
			{Label: "Email", Value: principal.Email},
			{Label: "Principal ID", Value: principal.ID},
			{Label: "Direct roles", Value: strings.Join(principal.DirectRoles, ", ")},
			{Label: "Group count", Value: fmt.Sprint(len(principal.Groups))},
			{Label: "Created", Value: principal.CreatedAt},
			{Label: "Updated", Value: principal.UpdatedAt},
		}
		page.Sections = []uisignals.AdminContentSectionSignal{{Title: "Groups", Grid: adminPrincipalGroupsGrid(principal, data.Groups)}}
	case "groups":
		page.HeaderTitle = "Groups"
		page.HeaderDetail = "Workspace groups and their read-only membership summaries."
		page.Sections = []uisignals.AdminContentSectionSignal{{Title: "Groups", Grid: adminGroupsGrid(data.Groups)}}
	case "group-detail":
		page.HeaderTitle = "Groups"
		page.HeaderDetail = "Read-only group membership."
		if data.SelectedGroup == nil {
			page.Empty = "Group not found."
			return page
		}
		group := *data.SelectedGroup
		name := adminDisplayLabel(group.Name, group.ExternalID, group.ID)
		page.HeaderTitle = "Groups / " + name
		page.HeaderDetail = "Read-only group membership and role assignments."
		page.Metrics = []uisignals.AdminMetricSignal{
			{Label: "Provider", Value: group.Provider},
			{Label: "External ID", Value: group.ExternalID},
			{Label: "Group ID", Value: group.ID},
			{Label: "Roles", Value: strings.Join(group.Roles, ", ")},
			{Label: "Member count", Value: fmt.Sprint(len(group.Members))},
		}
		page.Sections = []uisignals.AdminContentSectionSignal{{Title: "Members", Grid: adminGroupMembersGrid(group, data.Principals)}}
	default:
		page.HeaderTitle = "General"
		page.HeaderDetail = "Read-only workspace administration."
		if !data.RBACConfigured {
			page.Empty = data.RBACStatusLabel
		}
		page.Metrics = []uisignals.AdminMetricSignal{
			{Label: "Workspace", Value: data.Workspace.Title, Detail: data.Workspace.ID},
			{Label: "Auth", Value: configuredLabel(data.AuthConfigured)},
			{Label: "RBAC", Value: data.RBACStatusLabel},
			{Label: "Principals", Value: fmt.Sprint(data.PrincipalCount)},
			{Label: "Groups", Value: fmt.Sprint(data.GroupCount)},
			{Label: "Role bindings", Value: fmt.Sprint(data.BindingCount)},
			{Label: "Roles", Value: fmt.Sprint(data.RoleCount)},
		}
	}
	return page
}

func adminSidebarSignal(active string) uisignals.SubSidebarSignal {
	principalsActive := active == "principals" || active == "principal-detail"
	groupsActive := active == "groups" || active == "group-detail"
	return uisignals.SubSidebarSignal{
		Label:       "Admin",
		RailLabel:   "Admin",
		AriaLabel:   "Admin navigation",
		StorageKey:  "libredash-admin-sidebar-collapsed",
		ActiveID:    active,
		Numbered:    false,
		Collapsible: false,
		Items: []uisignals.SubSidebarItemSignal{
			{ID: "general", Title: "General", Href: "/admin", Active: active == "general"},
			{ID: "principals", Title: "Principals", Href: "/admin/principals", Active: principalsActive},
			{ID: "groups", Title: "Groups", Href: "/admin/groups", Active: groupsActive},
		},
	}
}

func adminPrincipalsGrid(principals []AdminPrincipal) metricGrid {
	rows := make([]map[string]any, 0, len(principals))
	for _, principal := range principals {
		rows = append(rows, map[string]any{
			"name":        adminDisplayLabel(principal.DisplayName, principal.Email, principal.ID),
			"name_href":   adminPrincipalHref(principal.ID),
			"email":       principal.Email,
			"id":          principal.ID,
			"roles":       principal.DirectRoles,
			"group_count": len(principal.Groups),
			"updated_at":  principal.UpdatedAt,
		})
	}
	return metricGrid{
		Columns: []metricGridColumn{
			{ID: "name", Header: "Name", Kind: "link", HrefKey: "name_href", Width: "150px"},
			{ID: "email", Header: "Email", Width: "190px"},
			{ID: "roles", Header: "Direct roles", Kind: "tags", Width: "135px"},
			{ID: "group_count", Header: "Group count", Kind: "number", Align: "right", Width: "120px"},
			{ID: "id", Header: "Principal ID", Kind: "code", Width: "190px"},
			{ID: "updated_at", Header: "Updated", Width: "150px"},
		},
		Rows:     rows,
		Empty:    "No principals found.",
		MinWidth: "935px",
	}
}

func adminPrincipalGroupsGrid(principal AdminPrincipal, groups []AdminGroup) metricGrid {
	groupsByID := make(map[string]AdminGroup, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
	}
	rows := make([]map[string]any, 0, len(principal.Groups))
	for _, ref := range principal.Groups {
		group := groupsByID[ref.ID]
		rows = append(rows, map[string]any{
			"name":         adminDisplayLabel(group.Name, ref.Name, group.ExternalID, ref.ExternalID, ref.ID),
			"name_href":    adminGroupHref(ref.ID),
			"provider":     group.Provider,
			"external_id":  adminDisplayLabel(group.ExternalID, ref.ExternalID),
			"roles":        group.Roles,
			"member_count": len(group.Members),
		})
	}
	return metricGrid{
		Columns: []metricGridColumn{
			{ID: "name", Header: "Name", Kind: "link", HrefKey: "name_href", Width: "180px"},
			{ID: "provider", Header: "Provider", Width: "120px"},
			{ID: "external_id", Header: "External ID", Kind: "code", Width: "180px"},
			{ID: "roles", Header: "Roles", Kind: "tags", Width: "160px"},
			{ID: "member_count", Header: "Member count", Kind: "number", Align: "right", Width: "130px"},
		},
		Rows:     rows,
		Empty:    "No groups found.",
		MinWidth: "800px",
	}
}

func adminGroupsGrid(groups []AdminGroup) metricGrid {
	rows := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, map[string]any{
			"name":         adminDisplayLabel(group.Name, group.ExternalID, group.ID),
			"name_href":    adminGroupHref(group.ID),
			"provider":     group.Provider,
			"external_id":  group.ExternalID,
			"id":           group.ID,
			"roles":        group.Roles,
			"member_count": len(group.Members),
		})
	}
	return metricGrid{
		Columns: []metricGridColumn{
			{ID: "name", Header: "Name", Kind: "link", HrefKey: "name_href", Width: "180px"},
			{ID: "provider", Header: "Provider", Width: "120px"},
			{ID: "external_id", Header: "External ID", Kind: "code", Width: "180px"},
			{ID: "roles", Header: "Roles", Kind: "tags", Width: "180px"},
			{ID: "member_count", Header: "Member count", Kind: "number", Align: "right", Width: "130px"},
			{ID: "id", Header: "Group ID", Kind: "code", Width: "220px"},
		},
		Rows:     rows,
		Empty:    "No groups found.",
		MinWidth: "1010px",
	}
}

func adminGroupMembersGrid(group AdminGroup, principals []AdminPrincipal) metricGrid {
	principalsByID := make(map[string]AdminPrincipal, len(principals))
	for _, principal := range principals {
		principalsByID[principal.ID] = principal
	}
	rows := make([]map[string]any, 0, len(group.Members))
	for _, member := range group.Members {
		principal := principalsByID[member.ID]
		rows = append(rows, map[string]any{
			"name":         adminDisplayLabel(member.DisplayName, principal.DisplayName, member.Email, principal.Email, member.ID),
			"email":        adminDisplayLabel(member.Email, principal.Email),
			"id":           member.ID,
			"direct_roles": principal.DirectRoles,
			"updated_at":   principal.UpdatedAt,
		})
	}
	return metricGrid{
		Columns: []metricGridColumn{
			{ID: "name", Header: "Name", Width: "150px"},
			{ID: "email", Header: "Email", Width: "190px"},
			{ID: "id", Header: "Principal ID", Kind: "code", Width: "180px"},
			{ID: "direct_roles", Header: "Direct roles", Kind: "tags", Width: "130px"},
			{ID: "updated_at", Header: "Updated", Width: "150px"},
		},
		Rows:     rows,
		Empty:    "No members found.",
		MinWidth: "840px",
	}
}

func adminGroupHref(groupID string) string {
	return "/admin/groups/" + url.PathEscape(groupID)
}

func adminPrincipalHref(principalID string) string {
	return "/admin/principals/" + url.PathEscape(principalID)
}

func adminPageTitle(active string) string {
	switch active {
	case "principals":
		return "Principals"
	case "principal-detail":
		return "Principal"
	case "groups":
		return "Groups"
	case "group-detail":
		return "Group"
	default:
		return "General"
	}
}

func configuredLabel(configured bool) string {
	if configured {
		return "Configured"
	}
	return "Not configured"
}

func adminDisplayLabel(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "-"
}
