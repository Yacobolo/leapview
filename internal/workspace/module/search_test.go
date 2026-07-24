package module

import (
	"context"
	"testing"

	"github.com/Yacobolo/leapview/internal/access"
	productsearch "github.com/Yacobolo/leapview/internal/search"
)

func TestSearchAPIResultsIncludeVisualSubtype(t *testing.T) {
	items := searchAPIResults([]productsearch.Result{{
		Reference:  productsearch.Reference{WorkspaceID: "sales", Type: productsearch.TypeVisual, ID: "orders.revenue"},
		Name:       "Revenue",
		VisualType: "line",
		Workspace:  productsearch.Workspace{ID: "sales", Name: "Sales"},
		Locations:  []productsearch.Location{},
		Context:    []productsearch.ContextTag{},
	}})
	if len(items) != 1 || items[0].VisualType == nil || *items[0].VisualType != "line" {
		t.Fatalf("search API visual subtype = %#v", items)
	}
}

func TestSearchAuthorizerRejectsCredentialWithoutViewPrivilege(t *testing.T) {
	called := false
	allowed, err := (searchAuthorizer{authorize: func(context.Context, string, access.Privilege, access.ObjectRef) (bool, error) {
		called = true
		return true, nil
	}}).CanView(t.Context(), productsearch.Subject{
		ID: "principal", CredentialRestricted: true, Privileges: []string{string(access.PrivilegeUseAgent)},
	}, access.WorkspaceObject("sales"))
	if err != nil || allowed || called {
		t.Fatalf("allowed = %v, authorize called = %v, err = %v", allowed, called, err)
	}
}
