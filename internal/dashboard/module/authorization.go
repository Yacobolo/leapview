package module

import (
	"net/http"

	"github.com/flidai/leapview/internal/access"
	dashboardhttp "github.com/flidai/leapview/internal/dashboard/http"
	semanticapi "github.com/flidai/leapview/internal/dashboard/semanticapi"
)

// DashboardObjectRefs resolves the authorization chain for dashboard
// operations without exposing the dashboard HTTP adapter to composition.
func DashboardObjectRefs(r *http.Request, workspaceID string) []access.ObjectRef {
	return dashboardhttp.DashboardObjectRefs(r, workspaceID)
}

// SemanticDatasetObjectRefs resolves the authorization chain for semantic
// dataset operations without exposing the semantic HTTP adapter to composition.
func SemanticDatasetObjectRefs(r *http.Request, workspaceID string) []access.ObjectRef {
	return semanticapi.SemanticDatasetObjectRefs(r, workspaceID)
}
