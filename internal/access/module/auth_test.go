package module

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yacobolo/leapview/internal/access"
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
