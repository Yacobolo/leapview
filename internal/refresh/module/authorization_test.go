package module

import (
	"context"
	"net/http"
	"testing"

	"github.com/Yacobolo/leapview/internal/access"
)

func TestAuthorizePipelineAllowsDevelopmentBypassWithoutResolution(t *testing.T) {
	allowed, err := authorizePipeline(newAuthorizationRequest(t), "sales", "daily", access.PrivilegeRefreshData, AuthorizationConfig{
		CurrentPrincipal: func(*http.Request) (AuthorizationPrincipal, bool) {
			return AuthorizationPrincipal{ID: "dev", DevBypass: true}, true
		},
	})
	if err != nil || !allowed {
		t.Fatalf("allowed = %v, err = %v", allowed, err)
	}
}

func TestAuthorizePipelineRejectsCredentialOutsideWorkspaceBeforeResolution(t *testing.T) {
	resolved := false
	allowed, err := authorizePipeline(newAuthorizationRequest(t), "sales", "daily", access.PrivilegeRefreshData, AuthorizationConfig{
		CurrentPrincipal: func(*http.Request) (AuthorizationPrincipal, bool) {
			return AuthorizationPrincipal{ID: "agent"}, true
		},
		CurrentCredential: func(*http.Request) (access.APICredential, bool) {
			return access.APICredential{Token: access.APIToken{WorkspaceID: "finance"}}, true
		},
		ResolvePipelineModel: func(context.Context, string, string) (string, bool, error) {
			resolved = true
			return "orders", true, nil
		},
	})
	if err != nil || allowed || resolved {
		t.Fatalf("allowed = %v, resolved = %v, err = %v", allowed, resolved, err)
	}
}

func TestAuthorizePipelineUsesSemanticModelObject(t *testing.T) {
	var object access.ObjectRef
	allowed, err := authorizePipeline(newAuthorizationRequest(t), "sales", " daily ", access.PrivilegeViewItem, AuthorizationConfig{
		CurrentPrincipal: func(*http.Request) (AuthorizationPrincipal, bool) {
			return AuthorizationPrincipal{ID: "reader"}, true
		},
		ResolvePipelineModel: func(_ context.Context, workspaceID, pipelineID string) (string, bool, error) {
			if workspaceID != "sales" || pipelineID != "daily" {
				t.Fatalf("resolution input = %q %q", workspaceID, pipelineID)
			}
			return "orders", true, nil
		},
		AuthorizeObject: func(_ context.Context, principalID string, privilege access.Privilege, candidate access.ObjectRef) (bool, error) {
			if principalID != "reader" || privilege != access.PrivilegeViewItem {
				t.Fatalf("authorization input = %q %q", principalID, privilege)
			}
			object = candidate
			return true, nil
		},
	})
	if err != nil || !allowed {
		t.Fatalf("allowed = %v, err = %v", allowed, err)
	}
	if object.Type != access.SecurableSemanticModel || object.WorkspaceID != "sales" || object.ObjectID != "orders" {
		t.Fatalf("object = %#v", object)
	}
}

func newAuthorizationRequest(t *testing.T) *http.Request {
	t.Helper()
	return (&http.Request{}).WithContext(t.Context())
}
