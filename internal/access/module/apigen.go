package module

import (
	"fmt"
	"net/http"
	"strings"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
)

const apiGenObjectScopeExtension = "x-leapview-object-scope"

type APIGenObjectResolvers struct {
	Dashboard      ObjectResolver
	SemanticModel  ObjectResolver
	WorkspaceAsset ObjectResolver
}

type apiGenObjectScope struct {
	pathParameter string
	resolver      ObjectResolver
}

type APIGenAuthorizer struct {
	module *Module
	scopes map[string]apiGenObjectScope
}

func (m *Module) APIGenAuthorizer(resolvers APIGenObjectResolvers) (*APIGenAuthorizer, error) {
	if m == nil {
		return nil, fmt.Errorf("access module is required")
	}
	authorizer := &APIGenAuthorizer{
		module: m,
		scopes: map[string]apiGenObjectScope{
			"dashboard":       {pathParameter: "dashboard", resolver: resolvers.Dashboard},
			"semantic-model":  {pathParameter: "model", resolver: resolvers.SemanticModel},
			"workspace-asset": {pathParameter: "assetId", resolver: resolvers.WorkspaceAsset},
			"grant-management": {resolver: func(_ *http.Request, workspaceID string) []ObjectRef {
				return []ObjectRef{PlatformObject(), WorkspaceObject(workspaceID)}
			}},
			"principal": {resolver: func(_ *http.Request, workspaceID string) []ObjectRef {
				return []ObjectRef{PlatformObject(), WorkspaceObject(workspaceID)}
			}},
			"platform": {resolver: func(*http.Request, string) []ObjectRef {
				return []ObjectRef{PlatformObject()}
			}},
		},
	}
	for name, scope := range authorizer.scopes {
		if scope.pathParameter != "" && scope.resolver == nil {
			return nil, fmt.Errorf("APIGen object resolver %q is required", name)
		}
	}
	return authorizer, nil
}

func (a *APIGenAuthorizer) Protect(operationID string, next http.Handler) (http.Handler, bool) {
	contract, ok := apigenapi.GetAPIGenOperationContract(operationID)
	if !ok || !contract.Protected {
		return nil, false
	}
	privilege, ok := apiGenOperationPrivilege(contract)
	if !ok {
		return nil, false
	}
	if isGlobalAgentOperation(operationID) {
		return a.module.ProtectGlobal(privilege, next.ServeHTTP), true
	}
	resolver, ok := a.objectResolverForContract(contract)
	if !ok {
		return nil, false
	}
	return a.module.ProtectHandlerWithObjects(privilege, resolver, next), true
}

func apiGenOperationPrivilege(contract apigenapi.GenOperationContract) (Privilege, bool) {
	if contract.AuthzMode == "authenticated" {
		return "", true
	}
	if contract.AuthzMode != "privilege" {
		return "", false
	}
	authz, ok := contract.Extensions["x-authz"].(map[string]any)
	if !ok || authz["mode"] != "privilege" {
		return "", false
	}
	value, ok := authz["privilege"].(string)
	if !ok {
		return "", false
	}
	return ParsePrivilege(value)
}

func isGlobalAgentOperation(operationID string) bool {
	switch operationID {
	case "search", "listAgentConversations", "createAgentConversation", "archiveAgentConversation", "getAgentConversation", "updateAgentConversation",
		"listAgentMessages", "listAgentRuns", "createAgentRun", "getAgentRun", "cancelAgentRun", "listAgentEvents":
		return true
	default:
		return false
	}
}

func (a *APIGenAuthorizer) objectResolverForContract(contract apigenapi.GenOperationContract) (ObjectResolver, bool) {
	expectedScope, ambiguous := a.objectScopeForPath(contract.Path)
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
	definition, ok := a.scopes[scope]
	if !ok || definition.resolver == nil {
		return nil, false
	}
	if (expectedScope != "" && scope != expectedScope) || (expectedScope == "" && definition.pathParameter != "") {
		return nil, false
	}
	return definition.resolver, true
}

func (a *APIGenAuthorizer) objectScopeForPath(path string) (string, bool) {
	matched := ""
	for scope, definition := range a.scopes {
		if definition.pathParameter == "" || !strings.Contains(path, "{"+definition.pathParameter+"}") {
			continue
		}
		if matched != "" {
			return "", true
		}
		matched = scope
	}
	return matched, false
}
