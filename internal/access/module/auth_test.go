package module

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flidai/leapview/internal/access"
	"github.com/go-chi/chi/v5"
)

func TestPrivilegeWorkspaceIDUsesConfiguredWorkspaceWhenRouteHasNoScope(t *testing.T) {
	auth := &Auth{workspaceID: "default-workspace"}

	request := httptest.NewRequest("POST", "/api/v1/principals", nil)
	if got := auth.privilegeWorkspaceID(request); got != "default-workspace" {
		t.Fatalf("unscoped route workspace = %q, want configured default", got)
	}
}

func TestPrivilegeWorkspaceIDPreservesExplicitAPIWorkspace(t *testing.T) {
	auth := &Auth{workspaceID: "default-workspace"}

	request := httptest.NewRequest("GET", "/api/v1/workspaces/acme/groups", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("workspace", "acme")
	request = request.WithContext(contextWithRouteContext(request, routeContext))

	if got := auth.privilegeWorkspaceID(request); got != "acme" {
		t.Fatalf("workspace API route workspace = %q, want acme", got)
	}
}

func contextWithRouteContext(request *http.Request, routeContext *chi.Context) context.Context {
	return context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
}

func TestAuthorizationObjectsIncludePlatformForSessionAuthentication(t *testing.T) {
	objects := authorizationObjects(nil, "", nil, access.PrivilegeViewAudit)
	if len(objects) != 1 || objects[0] != access.PlatformObject() {
		t.Fatalf("authorization objects = %#v, want platform object", objects)
	}
}

func TestAuthorizationObjectsDoNotExpandWorkspaceScopedAPITokenToPlatform(t *testing.T) {
	credential := &access.APICredential{Token: access.APIToken{
		WorkspaceID: "acme",
		Privileges:  []access.Privilege{access.PrivilegeViewAudit},
	}}
	objects := authorizationObjects([]string{"acme"}, "", credential, access.PrivilegeViewAudit)
	if len(objects) != 1 || objects[0] != access.WorkspaceObject("acme") {
		t.Fatalf("authorization objects = %#v, want only acme workspace", objects)
	}
}

func TestAuthorizationObjectsIncludeConfiguredWorkspaceBeforeItIsPersisted(t *testing.T) {
	credential := &access.APICredential{Token: access.APIToken{
		WorkspaceID: "test",
		Privileges:  []access.Privilege{access.PrivilegeViewAudit},
	}}
	objects := authorizationObjects(nil, "test", credential, access.PrivilegeViewAudit)
	if len(objects) != 1 || objects[0] != access.WorkspaceObject("test") {
		t.Fatalf("authorization objects = %#v, want configured test workspace", objects)
	}
}

func TestAuthorizationDenialAuditInputIdentifiesTheDeniedObject(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/v1/workspaces/acme/semantic-models/sales/query", nil)
	request.Header.Set("X-Request-ID", "request-1")
	request.Header.Set("X-Correlation-ID", "correlation-1")
	input := authorizationDenialAuditInput(
		request,
		"principal-1",
		"acme",
		access.PrivilegeQueryData,
		[]access.ObjectRef{access.ItemObject(access.SecurableSemanticModel, "acme", "sales")},
		access.ReasonMissingPrivilege,
	)
	if input.Action != "authorization.denied" || input.Status != "denied" {
		t.Fatalf("denial audit action/status = %q/%q", input.Action, input.Status)
	}
	if input.WorkspaceID != "acme" || input.PrincipalID != "principal-1" {
		t.Fatalf("denial audit identity = %#v", input)
	}
	if input.TargetType != "semantic_model" || input.TargetID != "semantic_model:acme:sales" {
		t.Fatalf("denial audit target = %q/%q", input.TargetType, input.TargetID)
	}
	if input.Privilege != access.PrivilegeQueryData || input.RequestID != "request-1" || input.CorrelationID != "correlation-1" {
		t.Fatalf("denial audit request contract = %#v", input)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(input.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["reason"] != string(access.ReasonMissingPrivilege) {
		t.Fatalf("denial audit metadata = %#v", metadata)
	}
}

func TestAuthorizationAllowedAuditInputIdentifiesConnectionDecision(t *testing.T) {
	request := httptest.NewRequest(
		"POST",
		"/api/v1/workspaces/acme/targets/prod/environments/prod/connection-bindings/warehouse/test",
		nil,
	)
	request.Header.Set("X-Request-ID", "request-1")
	input := authorizationAllowedAuditInput(
		request,
		"operator-1",
		"acme",
		access.PrivilegeTestConnection,
		[]access.ObjectRef{access.WorkspaceObject("acme")},
	)
	if input.Action != "authorization.allowed" || input.Status != "allowed" ||
		input.WorkspaceID != "acme" || input.PrincipalID != "operator-1" ||
		input.TargetType != "workspace" || input.TargetID != "workspace:acme" ||
		input.Privilege != access.PrivilegeTestConnection || input.RequestID != "request-1" {
		t.Fatalf("allowed audit input = %#v", input)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(input.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["reason"] != "granted" {
		t.Fatalf("allowed audit metadata = %#v", metadata)
	}
}
