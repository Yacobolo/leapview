package app

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/queryaudit"
	"github.com/go-chi/chi/v5"
)

type pageResponse struct {
	NextCursor string `json:"nextCursor"`
}

func pagedResponse(items any) map[string]any {
	return pagedResponseWithCursor(items, "")
}

func pagedResponseWithCursor(items any, nextCursor string) map[string]any {
	return map[string]any{"items": items, "page": pageResponse{NextCursor: nextCursor}}
}

func writePagedJSON[T any](w http.ResponseWriter, r *http.Request, items []T) bool {
	page, nextCursor, ok := pageSliceForRequest(w, r, items)
	if !ok {
		return false
	}
	writeJSON(w, http.StatusOK, pagedResponseWithCursor(page, nextCursor))
	return true
}

func pageSliceForRequest[T any](w http.ResponseWriter, r *http.Request, items []T) ([]T, string, bool) {
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

func currentPrincipal(s *Server, r *http.Request) (Principal, bool) {
	if s.auth == nil {
		return localDeveloperPrincipal(), true
	}
	return s.auth.Principal(r)
}

func currentAPICredential(s *Server, r *http.Request) (access.APICredential, bool) {
	if s.auth == nil {
		return access.APICredential{}, false
	}
	return s.auth.APICredential(r)
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

func (s *Server) groupByID(w http.ResponseWriter, r *http.Request) (access.Group, bool) {
	repo, err := s.accessRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return access.Group{}, false
	}
	rows, err := repo.ListGroups(r.Context(), s.workspaceID(chi.URLParam(r, "workspace")))
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return access.Group{}, false
	}
	for _, row := range rows {
		if row.ID == chi.URLParam(r, "group") {
			return row, true
		}
	}
	writeJSONError(w, sql.ErrNoRows, http.StatusNotFound)
	return access.Group{}, false
}

func decodeRoleBindingInput(w http.ResponseWriter, r *http.Request) (access.RoleBindingInput, bool) {
	var input struct {
		SubjectType string `json:"subjectType"`
		SubjectID   string `json:"subjectId"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return access.RoleBindingInput{}, false
	}
	return access.RoleBindingInput{
		WorkspaceID: chi.URLParam(r, "workspace"),
		SubjectType: access.SubjectType(input.SubjectType),
		SubjectID:   input.SubjectID,
		Role:        input.Role,
	}, true
}

const (
	defaultAPILimit = 50
	maxAPILimit     = 100
)

func apiLimitForRequest(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit, err := parseAPILimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
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

func apiCursorOffsetForRequest(w http.ResponseWriter, r *http.Request) (int, bool) {
	offset, err := decodeIndexCursor(r.URL.Query().Get("pageToken"))
	if err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
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
