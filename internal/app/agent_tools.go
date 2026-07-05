package app

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/Yacobolo/libredash/internal/access"
	agentcap "github.com/Yacobolo/libredash/internal/agent"
	agenttools "github.com/Yacobolo/libredash/internal/agent/tools"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/workspace"
	agentcore "github.com/Yacobolo/libredash/pkg/agent"
)

func (s *Server) configureAgentTools() {
	if s.agent == nil {
		return
	}
	if s.store != nil {
		s.agent.SetSystemPromptProvider(s.agentSystemPrompt)
	}
	s.agent.SetPolicyProvider(s.agentPolicyForScope)
	visualProvider := s.agentVisualToolProvider()
	apigenProvider := s.agentAPIGenToolProvider()
	s.agent.AppendToolProviders(
		func(scope agentcap.Scope) []agentcore.ToolDefinition {
			return visualProvider.Definitions(agentToolsScope(scope))
		},
		func(scope agentcap.Scope) []agentcore.ToolDefinition {
			return apigenProvider.Definitions(agentToolsScope(scope))
		},
	)
}

func (s *Server) agentPolicyForScope(scope agentcap.Scope) (workspace.AgentPolicy, bool) {
	metrics, ok := s.metricsForWorkspace(scope.WorkspaceID)
	if !ok || metrics == nil {
		return workspace.AgentPolicy{}, false
	}
	provider, ok := metrics.(agentPolicyProvider)
	if !ok {
		return workspace.AgentPolicy{}, false
	}
	return provider.AgentPolicy(), true
}

func (s *Server) agentVisualToolProvider() agenttools.VisualProvider {
	return agenttools.VisualProvider{
		Authorize: func(ctx context.Context, scope agenttools.Scope) (agentcore.ToolResult, bool) {
			return s.authorizeAgentPermission(ctx, agentScopeFromTools(scope), access.PermissionAssetRead)
		},
		SemanticModel: func(modelID string) (model *semanticmodel.Model, ok bool) {
			if s.metrics == nil {
				return nil, false
			}
			return s.metrics.SemanticModel(modelID)
		},
		AggregateRows: func(ctx context.Context, modelID string, request reportdef.AggregateQuery) (reportdef.QueryRows, error) {
			return executeAggregateRows(ctx, s.metrics, modelID, request)
		},
		PreviewRows: func(ctx context.Context, modelID string, request reportdef.RowQuery) (reportdef.QueryRows, error) {
			return executePreviewRows(ctx, s.metrics, modelID, request)
		},
	}
}

func (s *Server) agentAPIGenToolProvider() agenttools.APIGenProvider {
	return agenttools.APIGenProvider{
		Authorize: func(ctx context.Context, scope agenttools.Scope, operationID string) (agentcore.ToolResult, bool) {
			return s.authorizeAPIGenAgentOperation(ctx, agentScopeFromTools(scope), operationID)
		},
		Dispatch: func(operationID string, request *http.Request) (*http.Response, bool) {
			recorder := httptest.NewRecorder()
			if ok := apigenapi.DispatchAPIGenOperation(operationID, apiGenAdapter{server: s}, recorder, request); !ok {
				return nil, false
			}
			return recorder.Result(), true
		},
	}
}

func agentToolsScope(scope agentcap.Scope) agenttools.Scope {
	return agenttools.Scope{
		WorkspaceID:   scope.WorkspaceID,
		PrincipalID:   scope.PrincipalID,
		DevAuthBypass: scope.DevAuthBypass,
		Credential: agenttools.CredentialScope{
			WorkspaceID: scope.Credential.WorkspaceID,
			Restricted:  scope.Credential.Restricted,
			Permissions: append([]string{}, scope.Credential.Permissions...),
		},
	}
}

func agentScopeFromTools(scope agenttools.Scope) agentcap.Scope {
	return agentcap.Scope{
		WorkspaceID:   scope.WorkspaceID,
		PrincipalID:   scope.PrincipalID,
		DevAuthBypass: scope.DevAuthBypass,
		Credential: agentcap.CredentialScope{
			WorkspaceID: scope.Credential.WorkspaceID,
			Restricted:  scope.Credential.Restricted,
			Permissions: append([]string{}, scope.Credential.Permissions...),
		},
	}
}

func (s *Server) authorizeAPIGenAgentOperation(ctx context.Context, scope agentcap.Scope, operationID string) (agentcore.ToolResult, bool) {
	permission := apigenOperationPermissions[operationID]
	if permission == "" {
		return agenttools.ToolError("forbidden", "operation has no LibreDash permission mapping"), false
	}
	return s.authorizeAgentPermission(ctx, scope, permission)
}

func (s *Server) authorizeAgentPermission(ctx context.Context, scope agentcap.Scope, permission string) (agentcore.ToolResult, bool) {
	if scope.PrincipalID == "" {
		return agenttools.ToolError("unauthorized", "agent tool requires an authenticated principal"), false
	}
	if !agentCredentialAllows(scope, permission) {
		return agenttools.ToolError("forbidden", "credential is not allowed to call this tool"), false
	}
	if scope.DevAuthBypass {
		return agentcore.ToolResult{}, true
	}
	repo, err := s.accessRepository()
	if err != nil {
		return agenttools.ToolError("authorization_failed", err.Error()), false
	}
	if repo == nil {
		return agentcore.ToolResult{}, true
	}
	allowed, err := repo.HasPermission(ctx, scope.WorkspaceID, scope.PrincipalID, permission)
	if err != nil {
		return agenttools.ToolError("authorization_failed", err.Error()), false
	}
	if !allowed {
		return agenttools.ToolError("forbidden", "principal does not have permission to call this tool"), false
	}
	return agentcore.ToolResult{}, true
}

func agentCredentialAllows(scope agentcap.Scope, permission string) bool {
	credential := scope.Credential
	if credential.WorkspaceID != "" && credential.WorkspaceID != scope.WorkspaceID {
		return false
	}
	if !credential.Restricted {
		return true
	}
	for _, allowed := range credential.Permissions {
		if allowed == permission {
			return true
		}
	}
	return false
}
