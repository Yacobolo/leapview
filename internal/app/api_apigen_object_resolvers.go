package app

import (
	"github.com/Yacobolo/libredash/internal/access/httpauth"
	agenthttp "github.com/Yacobolo/libredash/internal/agent/http"
	queryhttp "github.com/Yacobolo/libredash/internal/analytics/query/http"
	dashboardhttp "github.com/Yacobolo/libredash/internal/dashboard/http"
	workspacehttp "github.com/Yacobolo/libredash/internal/workspace/http"
)

// Object resolver implementations remain handwritten because they translate
// request parameters into domain object references. TypeSpec owns the
// operation-to-scope assignments in api_apigen_object_scopes.gen.go.
var apigenObjectResolvers = map[apiGenObjectScope]httpauth.ObjectResolver{
	apiGenObjectScopeWorkspaceAsset:    workspacehttp.AssetObjectRefs,
	apiGenObjectScopeDashboard:         dashboardhttp.DashboardObjectRefs,
	apiGenObjectScopeSemanticModel:     queryhttp.SemanticDatasetObjectRefs,
	apiGenObjectScopeAgentConversation: agenthttp.ConversationObjectRefs,
}

func apigenOperationObjectResolver(operationID string) httpauth.ObjectResolver {
	return apigenObjectResolvers[apigenOperationObjectScopes[operationID]]
}
