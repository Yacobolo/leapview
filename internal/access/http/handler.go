package http

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/queryaudit"
	"github.com/go-chi/chi/v5"
)

type Principal struct {
	ID          string
	Email       string
	DisplayName string
}

type RepositoryProvider func() (access.Repository, error)
type QueryAuditRepositoryProvider func() (queryaudit.Repository, error)
type PrincipalProvider func(*stdhttp.Request) (Principal, bool)
type CredentialProvider func(*stdhttp.Request) (access.APICredential, bool)
type WorkspaceIDNormalizer func(string) string

type Handler struct {
	Repository           RepositoryProvider
	QueryAuditRepository QueryAuditRepositoryProvider
	CurrentPrincipal     PrincipalProvider
	CurrentCredential    CredentialProvider
	WorkspaceID          WorkspaceIDNormalizer
}

func (h Handler) GetCurrentPrincipal(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	writeJSON(w, stdhttp.StatusOK, currentPrincipalDTO(principal))
}

func (h Handler) ListCurrentPermissions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	workspaceID := h.workspaceID(r.URL.Query().Get("workspace"))
	permissions := knownPermissions()
	allowed := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if credential, ok := h.currentCredential(r); ok && !apiTokenAllows(credential.Token, workspaceID, permission) {
			continue
		}
		ok, err := repo.HasPermission(r.Context(), workspaceID, principal.ID, permission)
		if err != nil {
			writeJSONError(w, err, stdhttp.StatusInternalServerError)
			return
		}
		if ok {
			allowed = append(allowed, permission)
		}
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{"workspaceId": workspaceID, "permissions": allowed})
}

