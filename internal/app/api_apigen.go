package app

import (
	"net/http"

	"github.com/Yacobolo/libredash/internal/access"
	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
	"github.com/go-chi/chi/v5"
)

var apigenOperationPermissions = map[string]string{
	"listWorkspaces":           access.PermissionDashboardView,
	"listWorkspaceAssets":      access.PermissionDashboardView,
	"listWorkspaceAssetEdges":  access.PermissionDashboardView,
	"createAgentConversation":  access.PermissionDashboardView,
	"listAgentConversations":   access.PermissionDashboardView,
	"listAgentMessages":        access.PermissionDashboardView,
	"createAgentTurn":          access.PermissionDashboardView,
	"listAgentEvents":          access.PermissionDashboardView,
	"listWorkspaceRoles":       access.PermissionRBACManage,
	"listRoleBindings":         access.PermissionRBACManage,
	"upsertRoleBinding":        access.PermissionRBACManage,
	"deleteRoleBinding":        access.PermissionRBACManage,
	"createDeployment":         access.PermissionDeploymentCreate,
	"listDeployments":          access.PermissionDeploymentCreate,
	"getDeployment":            access.PermissionDeploymentCreate,
	"uploadDeploymentArtifact": access.PermissionDeploymentCreate,
	"validateDeployment":       access.PermissionDeploymentCreate,
	"activateDeployment":       access.PermissionDeploymentActivate,
	"rollbackDeployment":       access.PermissionDeploymentRollback,
}

func (s *Server) registerAPIGenRoutes(r chi.Router) {
	apigenapi.RegisterAPIGenRoutes(r, apiGenAdapter{server: s})
}

type apiGenAdapter struct {
	server *Server
}

func (a apiGenAdapter) HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request) {
	permission, ok := apigenOperationPermissions[operationID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.server.protect(permission, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok := apigenapi.DispatchAPIGenOperation(operationID, a, w, r); !ok {
			http.NotFound(w, r)
		}
	})).ServeHTTP(w, r)
}

func (a apiGenAdapter) ListDeployments(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListDeploymentsParams) {
	a.server.listDeployments(w, r)
}

func (a apiGenAdapter) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	a.server.createDeployment(w, r)
}

func (a apiGenAdapter) GetDeployment(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.getDeployment(w, r)
}

func (a apiGenAdapter) UploadDeploymentArtifact(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.uploadDeploymentArtifact(w, r)
}

func (a apiGenAdapter) ActivateDeployment(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.activateDeployment(w, r)
}

func (a apiGenAdapter) RollbackDeployment(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.rollbackDeployment(w, r)
}

func (a apiGenAdapter) ValidateDeployment(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.validateDeployment(w, r)
}

func (a apiGenAdapter) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	a.server.apiWorkspaces(w, r)
}

func (a apiGenAdapter) ListAgentConversations(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.listAgentConversations(w, r)
}

func (a apiGenAdapter) CreateAgentConversation(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.createAgentConversation(w, r)
}

func (a apiGenAdapter) ListAgentMessages(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.listAgentMessages(w, r)
}

func (a apiGenAdapter) CreateAgentTurn(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.createAgentTurn(w, r)
}

func (a apiGenAdapter) ListAgentEvents(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.listAgentEvents(w, r)
}

func (a apiGenAdapter) ListWorkspaceAssetEdges(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.apiWorkspaceAssetEdges(w, r)
}

func (a apiGenAdapter) ListWorkspaceAssets(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListWorkspaceAssetsParams) {
	a.server.apiWorkspaceAssets(w, r)
}

func (a apiGenAdapter) ListRoleBindings(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.apiRoleBindings(w, r)
}

func (a apiGenAdapter) UpsertRoleBinding(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.apiUpsertRoleBinding(w, r)
}

func (a apiGenAdapter) DeleteRoleBinding(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.apiDeleteRoleBinding(w, r)
}

func (a apiGenAdapter) ListWorkspaceRoles(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.apiWorkspaceRoles(w, r)
}
