package app

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
	"github.com/Yacobolo/libredash/internal/queryaudit"
	"github.com/Yacobolo/libredash/internal/ui"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
	"github.com/Yacobolo/libredash/internal/workspace"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
)

const adminQueryHistoryDefaultLimit = 50

type adminQueryHistoryCommandSignals struct {
	AdminQueryHistory        uisignals.AdminQueryHistorySignal  `json:"adminQueryHistory"`
	AdminQueryDetail         uisignals.AdminQueryDetailSignal   `json:"adminQueryDetail"`
	AdminQueryHistoryCommand uisignals.AdminQueryHistoryCommand `json:"adminQueryHistoryCommand"`
}

func (s *Server) adminGeneral(w http.ResponseWriter, r *http.Request) {
	s.renderAdminPage(w, r, "general")
}

func (s *Server) adminPrincipals(w http.ResponseWriter, r *http.Request) {
	s.renderAdminPage(w, r, "principals")
}

func (s *Server) adminPrincipalDetail(w http.ResponseWriter, r *http.Request) {
	data, err := s.adminData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	principalID := chi.URLParam(r, "principal")
	for i := range data.Principals {
		if data.Principals[i].ID == principalID {
			data.SelectedPrincipal = &data.Principals[i]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			if err := ui.AdminPage(s.metrics.Catalog(), "principal-detail", s.currentAdminRoleLabel(r), data, s.chatChromeOption(r)).Render(w); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) adminGroups(w http.ResponseWriter, r *http.Request) {
	s.renderAdminPage(w, r, "groups")
}

func (s *Server) adminAgent(w http.ResponseWriter, r *http.Request) {
	s.renderAdminPage(w, r, "agent")
}

func (s *Server) adminStorage(w http.ResponseWriter, r *http.Request) {
	_ = lddatastar.EnsureClientID(w, r)
	s.renderAdminPage(w, r, "storage")
}

func (s *Server) adminQueries(w http.ResponseWriter, r *http.Request) {
	_ = lddatastar.EnsureClientID(w, r)
	s.renderAdminPage(w, r, "queries")
}

func (s *Server) adminGroupDetail(w http.ResponseWriter, r *http.Request) {
	data, err := s.adminData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	groupID := chi.URLParam(r, "group")
	for i := range data.Groups {
		if data.Groups[i].ID == groupID {
			data.SelectedGroup = &data.Groups[i]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			if err := ui.AdminPage(s.metrics.Catalog(), "group-detail", s.currentAdminRoleLabel(r), data, s.chatChromeOption(r)).Render(w); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) renderAdminPage(w http.ResponseWriter, r *http.Request, active string) {
	data, err := s.adminData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := ui.AdminPage(s.metrics.Catalog(), active, s.currentAdminRoleLabel(r), data, s.chatChromeOption(r)).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) currentAdminRoleLabel(r *http.Request) string {
	if s.auth == nil {
		return "Local platform"
	}
	principal, ok := s.auth.Principal(r)
	if ok && principal.DevBypass {
		return "Platform admin"
	}
	return "Platform access"
}

func (s *Server) adminData(r *http.Request) (ui.AdminData, error) {
	data := ui.AdminData{
		Workspace:       workspace.WorkspaceView{ID: "platform", Title: "Platform"},
		CSRFToken:       csrfToken(r, s.auth),
		AuthConfigured:  s.auth != nil,
		RBACConfigured:  s.store != nil,
		RBACStatusLabel: "Configured",
	}
	var err error
	data.Agent, err = s.adminAgentData(r)
	if err != nil {
		return data, err
	}
	repo, err := s.accessRepository()
	if err != nil {
		return data, err
	}
	if repo == nil {
		data.RBACConfigured = false
		data.RBACStatusLabel = "RBAC store is not configured"
		data.RoleCount = len(defaultWorkspaceRoles())
		data.Storage = s.adminStorageData(r)
		return data, nil
	}
	principals, err := s.adminPrincipalsData(r)
	if err != nil {
		return data, err
	}
	groups, err := s.adminGroupsData(r)
	if err != nil {
		return data, err
	}
	bindings, roles, err := s.adminRoleBindingsAndRoles(r)
	if err != nil {
		return data, err
	}
	membersByGroup := map[string][]ui.AdminPrincipalRef{}
	groupsByID := map[string]access.Group{}
	for _, group := range groups {
		groupsByID[group.ID] = group
		members := s.adminGroupMembersData(r, group.ID)
		for _, member := range members {
			membersByGroup[group.ID] = append(membersByGroup[group.ID], ui.AdminPrincipalRef{
				ID:          member.ID,
				Email:       member.Email,
				DisplayName: member.DisplayName,
			})
		}
	}
	data.RoleCount = len(roles)
	data.BindingCount = len(bindings)
	data.Principals = buildAdminPrincipals(principals, bindings, groupsByID, membersByGroup)
	data.Groups = buildAdminGroups(groups, bindings, membersByGroup)
	data.Storage = s.adminStorageData(r)
	data.QueryHistory = s.adminQueryHistoryData(r, uisignals.AdminQueryHistoryFilters{}, "", adminQueryHistoryDefaultLimit)
	data.PrincipalCount = len(data.Principals)
	data.GroupCount = len(data.Groups)
	return data, nil
}

func (s *Server) adminQueryEventsData(r *http.Request) []ui.AdminQueryEvent {
	return s.adminQueryHistoryData(r, uisignals.AdminQueryHistoryFilters{}, "", 100).Events
}

func (s *Server) adminQueryHistoryData(r *http.Request, filters uisignals.AdminQueryHistoryFilters, pageToken string, limit int) ui.AdminQueryHistoryData {
	repo, err := s.queryAuditRepository()
	if err != nil || repo == nil {
		return ui.AdminQueryHistoryData{Filters: filters, Limit: normalizeAdminQueryHistoryLimit(limit), Error: queryHistoryErrorText(err)}
	}
	events, nextCursor, hasMore, err := s.listAdminQueryHistoryPage(r, repo, filters, pageToken, limit)
	if err != nil {
		return ui.AdminQueryHistoryData{Filters: filters, Limit: normalizeAdminQueryHistoryLimit(limit), Error: err.Error()}
	}
	return ui.AdminQueryHistoryData{
		Events:     events,
		Filters:    filters,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Limit:      normalizeAdminQueryHistoryLimit(limit),
	}
}

func (s *Server) adminQueryHistoryUpdates(w http.ResponseWriter, r *http.Request) {
	clientID := lddatastar.EnsureClientID(w, r)
	sse := datastar.NewSSE(w, r)
	updates, unsubscribe := s.broker.Subscribe(adminQueryHistoryStreamID(clientID))
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case patch := <-updates:
			if err := sse.MarshalAndPatchSignals(patch); err != nil {
				return
			}
		}
	}
}

func (s *Server) adminQueryHistoryCommand(w http.ResponseWriter, r *http.Request) {
	clientID := lddatastar.EnsureClientID(w, r)
	var signals adminQueryHistoryCommandSignals
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	command := normalizeAdminQueryHistoryCommand(signals.AdminQueryHistoryCommand)
	repo, err := s.queryAuditRepository()
	if err != nil || repo == nil {
		errorText := queryHistoryErrorText(err)
		if errorText == "" {
			errorText = "Query audit repository is not configured."
		}
		if command.Action == "select_detail" {
			detail := signals.AdminQueryDetail
			detail.EventID = command.EventID
			detail.Loading = false
			detail.Error = errorText
			s.publishAdminQueryDetailPatch(clientID, detail)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		history := signals.AdminQueryHistory
		history.Loading = false
		history.Error = errorText
		s.publishAdminQueryHistoryPatch(clientID, history)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch command.Action {
	case "select_detail":
		event, err := repo.GetQueryEvent(r.Context(), command.EventID)
		if err != nil {
			detail := signals.AdminQueryDetail
			detail.EventID = command.EventID
			detail.Loading = false
			detail.Error = err.Error()
			s.publishAdminQueryDetailPatch(clientID, detail)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.publishAdminQueryDetailPatch(clientID, ui.AdminQueryDetailSignalFromEvent(adminQueryEventFromAudit(event)))
		w.WriteHeader(http.StatusNoContent)
		return
	case "close_detail":
		s.publishAdminQueryDetailPatch(clientID, uisignals.AdminQueryDetailSignal{})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	events, nextCursor, hasMore, err := s.listAdminQueryHistoryPage(r, repo, command.Filters, command.PageToken, command.Limit)
	history := signals.AdminQueryHistory
	incomingCount := len(history.Table.Rows)
	if command.Action == "load_more" {
		nextTable := ui.AdminQueryHistorySignalFromData(ui.AdminQueryHistoryData{Events: events}).Table
		history.Table.Rows = append(history.Table.Rows, nextTable.Rows...)
	} else {
		history.Table = ui.AdminQueryHistorySignalFromData(ui.AdminQueryHistoryData{Events: events}).Table
		incomingCount = 0
	}
	history.Filters = command.Filters
	history.NextCursor = nextCursor
	history.HasMore = hasMore
	history.LoadedCountLabel = queryHistoryLoadedCountLabel(incomingCount + len(events))
	history.Loading = false
	history.Error = ""
	history.Limit = normalizeAdminQueryHistoryLimit(command.Limit)
	if err != nil {
		history.Loading = false
		history.Error = err.Error()
	}
	s.publishAdminQueryHistoryPatch(clientID, history)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) publishAdminQueryHistoryPatch(clientID string, history uisignals.AdminQueryHistorySignal) {
	s.broker.Publish(adminQueryHistoryStreamID(clientID), map[string]any{
		"adminQueryHistory": history,
		"adminQueryHistoryCommand": uisignals.AdminQueryHistoryCommand{
			Action:    "load_more",
			Filters:   history.Filters,
			PageToken: history.NextCursor,
			Limit:     history.Limit,
		},
	})
}

func (s *Server) publishAdminQueryDetailPatch(clientID string, detail uisignals.AdminQueryDetailSignal) {
	s.broker.Publish(adminQueryHistoryStreamID(clientID), map[string]any{
		"adminQueryDetail": detail,
	})
}

func normalizeAdminQueryHistoryCommand(command uisignals.AdminQueryHistoryCommand) uisignals.AdminQueryHistoryCommand {
	action := strings.TrimSpace(command.Action)
	switch action {
	case "load_more", "select_detail", "close_detail":
	default:
		action = "reset"
		command.PageToken = ""
	}
	command.Action = action
	command.Limit = normalizeAdminQueryHistoryLimit(command.Limit)
	command.PageToken = strings.TrimSpace(command.PageToken)
	command.EventID = strings.TrimSpace(command.EventID)
	command.Filters = normalizeAdminQueryHistoryFilters(command.Filters)
	return command
}

func normalizeAdminQueryHistoryFilters(filters uisignals.AdminQueryHistoryFilters) uisignals.AdminQueryHistoryFilters {
	return uisignals.AdminQueryHistoryFilters{
		Workspace: strings.TrimSpace(filters.Workspace),
		Principal: strings.TrimSpace(filters.Principal),
		Surface:   strings.TrimSpace(filters.Surface),
		Kind:      strings.TrimSpace(filters.Kind),
		Status:    strings.TrimSpace(filters.Status),
		Target:    strings.TrimSpace(filters.Target),
		Search:    strings.TrimSpace(filters.Search),
		From:      strings.TrimSpace(filters.From),
		To:        strings.TrimSpace(filters.To),
	}
}

func queryHistoryLoadedCountLabel(count int) string {
	if count == 1 {
		return "1 query loaded"
	}
	return strconv.Itoa(count) + " queries loaded"
}

func adminQueryHistoryStreamID(clientID string) string {
	if strings.TrimSpace(clientID) == "" {
		clientID = "default"
	}
	return "admin-queries:" + clientID
}

func (s *Server) listAdminQueryHistoryPage(r *http.Request, repo queryaudit.Repository, filters uisignals.AdminQueryHistoryFilters, pageToken string, limit int) ([]ui.AdminQueryEvent, string, bool, error) {
	limit = normalizeAdminQueryHistoryLimit(limit)
	rows, err := repo.ListQueryEvents(r.Context(), queryaudit.Filter{
		WorkspaceID: strings.TrimSpace(filters.Workspace),
		PrincipalID: strings.TrimSpace(filters.Principal),
		Surface:     strings.TrimSpace(filters.Surface),
		QueryKind:   strings.TrimSpace(filters.Kind),
		Target:      strings.TrimSpace(filters.Target),
		Status:      strings.TrimSpace(filters.Status),
		Search:      strings.TrimSpace(filters.Search),
		From:        strings.TrimSpace(filters.From),
		To:          strings.TrimSpace(filters.To),
		PageToken:   strings.TrimSpace(pageToken),
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, "", false, err
	}
	nextCursor := ""
	hasMore := len(rows) > limit
	if hasMore {
		last := rows[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		rows = rows[:limit]
	}
	out := make([]ui.AdminQueryEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminQueryEventFromAudit(row))
	}
	return out, nextCursor, hasMore, nil
}

func adminQueryEventFromAudit(row queryaudit.Event) ui.AdminQueryEvent {
	return ui.AdminQueryEvent{
		ID:            row.ID,
		WorkspaceID:   row.WorkspaceID,
		PrincipalID:   row.PrincipalID,
		Surface:       row.Surface,
		Operation:     row.Operation,
		QueryKind:     row.QueryKind,
		ModelID:       row.ModelID,
		Target:        row.Target,
		ObjectType:    row.ObjectType,
		ObjectID:      row.ObjectID,
		RequestID:     row.RequestID,
		CorrelationID: row.CorrelationID,
		Status:        row.Status,
		DurationMS:    row.DurationMS,
		RowsReturned:  row.RowsReturned,
		Error:         row.Error,
		SQL:           row.SQL,
		PlanText:      row.PlanText,
		QueryJSON:     row.QueryJSON,
		CreatedAt:     row.CreatedAt,
	}
}

func queryHistoryErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func normalizeAdminQueryHistoryLimit(limit int) int {
	if limit <= 0 {
		return adminQueryHistoryDefaultLimit
	}
	if limit > maxAPILimit {
		return maxAPILimit
	}
	return limit
}

func (s *Server) adminAgentData(r *http.Request) (ui.AdminAgentData, error) {
	details, err := s.adminAgentDetails(r.Context())
	if err != nil {
		return ui.AdminAgentData{}, err
	}
	data := ui.AdminAgentData{
		Enabled:      details.Enabled,
		Model:        details.Model,
		SystemPrompt: details.SystemPrompt,
		CSRFToken:    csrfToken(r, s.auth),
		UpdatePath:   "/api/v1/admin/agent/config",
		CanWrite:     true,
	}
	for _, tool := range details.Tools {
		data.Tools = append(data.Tools, ui.AdminAgentTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	if s.auth == nil {
		return data, nil
	}
	principal, ok := s.auth.Principal(r)
	if !ok || principal.DevBypass {
		return data, nil
	}
	repo, err := s.accessRepository()
	if err != nil || repo == nil {
		return data, err
	}
	allowed, err := repo.HasPermission(r.Context(), s.defaultWorkspaceID, principal.ID, access.PermissionRBACWrite)
	if err != nil {
		return data, err
	}
	data.CanWrite = allowed
	return data, nil
}

func (s *Server) adminGroupsData(r *http.Request) ([]access.Group, error) {
	if s.store == nil {
		return nil, nil
	}
	rows, err := s.store.SQLDB().QueryContext(r.Context(), `
SELECT id, workspace_id, provider, external_id, name, created_at
FROM groups
ORDER BY workspace_id, name, id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []access.Group{}
	for rows.Next() {
		var group access.Group
		if err := rows.Scan(&group.ID, &group.WorkspaceID, &group.Provider, &group.ExternalID, &group.Name, &group.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *Server) adminGroupMembersData(r *http.Request, groupID string) []ui.AdminPrincipalRef {
	if s.store == nil {
		return nil
	}
	rows, err := s.store.SQLDB().QueryContext(r.Context(), `
SELECT gm.principal_id, p.email, p.display_name
FROM group_members gm
JOIN principals p ON p.id = gm.principal_id
WHERE gm.group_id = ?
ORDER BY p.email, p.display_name, gm.principal_id
`, groupID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	members := []ui.AdminPrincipalRef{}
	for rows.Next() {
		var member ui.AdminPrincipalRef
		if err := rows.Scan(&member.ID, &member.Email, &member.DisplayName); err != nil {
			return nil
		}
		members = append(members, member)
	}
	return members
}

func (s *Server) adminRoleBindingsAndRoles(r *http.Request) ([]workspace.RoleBindingView, []workspace.RoleView, error) {
	repo, err := s.accessRepository()
	if err != nil {
		return nil, nil, err
	}
	if repo == nil {
		return nil, defaultWorkspaceRoles(), nil
	}
	roleRows, err := repo.ListRoles(r.Context())
	if err != nil {
		return nil, nil, err
	}
	bindings, err := s.adminRoleBindingsData(r)
	if err != nil {
		return nil, nil, err
	}
	roles := make([]workspace.RoleView, 0, len(roleRows))
	for _, row := range roleRows {
		roles = append(roles, roleView(row))
	}
	return bindings, roles, nil
}

func (s *Server) adminRoleBindingsData(r *http.Request) ([]workspace.RoleBindingView, error) {
	if s.store == nil {
		return nil, nil
	}
	rows, err := s.store.SQLDB().QueryContext(r.Context(), `
SELECT
  rb.id,
  rb.workspace_id,
  CASE WHEN NULLIF(rb.principal_id, '') IS NOT NULL THEN 'principal' ELSE 'group' END AS subject_type,
  COALESCE(NULLIF(rb.principal_id, ''), rb.group_id, '') AS subject_id,
  rb.principal_id,
  rb.group_id,
  p.email,
  p.display_name,
  g.name AS group_name,
  r.name AS role_name,
  rb.created_at
FROM role_bindings rb
JOIN roles r ON r.id = rb.role_id
LEFT JOIN principals p ON p.id = NULLIF(rb.principal_id, '')
LEFT JOIN groups g ON g.id = rb.group_id
ORDER BY rb.workspace_id, subject_type, p.email, g.name, r.name
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bindings := []workspace.RoleBindingView{}
	for rows.Next() {
		var binding workspace.RoleBindingView
		var principalID, groupID, email, displayName, groupName sql.NullString
		if err := rows.Scan(&binding.ID, &binding.WorkspaceID, &binding.SubjectType, &binding.SubjectID, &principalID, &groupID, &email, &displayName, &groupName, &binding.Role, &binding.CreatedAt); err != nil {
			return nil, err
		}
		binding.PrincipalID = adminNullString(principalID)
		binding.GroupID = adminNullString(groupID)
		binding.Email = adminNullString(email)
		binding.DisplayName = firstNonEmpty(adminNullString(displayName), adminNullString(groupName))
		binding.GroupName = adminNullString(groupName)
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (s *Server) adminPrincipalsData(r *http.Request) ([]ui.AdminPrincipal, error) {
	rows, err := s.queryPrincipals(r)
	if err != nil {
		return nil, err
	}
	principals := make([]ui.AdminPrincipal, 0, len(rows))
	for _, row := range rows {
		principals = append(principals, ui.AdminPrincipal{
			ID:          stringMapValue(row, "id"),
			Email:       stringMapValue(row, "email"),
			DisplayName: stringMapValue(row, "displayName"),
			CreatedAt:   stringMapValue(row, "createdAt"),
			UpdatedAt:   stringMapValue(row, "updatedAt"),
		})
	}
	sort.SliceStable(principals, func(i, j int) bool {
		return adminPrincipalSortKey(principals[i]) < adminPrincipalSortKey(principals[j])
	})
	return principals, nil
}

func buildAdminPrincipals(principals []ui.AdminPrincipal, bindings []workspace.RoleBindingView, groupsByID map[string]access.Group, membersByGroup map[string][]ui.AdminPrincipalRef) []ui.AdminPrincipal {
	byID := make(map[string]*ui.AdminPrincipal, len(principals))
	out := make([]ui.AdminPrincipal, 0, len(principals))
	for _, principal := range principals {
		row := principal
		byID[row.ID] = &row
		out = append(out, row)
	}
	for _, binding := range bindings {
		if binding.SubjectType == string(access.SubjectPrincipal) && binding.PrincipalID != "" {
			if principal := byID[binding.PrincipalID]; principal != nil {
				principal.DirectRoles = appendUnique(principal.DirectRoles, binding.Role)
			}
		}
	}
	for groupID, members := range membersByGroup {
		group := groupsByID[groupID]
		for _, member := range members {
			if principal := byID[member.ID]; principal != nil {
				principal.Groups = append(principal.Groups, ui.AdminGroupRef{
					ID:         group.ID,
					Name:       group.Name,
					ExternalID: group.ExternalID,
				})
			}
		}
	}
	for i := range out {
		if principal := byID[out[i].ID]; principal != nil {
			sort.Strings(principal.DirectRoles)
			sort.SliceStable(principal.Groups, func(i, j int) bool {
				return principal.Groups[i].Name < principal.Groups[j].Name
			})
			out[i] = *principal
		}
	}
	return out
}

func buildAdminGroups(groups []access.Group, bindings []workspace.RoleBindingView, membersByGroup map[string][]ui.AdminPrincipalRef) []ui.AdminGroup {
	out := make([]ui.AdminGroup, 0, len(groups))
	byID := make(map[string]*ui.AdminGroup, len(groups))
	for _, group := range groups {
		row := ui.AdminGroup{
			ID:         group.ID,
			Name:       group.Name,
			Provider:   group.Provider,
			ExternalID: group.ExternalID,
			CreatedAt:  group.CreatedAt,
			Members:    membersByGroup[group.ID],
		}
		sort.SliceStable(row.Members, func(i, j int) bool {
			return adminPrincipalRefSortKey(row.Members[i]) < adminPrincipalRefSortKey(row.Members[j])
		})
		byID[row.ID] = &row
		out = append(out, row)
	}
	for _, binding := range bindings {
		if binding.SubjectType == string(access.SubjectGroup) && binding.GroupID != "" {
			if group := byID[binding.GroupID]; group != nil {
				group.Roles = appendUnique(group.Roles, binding.Role)
			}
		}
	}
	for i := range out {
		if group := byID[out[i].ID]; group != nil {
			sort.Strings(group.Roles)
			out[i] = *group
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func adminNullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func stringMapValue(row map[string]any, key string) string {
	if value, ok := row[key].(string); ok {
		return value
	}
	return ""
}

func adminPrincipalSortKey(row ui.AdminPrincipal) string {
	return firstNonEmpty(row.Email, row.DisplayName, row.ID)
}

func adminPrincipalRefSortKey(row ui.AdminPrincipalRef) string {
	return firstNonEmpty(row.Email, row.DisplayName, row.ID)
}
