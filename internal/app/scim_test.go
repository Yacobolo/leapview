package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
)

const testSCIMToken = "test-scim-token"

func TestSCIMRoutesRequireBearerAndServeMetadata(t *testing.T) {
	store := testStore(t)
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, DefaultWorkspaceID: "test", SCIMBearerToken: testSCIMToken})

	missingToken := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	missingRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(missingRec, missingToken)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want %d body=%s", missingRec.Code, http.StatusUnauthorized, missingRec.Body.String())
	}

	for _, path := range []string{"/scim/v2/ServiceProviderConfig", "/scim/v2/Schemas", "/scim/v2/ResourceTypes"} {
		req := scimRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		server.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d body=%s", path, rec.Code, http.StatusOK, rec.Body.String())
		}
	}
}

func TestSCIMUserAndGroupProvisioningDriveGrantAccess(t *testing.T) {
	store := testStore(t)
	repo := testAccessRepository(store)
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, AccessRepo: repo, DefaultWorkspaceID: "test", SCIMBearerToken: testSCIMToken})
	ctx := context.Background()

	userID := createSCIMUser(t, server, "user-ext-1", "analyst@example.com", "Analyst User")
	groupID := createSCIMGroup(t, server, "group-ext-1", "Analysts", []string{userID})
	groups, err := repo.ListGroups(ctx, "test")
	if err != nil {
		t.Fatalf("list grantable groups: %v", err)
	}
	if !hasSCIMGroup(groups, groupID) {
		t.Fatalf("grantable groups = %#v, want SCIM directory group %s", groups, groupID)
	}
	if _, err := repo.CreateGrant(ctx, access.GrantInput{
		Object:      access.WorkspaceObject("test"),
		SubjectType: access.SubjectGroup,
		SubjectID:   groupID,
		Privilege:   access.PrivilegeUseWorkspace,
	}); err != nil {
		t.Fatalf("create group grant: %v", err)
	}

	decision, err := repo.Authorize(ctx, userID, access.PrivilegeUseWorkspace, access.WorkspaceObject("test"))
	if err != nil {
		t.Fatalf("authorize provisioned group member: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("provisioned group member was not allowed: %#v", decision)
	}

	removeBody := map[string]any{
		"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{{"op": "remove", "path": `members[value eq "` + userID + `"]`}},
	}
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, scimRequest(http.MethodPatch, "/scim/v2/Groups/"+url.PathEscape(groupID), removeBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("remove group member status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	decision, err = repo.Authorize(ctx, userID, access.PrivilegeUseWorkspace, access.WorkspaceObject("test"))
	if err != nil {
		t.Fatalf("authorize removed member: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("removed SCIM member still has group grant access: %#v", decision)
	}
}

func hasSCIMGroup(groups []access.Group, id string) bool {
	for _, group := range groups {
		if group.ID == id && group.Provider == "scim" && group.WorkspaceID == "" {
			return true
		}
	}
	return false
}

func TestSCIMDisableRevokesCredentialsAndBlocksAuthorization(t *testing.T) {
	store := testStore(t)
	repo := testAccessRepository(store)
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, AccessRepo: repo, DefaultWorkspaceID: "test", SCIMBearerToken: testSCIMToken})
	ctx := context.Background()

	userID := createSCIMUser(t, server, "user-ext-2", "disabled@example.com", "Disabled User")
	if _, err := repo.CreateGrant(ctx, access.GrantInput{
		Object:      access.WorkspaceObject("test"),
		SubjectType: access.SubjectPrincipal,
		SubjectID:   userID,
		Privilege:   access.PrivilegeUseWorkspace,
	}); err != nil {
		t.Fatalf("create direct grant: %v", err)
	}
	sessionToken, err := repo.CreateSession(ctx, userID, time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	apiToken, _, err := repo.CreateAPITokenWithMetadata(ctx, access.APITokenInput{
		PrincipalID: userID,
		WorkspaceID: "test",
		Name:        "disabled-user-token",
		Permissions: []access.Privilege{access.PrivilegeUseWorkspace},
	})
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}

	patchBody := map[string]any{
		"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": []map[string]any{{"op": "replace", "path": "active", "value": false}},
	}
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, scimRequest(http.MethodPatch, "/scim/v2/Users/"+url.PathEscape(userID), patchBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("disable user status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if _, err := repo.PrincipalForToken(ctx, sessionToken); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("disabled principal session err = %v, want sql.ErrNoRows", err)
	}
	if _, err := repo.CredentialForAPIToken(ctx, apiToken); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("disabled principal api token err = %v, want sql.ErrNoRows", err)
	}
	decision, err := repo.Authorize(ctx, userID, access.PrivilegeUseWorkspace, access.WorkspaceObject("test"))
	if err != nil {
		t.Fatalf("authorize disabled principal: %v", err)
	}
	if decision.Allowed || decision.Reason != "principal_disabled" {
		t.Fatalf("disabled principal decision = %#v, want denied principal_disabled", decision)
	}
}

func TestSCIMFiltersUsersAndGroups(t *testing.T) {
	store := testStore(t)
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, DefaultWorkspaceID: "test", SCIMBearerToken: testSCIMToken})
	userID := createSCIMUser(t, server, "filter-user", "filter@example.com", "Filter User")
	groupID := createSCIMGroup(t, server, "filter-group", "Filter Analysts", []string{userID})

	userRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(userRec, scimRequest(http.MethodGet, `/scim/v2/Users?filter=userName%20eq%20%22filter@example.com%22`, nil))
	if userRec.Code != http.StatusOK {
		t.Fatalf("user filter status = %d body=%s", userRec.Code, userRec.Body.String())
	}
	if totalResults(t, userRec.Body.Bytes()) != 1 {
		t.Fatalf("user filter body=%s, want one result", userRec.Body.String())
	}

	groupRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(groupRec, scimRequest(http.MethodGet, `/scim/v2/Groups?filter=displayName%20eq%20%22Filter%20Analysts%22`, nil))
	if groupRec.Code != http.StatusOK {
		t.Fatalf("group filter status = %d body=%s", groupRec.Code, groupRec.Body.String())
	}
	if totalResults(t, groupRec.Body.Bytes()) != 1 {
		t.Fatalf("group filter body=%s, want one result for group %s", groupRec.Body.String(), groupID)
	}
}