func (h Handler) ListCurrentAPITokens(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	rows, err := repo.ListAPITokens(r.Context(), principal.ID)
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, apiTokenDTO(row))
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) CreateCurrentAPIToken(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	var input struct {
		Name        string   `json:"name"`
		WorkspaceID string   `json:"workspaceId"`
		Permissions []string `json:"permissions"`
		ExpiresAt   string   `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	var expiresAt time.Time
	if strings.TrimSpace(input.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, input.ExpiresAt)
		if err != nil {
			writeJSONError(w, err, stdhttp.StatusBadRequest)
			return
		}
		expiresAt = parsed
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	token, row, err := repo.CreateAPITokenWithMetadata(r.Context(), access.APITokenInput{
		PrincipalID: principal.ID,
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		Permissions: input.Permissions,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusCreated, map[string]any{"token": token, "apiToken": apiTokenDTO(row)})
}

func (h Handler) RevokeCurrentAPIToken(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if err := repo.RevokeAPITokenForPrincipal(r.Context(), principal.ID, chi.URLParam(r, "token")); err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "revoked"})
}

func (h Handler) ListCurrentSessions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	rows, err := repo.ListSessions(r.Context(), principal.ID)
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, sessionDTO(row))
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) RevokeCurrentSession(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	principal, ok := h.currentPrincipal(r)
	if !ok {
		writeJSONError(w, fmt.Errorf("authenticated principal is required"), stdhttp.StatusUnauthorized)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if err := repo.RevokeSessionForPrincipal(r.Context(), principal.ID, chi.URLParam(r, "session")); err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "revoked"})
}

func (h Handler) ListPrincipals(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if _, ok := apiLimitForRequest(w, r); !ok {
		return
	}
	if _, ok := apiCursorOffsetForRequest(w, r); !ok {
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if repo == nil {
		_ = writePagedJSON(w, r, []map[string]any{})
		return
	}
	rows, err := repo.ListPrincipals(r.Context(), access.PrincipalFilter{
		Email: r.URL.Query().Get("email"),
		Query: r.URL.Query().Get("q"),
	})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, principalDTO(row))
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) GetPrincipal(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	principal, err := repo.PrincipalByID(r.Context(), chi.URLParam(r, "principal"))
	if err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	writeJSON(w, stdhttp.StatusOK, principalDTO(principal))
}

func (h Handler) UpdatePrincipal(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var input struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	existing, err := repo.PrincipalByID(r.Context(), chi.URLParam(r, "principal"))
	if err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	if strings.TrimSpace(input.DisplayName) != "" {
		existing.DisplayName = input.DisplayName
	}
	principal, err := repo.UpsertPrincipal(r.Context(), access.PrincipalInput{ID: existing.ID, Email: existing.Email, DisplayName: existing.DisplayName})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, principalDTO(principal))
}

func (h Handler) ListGroups(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	rows, err := repo.ListGroups(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")))
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, groupDTO(row))
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) CreateGroup(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var input struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	name := firstNonEmpty(input.DisplayName, input.Name)
	group, err := repo.UpsertGroup(r.Context(), access.GroupInput{WorkspaceID: h.workspaceID(chi.URLParam(r, "workspace")), Provider: "local", ExternalID: input.Name, Name: name})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusCreated, groupDTO(group))
}

func (h Handler) GetGroup(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	group, ok := h.groupByID(w, r)
	if !ok {
		return
	}
	writeJSON(w, stdhttp.StatusOK, groupDTO(group))
}

func (h Handler) UpdateGroup(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var input struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	group, ok := h.groupByID(w, r)
	if !ok {
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	updated, err := repo.UpsertGroup(r.Context(), access.GroupInput{ID: group.ID, WorkspaceID: group.WorkspaceID, Provider: group.Provider, ExternalID: group.ExternalID, Name: firstNonEmpty(input.DisplayName, group.Name)})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, groupDTO(updated))
}

func (h Handler) DeleteGroup(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	group, ok := h.groupByID(w, r)
	if !ok {
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if err := repo.DeleteGroup(r.Context(), group.WorkspaceID, group.ID); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h Handler) ListGroupMembers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	rows, err := repo.ListGroupMembers(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")), chi.URLParam(r, "group"))
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, groupMemberPrincipalDTO(row))
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) AddGroupMember(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if err := repo.AddGroupMember(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")), chi.URLParam(r, "group"), chi.URLParam(r, "principal")); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "added"})
}

func (h Handler) RemoveGroupMember(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if err := repo.RemoveGroupMember(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")), chi.URLParam(r, "group"), chi.URLParam(r, "principal")); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "removed"})
}

func (h Handler) ListWorkspaceRoles(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	roles, err := repo.ListRoles(r.Context())
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]api.RoleResponse, 0, len(roles))
	for _, role := range roles {
		out = append(out, api.RoleResponse{Name: role.Name, Permissions: role.Permissions})
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) ListRoleBindings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if repo == nil {
		_ = writePagedJSON(w, r, []map[string]any{})
		return
	}
	bindings, err := repo.ListRoleBindings(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")))
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, apiRoleBindingDTO(binding))
	}
	_ = writePagedJSON(w, r, out)
}

func (h Handler) CreateRoleBinding(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	input, ok := decodeRoleBindingInput(w, r)
	if !ok {
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	row, err := repo.CreateRoleBinding(r.Context(), input)
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusCreated, apiRoleBindingDTO(row))
}

func (h Handler) UpdateRoleBinding(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	input, ok := decodeRoleBindingInput(w, r)
	if !ok {
		return
	}
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	row, err := repo.UpdateRoleBinding(r.Context(), input.WorkspaceID, chi.URLParam(r, "binding"), input)
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, apiRoleBindingDTO(row))
}

func (h Handler) DeleteRoleBinding(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if repo == nil {
		writeJSONError(w, errors.New("Workspace RBAC store is not configured."), stdhttp.StatusInternalServerError)
		return
	}
	if err := repo.DeleteRoleBinding(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")), chi.URLParam(r, "binding")); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "removed"})
}

func (h Handler) ListAuditEvents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	limit, ok := apiLimitForRequest(w, r)
	if !ok {
		return
	}
	cursorTime, cursorID := decodeCursor(r.URL.Query().Get("pageToken"))
	rows, err := repo.ListAuditEvents(r.Context(), access.AuditEventFilter{
		WorkspaceID: h.workspaceID(chi.URLParam(r, "workspace")),
		PrincipalID: r.URL.Query().Get("actor"),
		Action:      r.URL.Query().Get("action"),
		TargetType:  r.URL.Query().Get("targetType"),
		TargetID:    r.URL.Query().Get("targetId"),
		From:        r.URL.Query().Get("from"),
		To:          r.URL.Query().Get("to"),
		CursorTime:  cursorTime,
		CursorID:    cursorID,
		Limit:       limit + 1,
	})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	nextCursor := ""
	if len(rows) > limit {
		last := rows[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditEventDTO(row))
	}
	writeJSON(w, stdhttp.StatusOK, pagedResponseWithCursor(out, nextCursor))
}

func (h Handler) ListQueryEvents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	repo, err := h.queryAuditRepository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	if repo == nil {
		writeJSON(w, stdhttp.StatusOK, pagedResponseWithCursor([]map[string]any{}, ""))
		return
	}
	limit, ok := apiLimitForRequest(w, r)
	if !ok {
		return
	}
	cursorTime, cursorID := decodeCursor(r.URL.Query().Get("pageToken"))
	rows, err := repo.ListQueryEvents(r.Context(), queryaudit.Filter{
		WorkspaceID:  h.workspaceID(chi.URLParam(r, "workspace")),
		PrincipalID:  r.URL.Query().Get("principal"),
		PrincipalIDs: cleanQueryValues(r.URL.Query()["principal"]),
		Surface:      r.URL.Query().Get("surface"),
		Surfaces:     cleanQueryValues(r.URL.Query()["surface"]),
		Operation:    r.URL.Query().Get("operation"),
		QueryKind:    r.URL.Query().Get("kind"),
		QueryKinds:   cleanQueryValues(r.URL.Query()["kind"]),
		ModelID:      firstNonEmpty(r.URL.Query().Get("modelId"), r.URL.Query().Get("model")),
		Target:       r.URL.Query().Get("target"),
		Status:       r.URL.Query().Get("status"),
		Statuses:     cleanQueryValues(r.URL.Query()["status"]),
		Search:       r.URL.Query().Get("search"),
		From:         r.URL.Query().Get("from"),
		To:           r.URL.Query().Get("to"),
		CursorTime:   cursorTime,
		CursorID:     cursorID,
		Limit:        limit + 1,
	})
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return
	}
	nextCursor := ""
	if len(rows) > limit {
		last := rows[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		rows = rows[:limit]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, queryEventDTO(row))
	}
	writeJSON(w, stdhttp.StatusOK, pagedResponseWithCursor(out, nextCursor))
}

func (h Handler) repository() (access.Repository, error) {
	if h.Repository == nil {
		return nil, nil
	}
	return h.Repository()
}

func (h Handler) queryAuditRepository() (queryaudit.Repository, error) {
	if h.QueryAuditRepository == nil {
		return nil, nil
	}
	return h.QueryAuditRepository()
}

func (h Handler) currentPrincipal(r *stdhttp.Request) (Principal, bool) {
	if h.CurrentPrincipal == nil {
		return Principal{}, false
	}
	return h.CurrentPrincipal(r)
}

func (h Handler) currentCredential(r *stdhttp.Request) (access.APICredential, bool) {
	if h.CurrentCredential == nil {
		return access.APICredential{}, false
	}
	return h.CurrentCredential(r)
}

func (h Handler) workspaceID(value string) string {
	if h.WorkspaceID == nil {
		return strings.TrimSpace(value)
	}
	return h.WorkspaceID(value)
}

func principalDTO(row access.Principal) map[string]any {
	return map[string]any{"id": row.ID, "email": row.Email, "displayName": row.DisplayName, "createdAt": row.CreatedAt, "updatedAt": row.UpdatedAt}
}

func currentPrincipalDTO(row Principal) map[string]any {
	return map[string]any{"id": row.ID, "email": row.Email, "displayName": row.DisplayName, "createdAt": "", "updatedAt": ""}
}

func groupDTO(row access.Group) map[string]any {
	return map[string]any{"id": row.ID, "name": row.ExternalID, "displayName": row.Name, "createdAt": row.CreatedAt, "updatedAt": row.CreatedAt}
}

func groupMemberPrincipalDTO(row access.GroupMember) map[string]any {
	return map[string]any{"id": row.PrincipalID, "email": row.Email, "displayName": row.DisplayName, "createdAt": row.CreatedAt, "updatedAt": row.CreatedAt}
}

func apiRoleBindingDTO(row access.RoleBinding) map[string]any {
	return map[string]any{"id": row.ID, "workspaceId": row.WorkspaceID, "subjectType": string(row.SubjectType), "subjectId": row.SubjectID, "email": row.Email, "displayName": firstNonEmpty(row.DisplayName, row.GroupName), "role": row.Role, "createdAt": row.CreatedAt}
}

func apiTokenDTO(row access.APIToken) map[string]any {
	return map[string]any{"id": row.ID, "name": row.Name, "workspaceId": row.WorkspaceID, "permissions": row.Permissions, "expiresAt": emptyToNil(row.ExpiresAt), "revokedAt": emptyToNil(row.RevokedAt), "createdAt": row.CreatedAt, "lastUsedAt": emptyToNil(row.LastUsedAt)}
}

func sessionDTO(row access.Session) map[string]any {
	return map[string]any{"id": row.ID, "createdAt": row.CreatedAt, "expiresAt": row.ExpiresAt, "lastSeenAt": emptyToNil(row.LastSeenAt), "revokedAt": emptyToNil(row.RevokedAt)}
}

func auditEventDTO(row access.AuditEvent) map[string]any {
	var metadata map[string]any
	if strings.TrimSpace(row.MetadataJSON) != "" {
		_ = json.Unmarshal([]byte(row.MetadataJSON), &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return map[string]any{"id": row.ID, "workspaceId": row.WorkspaceID, "principalId": row.PrincipalID, "action": row.Action, "targetType": row.TargetType, "targetId": row.TargetID, "metadata": metadata, "createdAt": row.CreatedAt}
}

func queryEventDTO(row queryaudit.Event) map[string]any {
	var query map[string]any
	if strings.TrimSpace(row.QueryJSON) != "" {
		_ = json.Unmarshal([]byte(row.QueryJSON), &query)
	}
	if query == nil {
		query = map[string]any{}
	}
	return map[string]any{
		"id":            row.ID,
		"workspaceId":   row.WorkspaceID,
		"principalId":   emptyToNil(row.PrincipalID),
		"surface":       row.Surface,
		"operation":     row.Operation,
		"queryKind":     row.QueryKind,
		"modelId":       row.ModelID,
		"target":        row.Target,
		"objectType":    row.ObjectType,
		"objectId":      row.ObjectID,
		"requestId":     row.RequestID,
		"correlationId": row.CorrelationID,
		"status":        row.Status,
		"durationMs":    row.DurationMS,
		"rowsReturned":  row.RowsReturned,
		"bytesEstimate": row.BytesEstimate,
		"error":         emptyToNil(row.Error),
		"sql":           emptyToNil(row.SQL),
		"planText":      emptyToNil(row.PlanText),
		"query":         query,
		"createdAt":     row.CreatedAt,
	}
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func knownPermissions() []string {
	return []string{
		access.PermissionWorkspaceRead,
		access.PermissionAssetRead,
		access.PermissionDeploymentRead,
		access.PermissionDeploymentWrite,
		access.PermissionDeploymentActivate,
		access.PermissionRBACRead,
		access.PermissionRBACWrite,
		access.PermissionAgentUse,
		access.PermissionAgentRead,
		access.PermissionMaterializationRun,
		access.PermissionAuditRead,
		access.PermissionTokenManage,
	}
}

func (h Handler) groupByID(w stdhttp.ResponseWriter, r *stdhttp.Request) (access.Group, bool) {
	repo, err := h.repository()
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return access.Group{}, false
	}
	rows, err := repo.ListGroups(r.Context(), h.workspaceID(chi.URLParam(r, "workspace")))
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusInternalServerError)
		return access.Group{}, false
	}
	for _, row := range rows {
		if row.ID == chi.URLParam(r, "group") {
			return row, true
		}
	}
	writeJSONError(w, sql.ErrNoRows, stdhttp.StatusNotFound)
	return access.Group{}, false
}

func decodeRoleBindingInput(w stdhttp.ResponseWriter, r *stdhttp.Request) (access.RoleBindingInput, bool) {
	var input struct {
		SubjectType string `json:"subjectType"`
		SubjectID   string `json:"subjectId"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return access.RoleBindingInput{}, false
	}
	return access.RoleBindingInput{
		WorkspaceID: chi.URLParam(r, "workspace"),
		SubjectType: access.SubjectType(input.SubjectType),
		SubjectID:   input.SubjectID,
		Role:        input.Role,
	}, true
}

