package app

import (
	"net/http"
	"strings"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
)

const apiGenObjectScopeExtension = "x-leapview-object-scope"

type apiGenObjectScope struct {
	pathParameter string
	resolver      accessmodule.ObjectResolver
}

// TypeSpec assigns operations to these domain scopes. The handwritten boundary
// only maps a stable scope name to the domain behavior that resolves objects.
var apiGenObjectScopes = map[string]apiGenObjectScope{
	"dashboard": {pathParameter: "dashboard", resolver: dashboardmodule.DashboardObjectRefs},
	"grant-management": {resolver: func(_ *http.Request, workspaceID string) []accessmodule.ObjectRef {
		return []accessmodule.ObjectRef{accessmodule.PlatformObject(), accessmodule.WorkspaceObject(workspaceID)}
	}},
	"principal": {resolver: func(_ *http.Request, workspaceID string) []accessmodule.ObjectRef {
		return []accessmodule.ObjectRef{accessmodule.PlatformObject(), accessmodule.WorkspaceObject(workspaceID)}
	}},
	"platform": {resolver: func(*http.Request, string) []accessmodule.ObjectRef {
		return []accessmodule.ObjectRef{accessmodule.PlatformObject()}
	}},
	"semantic-model":  {pathParameter: "model", resolver: dashboardmodule.SemanticDatasetObjectRefs},
	"workspace-asset": {pathParameter: "assetId", resolver: workspacemodule.AssetObjectRefs},
}

func apigenOperationObjectResolver(operationID string) (accessmodule.ObjectResolver, bool) {
	contract, ok := apigenapi.GetAPIGenOperationContract(operationID)
	if !ok {
		return nil, false
	}
	return apigenObjectResolverForContract(contract)
}

func apigenObjectResolverForContract(contract apigenapi.GenOperationContract) (accessmodule.ObjectResolver, bool) {
	expectedScope, ambiguous := apigenObjectScopeForPath(contract.Path)
	if ambiguous {
		return nil, false
	}
	rawScope, hasScope := contract.Extensions[apiGenObjectScopeExtension]
	if !hasScope {
		return nil, expectedScope == ""
	}
	scope, ok := rawScope.(string)
	if !ok || scope == "" {
		return nil, false
	}
	definition, ok := apiGenObjectScopes[scope]
	if !ok || definition.resolver == nil {
		return nil, false
	}
	if (expectedScope != "" && scope != expectedScope) || (expectedScope == "" && definition.pathParameter != "") {
		return nil, false
	}
	return definition.resolver, true
}

func apigenObjectScopeForPath(path string) (string, bool) {
	matched := ""
	for scope, definition := range apiGenObjectScopes {
		if definition.pathParameter == "" {
			continue
		}
		if !strings.Contains(path, "{"+definition.pathParameter+"}") {
			continue
		}
		if matched != "" {
			return "", true
		}
		matched = scope
	}
	return matched, false
}