func createSCIMUser(t *testing.T, server *Server, externalID, email, displayName string) string {
	t.Helper()
	body := map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		"externalId":  externalID,
		"userName":    email,
		"displayName": displayName,
		"name":        map[string]any{"formatted": displayName},
		"active":      true,
		"emails":      []map[string]any{{"value": email, "type": "work", "primary": true}},
	}
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, scimRequest(http.MethodPost, "/scim/v2/Users", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create SCIM user status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	return resourceID(t, rec.Body.Bytes())
}

func createSCIMGroup(t *testing.T, server *Server, externalID, displayName string, members []string) string {
	t.Helper()
	memberAttrs := make([]map[string]any, 0, len(members))
	for _, member := range members {
		memberAttrs = append(memberAttrs, map[string]any{"value": member})
	}
	body := map[string]any{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		"externalId":  externalID,
		"displayName": displayName,
		"members":     memberAttrs,
	}
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, scimRequest(http.MethodPost, "/scim/v2/Groups", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create SCIM group status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	return resourceID(t, rec.Body.Bytes())
}

func scimRequest(method, path string, body any) *http.Request {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+testSCIMToken)
	req.Header.Set("Content-Type", "application/scim+json")
	req.Header.Set("Accept", "application/scim+json")
	return req
}

func resourceID(t *testing.T, body []byte) string {
	t.Helper()
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode resource: %v body=%s", err, string(body))
	}
	if decoded.ID == "" {
		t.Fatalf("resource id missing: %s", string(body))
	}
	return decoded.ID
}

func totalResults(t *testing.T, body []byte) int {
	t.Helper()
	var decoded struct {
		TotalResults int `json:"totalResults"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode list response: %v body=%s", err, string(body))
	}
	return decoded.TotalResults
}