func apiTokenAllows(token access.APIToken, workspaceID, permission string) bool {
	if token.WorkspaceID != "" && token.WorkspaceID != workspaceID {
		return false
	}
	if token.Permissions == nil {
		return true
	}
	for _, allowed := range token.Permissions {
		if allowed == permission {
			return true
		}
	}
	return false
}

type pageResponse struct {
	NextCursor string `json:"nextCursor"`
}

func pagedResponseWithCursor(items any, nextCursor string) map[string]any {
	return map[string]any{"items": items, "page": pageResponse{NextCursor: nextCursor}}
}

func writePagedJSON[T any](w stdhttp.ResponseWriter, r *stdhttp.Request, items []T) bool {
	page, nextCursor, ok := pageSliceForRequest(w, r, items)
	if !ok {
		return false
	}
	writeJSON(w, stdhttp.StatusOK, pagedResponseWithCursor(page, nextCursor))
	return true
}

func pageSliceForRequest[T any](w stdhttp.ResponseWriter, r *stdhttp.Request, items []T) ([]T, string, bool) {
	limit, ok := apiLimitForRequest(w, r)
	if !ok {
		return nil, "", false
	}
	start, ok := apiCursorOffsetForRequest(w, r)
	if !ok {
		return nil, "", false
	}
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	nextCursor := ""
	if end < len(items) {
		nextCursor = encodeIndexCursor(end)
	}
	return append([]T(nil), items[start:end]...), nextCursor, true
}

const (
	defaultAPILimit = 50
	maxAPILimit     = 100
)

func apiLimitForRequest(w stdhttp.ResponseWriter, r *stdhttp.Request) (int, bool) {
	limit, err := parseAPILimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return 0, false
	}
	return limit, true
}

func parseAPILimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAPILimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if value <= 0 {
		return defaultAPILimit, nil
	}
	if value > maxAPILimit {
		return maxAPILimit, nil
	}
	return value, nil
}

func apiCursorOffsetForRequest(w stdhttp.ResponseWriter, r *stdhttp.Request) (int, bool) {
	offset, err := decodeIndexCursor(r.URL.Query().Get("pageToken"))
	if err != nil {
		writeJSONError(w, err, stdhttp.StatusBadRequest)
		return 0, false
	}
	return offset, true
}

func encodeIndexCursor(offset int) string {
	if offset <= 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("idx:%d", offset)))
}

func decodeIndexCursor(token string) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, fmt.Errorf("pageToken is invalid")
	}
	text := string(raw)
	if !strings.HasPrefix(text, "idx:") {
		return 0, fmt.Errorf("pageToken is invalid")
	}
	value, err := strconv.Atoi(strings.TrimPrefix(text, "idx:"))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("pageToken is invalid")
	}
	return value, nil
}

func cleanQueryValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func encodeCursor(createdAt, id string) string {
	if createdAt == "" || id == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt + "\x00" + id))
}

func decodeCursor(token string) (string, string) {
	if strings.TrimSpace(token) == "" {
		return "", ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ""
	}
	createdAt, id, ok := strings.Cut(string(raw), "\x00")
	if !ok {
		return "", ""
	}
	return createdAt, id
}

func writeJSON(w stdhttp.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w stdhttp.ResponseWriter, err error, status int) {
	writeJSON(w, status, api.ErrorResponse{
		Code:      status,
		Message:   err.Error(),
		Details:   map[string]any{},
		RequestID: "",
	})
}

func statusForNotFound(err error) int {
	if err == sql.ErrNoRows {
		return stdhttp.StatusNotFound
	}
	return stdhttp.StatusInternalServerError
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
