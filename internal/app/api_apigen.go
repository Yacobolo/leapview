package app

import (
	"net/http"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apiprotocol "github.com/Yacobolo/leapview/internal/api/protocol"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
	"github.com/go-chi/chi/v5"
)

func (s *runtimeRouter) registerAPIGenRoutes(r chi.Router) {
	apigenapi.RegisterAPIGenRoutes(r, apiGenAdapter{server: s})
}

type apiGenAdapter struct {
	server *runtimeRouter
}

func (a apiGenAdapter) GetInstance(w http.ResponseWriter, _ *http.Request) {
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.InstanceResponse{Environment: a.server.defaultEnvironment})
}

func (a apiGenAdapter) HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request) {
	contract, ok := apigenapi.GetAPIGenOperationContract(operationID)
	if !ok || !contract.Protected {
		http.NotFound(w, r)
		return
	}
	var privilege accessmodule.Privilege
	if contract.AuthzMode == "privilege" {
		privilege, ok = apigenOperationPrivilege(operationID)
		if !ok {
			http.NotFound(w, r)
			return
		}
	} else if contract.AuthzMode != "authenticated" {
		http.NotFound(w, r)
		return
	}
	var objectResolver accessmodule.ObjectResolver
	if !isGlobalAgentOperation(operationID) {
		objectResolver, ok = apigenOperationObjectResolver(operationID)
		if !ok {
			http.NotFound(w, r)
			return
		}
	}
	protected := a.server.protectWithObjects
	if isGlobalAgentOperation(operationID) {
		protected = func(privilege accessmodule.Privilege, _ accessmodule.ObjectResolver, next http.Handler) http.Handler {
			return a.server.protectGlobalAgent(privilege, next)
		}
	}
	protected(privilege, objectResolver, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffered := apiprotocol.NewResponseBuffer(w, r)
		if ok := apigenapi.DispatchAPIGenOperation(operationID, a, apiprotocol.TransportErrorResponder{Logger: a.server.logger}, buffered, r); !ok {
			http.NotFound(w, r)
			return
		}
		buffered.Flush()
	})).ServeHTTP(w, r)
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

func apigenOperationPrivilege(operationID string) (accessmodule.Privilege, bool) {
	contract, ok := apigenapi.GetAPIGenOperationContract(operationID)
	if !ok || !contract.Protected || contract.AuthzMode != "privilege" {
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
	return accessmodule.ParsePrivilege(value)
}

func (a apiGenAdapter) GetCurrentPrincipal(w http.ResponseWriter, r *http.Request) {
	a.server.accessModule.HTTP().GetCurrentPrincipal(w, r)
}

func (a apiGenAdapter) ListCurrentEffectivePrivileges(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListCurrentEffectivePrivilegesParams) {
	a.server.accessModule.HTTP().ListCurrentEffectivePrivileges(w, r)
}

func (a apiGenAdapter) ListCurrentAPITokens(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListCurrentAPITokensParams) {
	a.server.accessModule.HTTP().ListCurrentAPITokens(w, r)
}

func (a apiGenAdapter) CreateCurrentAPIToken(w http.ResponseWriter, r *http.Request, _ apigenapi.GenCreateCurrentAPITokenHeaders) {
	a.server.accessModule.HTTP().CreateCurrentAPIToken(w, r)
}

func (a apiGenAdapter) RevokeCurrentAPIToken(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().RevokeCurrentAPIToken(w, r)
}

func (a apiGenAdapter) ListCurrentSessions(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListCurrentSessionsParams) {
	a.server.accessModule.HTTP().ListCurrentSessions(w, r)
}

func (a apiGenAdapter) GetActiveManagedDataRevision(w http.ResponseWriter, r *http.Request, project, connection string) {
	a.server.managedDataModule.HTTP().GetActiveManagedDataRevision(w, r, project, connection)
}

func (a apiGenAdapter) ListManagedDataRevisions(w http.ResponseWriter, r *http.Request, project, connection string, params apigenapi.GenListManagedDataRevisionsParams) {
	a.server.managedDataModule.HTTP().ListManagedDataRevisions(w, r, project, connection, params)
}

func (a apiGenAdapter) GetManagedDataRevision(w http.ResponseWriter, r *http.Request, project, connection, revision string) {
	a.server.managedDataModule.HTTP().GetManagedDataRevision(w, r, project, connection, revision)
}

func (a apiGenAdapter) CreateManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection string, headers apigenapi.GenCreateManagedDataUploadSessionHeaders) {
	a.server.managedDataModule.HTTP().CreateManagedDataUploadSession(w, r, project, connection, headers)
}

func (a apiGenAdapter) GetManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string) {
	a.server.managedDataModule.HTTP().GetManagedDataUploadSession(w, r, project, connection, uploadSession)
}

func (a apiGenAdapter) ListManagedDataUploadSessions(w http.ResponseWriter, r *http.Request, project, connection string, params apigenapi.GenListManagedDataUploadSessionsParams) {
	a.server.managedDataModule.HTTP().ListManagedDataUploadSessions(w, r, project, connection, params)
}

func (a apiGenAdapter) CancelManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, headers apigenapi.GenCancelManagedDataUploadSessionHeaders) {
	a.server.managedDataModule.HTTP().CancelManagedDataUploadSession(w, r, project, connection, uploadSession, headers)
}

func (a apiGenAdapter) FinalizeManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, headers apigenapi.GenFinalizeManagedDataUploadSessionHeaders) {
	a.server.managedDataModule.HTTP().FinalizeManagedDataUploadSession(w, r, project, connection, uploadSession, headers)
}

func (a apiGenAdapter) CreateManagedDataS3MultipartUpload(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, headers apigenapi.GenCreateManagedDataS3MultipartUploadHeaders) {
	a.server.managedDataModule.HTTP().CreateManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, headers)
}

func (a apiGenAdapter) AbortManagedDataS3MultipartUpload(w http.ResponseWriter, r *http.Request, project, connection, uploadSession, multipartUpload string, headers apigenapi.GenAbortManagedDataS3MultipartUploadHeaders) {
	a.server.managedDataModule.HTTP().AbortManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, multipartUpload, headers)
}

func (a apiGenAdapter) CompleteManagedDataS3MultipartUpload(w http.ResponseWriter, r *http.Request, project, connection, uploadSession, multipartUpload string, headers apigenapi.GenCompleteManagedDataS3MultipartUploadHeaders) {
	a.server.managedDataModule.HTTP().CompleteManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, multipartUpload, headers)
}

func (a apiGenAdapter) SignManagedDataS3MultipartPart(w http.ResponseWriter, r *http.Request, project, connection, uploadSession, multipartUpload string, partNumber int32, _ apigenapi.GenSignManagedDataS3MultipartPartHeaders) {
	a.server.managedDataModule.HTTP().SignManagedDataS3MultipartPart(w, r, project, connection, uploadSession, multipartUpload, partNumber)
}

func (a apiGenAdapter) RevokeCurrentSession(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().RevokeCurrentSession(w, r)
}

func (a apiGenAdapter) ListWorkspaces(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListWorkspacesParams) {
	a.server.workspaceModule.HTTP().Workspaces(w, r)
}

func (a apiGenAdapter) Search(w http.ResponseWriter, r *http.Request, params apigenapi.GenSearchParams) {
	a.server.workspaceModule.SearchAPI(w, r, params)
}

func (a apiGenAdapter) ListWorkspaceAssets(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListWorkspaceAssetsParams) {
	a.server.workspaceModule.HTTP().Assets(w, r)
}

func (a apiGenAdapter) GetWorkspaceActiveAssetGraph(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.workspaceModule.HTTP().ActiveDeploymentGraph(w, r)
}

func (a apiGenAdapter) GetWorkspaceAsset(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.workspaceModule.HTTP().Asset(w, r)
}

func (a apiGenAdapter) GetWorkspaceAssetLineage(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.workspaceModule.HTTP().AssetLineage(w, r)
}

func (a apiGenAdapter) ListWorkspaceAssetEdges(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListWorkspaceAssetEdgesParams) {
	a.server.workspaceModule.HTTP().AssetEdges(w, r)
}

func (a apiGenAdapter) ListDashboards(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListDashboardsParams) {
	a.server.dashboardModule.HTTP().ListDashboards(w, r)
}

func (a apiGenAdapter) GetDashboard(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.dashboardModule.HTTP().GetDashboard(w, r)
}

func (a apiGenAdapter) GetDashboardPage(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.server.dashboardModule.HTTP().GetDashboardPage(w, r)
}

func (a apiGenAdapter) GetDashboardFilter(w http.ResponseWriter, r *http.Request, _, _, _, _ string) {
	a.server.dashboardModule.HTTP().GetDashboardFilter(w, r)
}

func (a apiGenAdapter) GetDashboardVisual(w http.ResponseWriter, r *http.Request, _, _, _, _ string) {
	a.server.dashboardModule.HTTP().GetDashboardVisual(w, r)
}

func (a apiGenAdapter) ListSemanticDatasets(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticDatasetsParams) {
	a.server.dashboardModule.SemanticAPI().ListSemanticDatasets(w, r)
}

func (a apiGenAdapter) GetSemanticDataset(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.server.dashboardModule.SemanticAPI().GetSemanticDataset(w, r)
}

func (a apiGenAdapter) ListSemanticModelFields(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticModelFieldsParams) {
	a.server.dashboardModule.SemanticAPI().ListSemanticModelFields(w, r)
}

func (a apiGenAdapter) ListSemanticRelationships(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticRelationshipsParams) {
	a.server.dashboardModule.SemanticAPI().ListSemanticRelationships(w, r)
}

func (a apiGenAdapter) ListSemanticSources(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticSourcesParams) {
	a.server.dashboardModule.SemanticAPI().ListSemanticSources(w, r)
}

func (a apiGenAdapter) QuerySemanticModel(w http.ResponseWriter, r *http.Request, workspaceID, _ string, headers apigenapi.GenQuerySemanticModelHeaders) {
	a.server.dashboardModule.QuerySemanticModel(w, r, workspaceID, headers)
}

func (a apiGenAdapter) ExplainSemanticModelQuery(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.dashboardModule.SemanticAPI().ExplainSemanticModelQuery(w, r)
}

func (a apiGenAdapter) ListSemanticFields(w http.ResponseWriter, r *http.Request, _, _, _ string, _ apigenapi.GenListSemanticFieldsParams) {
	a.server.dashboardModule.SemanticAPI().ListSemanticFields(w, r)
}

func (a apiGenAdapter) PreviewSemanticDataset(w http.ResponseWriter, r *http.Request, workspaceID, _, _ string, headers apigenapi.GenPreviewSemanticDatasetHeaders) {
	a.server.dashboardModule.PreviewSemanticDataset(w, r, workspaceID, headers)
}

func (a apiGenAdapter) ExplainSemanticPreview(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.server.dashboardModule.SemanticAPI().ExplainSemanticPreview(w, r)
}

func (a apiGenAdapter) QueryDashboardPage(w http.ResponseWriter, r *http.Request, workspaceID, _, _ string) {
	a.server.dashboardModule.QueryDashboardPage(w, r, workspaceID)
}

func (a apiGenAdapter) QueryDashboardVisualData(w http.ResponseWriter, r *http.Request, workspaceID, _, _, _ string, headers apigenapi.GenQueryDashboardVisualDataHeaders) {
	a.server.dashboardModule.QueryDashboardVisualData(w, r, workspaceID, headers)
}

func (a apiGenAdapter) ListDashboardFilterValues(w http.ResponseWriter, r *http.Request, workspaceID, _, _, _ string, params apigenapi.GenListDashboardFilterValuesParams) {
	a.server.dashboardModule.ListDashboardFilterValues(w, r, workspaceID, params)
}

func (a apiGenAdapter) CreateRefreshRun(w http.ResponseWriter, r *http.Request, workspaceID string, headers apigenapi.GenCreateRefreshRunHeaders) {
	a.server.refreshModule.CreateRefreshRun(w, r, workspaceID, headers)
}

func (a apiGenAdapter) ListRefreshRuns(w http.ResponseWriter, r *http.Request, workspaceID string, params apigenapi.GenListRefreshRunsParams) {
	a.server.refreshModule.ListRefreshRuns(w, r, workspaceID, params)
}

func (a apiGenAdapter) GetRefreshRun(w http.ResponseWriter, r *http.Request, workspaceID, runID string) {
	a.server.refreshModule.GetRefreshRun(w, r, workspaceID, runID)
}

func (a apiGenAdapter) ListAgentConversations(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListAgentConversationsParams) {
	a.server.agentModule.HTTP().ListConversations(w, r)
}

func (a apiGenAdapter) CreateAgentConversation(w http.ResponseWriter, r *http.Request, _ apigenapi.GenCreateAgentConversationHeaders) {
	a.server.agentModule.HTTP().CreateConversation(w, r)
}

func (a apiGenAdapter) GetAgentConversation(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.agentModule.HTTP().GetConversation(w, r)
}

func (a apiGenAdapter) UpdateAgentConversation(w http.ResponseWriter, r *http.Request, _ string, headers apigenapi.GenUpdateAgentConversationHeaders) {
	a.server.agentModule.UpdateConversation(w, r, headers)
}

func (a apiGenAdapter) ArchiveAgentConversation(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.agentModule.HTTP().ArchiveConversation(w, r)
}

func (a apiGenAdapter) ListAgentMessages(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListAgentMessagesParams) {
	a.server.agentModule.HTTP().ListMessages(w, r)
}

func (a apiGenAdapter) CreateAgentRun(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenCreateAgentRunHeaders) {
	a.server.agentModule.HTTP().CreateRun(w, r)
}

func (a apiGenAdapter) ListAgentRuns(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListAgentRunsParams) {
	a.server.agentModule.HTTP().ListRuns(w, r)
}

func (a apiGenAdapter) GetAgentRun(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.agentModule.HTTP().GetRun(w, r)
}

func (a apiGenAdapter) ListAgentEvents(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListAgentEventsParams, _ apigenapi.GenListAgentEventsHeaders) {
	a.server.agentModule.HTTP().ListEvents(w, r)
}

func (a apiGenAdapter) CancelAgentRun(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenCancelAgentRunHeaders) {
	a.server.agentModule.HTTP().CancelRun(w, r)
}

func (a apiGenAdapter) ListPrincipals(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListPrincipalsParams) {
	a.server.accessModule.HTTP().ListPrincipals(w, r)
}

func (a apiGenAdapter) CreatePrincipal(w http.ResponseWriter, r *http.Request, _ apigenapi.GenCreatePrincipalHeaders) {
	a.server.accessModule.HTTP().CreatePrincipal(w, r)
}

func (a apiGenAdapter) GetPrincipal(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().GetPrincipal(w, r)
}

func (a apiGenAdapter) UpdatePrincipal(w http.ResponseWriter, r *http.Request, _ string, headers apigenapi.GenUpdatePrincipalHeaders) {
	a.server.accessModule.UpdatePrincipal(w, r, headers)
}

func (a apiGenAdapter) DeletePrincipal(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().DeletePrincipal(w, r)
}

func (a apiGenAdapter) ResetPrincipalPassword(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenResetPrincipalPasswordHeaders) {
	a.server.accessModule.HTTP().ResetPrincipalPassword(w, r)
}

func (a apiGenAdapter) ListServicePrincipals(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListServicePrincipalsParams) {
	a.server.accessModule.HTTP().ListServicePrincipals(w, r)
}

func (a apiGenAdapter) CreateServicePrincipal(w http.ResponseWriter, r *http.Request, _ apigenapi.GenCreateServicePrincipalHeaders) {
	a.server.accessModule.HTTP().CreateServicePrincipal(w, r)
}

func (a apiGenAdapter) GetServicePrincipal(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().GetServicePrincipal(w, r)
}

func (a apiGenAdapter) UpdateServicePrincipal(w http.ResponseWriter, r *http.Request, _ string, headers apigenapi.GenUpdateServicePrincipalHeaders) {
	a.server.accessModule.UpdateServicePrincipal(w, r, headers)
}

func (a apiGenAdapter) DeleteServicePrincipal(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().DeleteServicePrincipal(w, r)
}

func (a apiGenAdapter) CreateServicePrincipalSecret(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenCreateServicePrincipalSecretHeaders) {
	a.server.accessModule.HTTP().CreateServicePrincipalSecret(w, r)
}

func (a apiGenAdapter) ListServicePrincipalSecrets(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListServicePrincipalSecretsParams) {
	a.server.accessModule.HTTP().ListServicePrincipalSecrets(w, r)
}

func (a apiGenAdapter) GetServicePrincipalSecret(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().GetServicePrincipalSecret(w, r)
}

func (a apiGenAdapter) RevokeServicePrincipalSecret(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().RevokeServicePrincipalSecret(w, r)
}

func (a apiGenAdapter) ListWorkspaceRoles(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListWorkspaceRolesParams) {
	a.server.accessModule.HTTP().ListWorkspaceRoles(w, r)
}

func (a apiGenAdapter) ListEffectivePrivileges(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListEffectivePrivilegesParams) {
	a.server.accessModule.HTTP().ListEffectivePrivileges(w, r)
}

func (a apiGenAdapter) ListGrants(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListGrantsParams) {
	a.server.accessModule.HTTP().ListGrants(w, r)
}

func (a apiGenAdapter) CreateGrant(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenCreateGrantHeaders) {
	a.server.accessModule.HTTP().CreateGrant(w, r)
}

func (a apiGenAdapter) GetGrant(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().GetGrant(w, r)
}

func (a apiGenAdapter) UpdateGrant(w http.ResponseWriter, r *http.Request, _, _ string, headers apigenapi.GenUpdateGrantHeaders) {
	a.server.accessModule.UpdateGrant(w, r, headers)
}

func (a apiGenAdapter) DeleteGrant(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().DeleteGrant(w, r)
}

func (a apiGenAdapter) ListDataPolicies(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListDataPoliciesParams) {
	a.server.accessModule.HTTP().ListDataPolicies(w, r)
}

func (a apiGenAdapter) CreateDataPolicy(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenCreateDataPolicyHeaders) {
	a.server.accessModule.HTTP().CreateDataPolicy(w, r)
}

func (a apiGenAdapter) GetDataPolicy(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().GetDataPolicy(w, r)
}

func (a apiGenAdapter) UpdateDataPolicy(w http.ResponseWriter, r *http.Request, _, _ string, headers apigenapi.GenUpdateDataPolicyHeaders) {
	a.server.accessModule.UpdateDataPolicy(w, r, headers)
}

func (a apiGenAdapter) CheckAuthorizationBatch(w http.ResponseWriter, r *http.Request, _ string) {
	a.server.accessModule.HTTP().CheckAuthorizationBatch(w, r)
}

func (a apiGenAdapter) DeleteDataPolicy(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().DeleteDataPolicy(w, r)
}

func (a apiGenAdapter) TransferOwnership(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenTransferOwnershipHeaders) {
	a.server.accessModule.HTTP().TransferOwnership(w, r)
}

func (a apiGenAdapter) ListSemanticModels(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListSemanticModelsParams) {
	a.server.dashboardModule.SemanticAPI().ListSemanticModels(w, r)
}

func (a apiGenAdapter) GetSemanticModel(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.dashboardModule.SemanticAPI().GetSemanticModel(w, r)
}

func (a apiGenAdapter) ListGroups(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListGroupsParams) {
	a.server.accessModule.HTTP().ListGroups(w, r)
}

func (a apiGenAdapter) CreateGroup(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenCreateGroupHeaders) {
	a.server.accessModule.HTTP().CreateGroup(w, r)
}

func (a apiGenAdapter) GetGroup(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().GetGroup(w, r)
}

func (a apiGenAdapter) UpdateGroup(w http.ResponseWriter, r *http.Request, _, _ string, headers apigenapi.GenUpdateGroupHeaders) {
	a.server.accessModule.UpdateGroup(w, r, headers)
}

func (a apiGenAdapter) DeleteGroup(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().DeleteGroup(w, r)
}

func (a apiGenAdapter) ListGroupMembers(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListGroupMembersParams) {
	a.server.accessModule.HTTP().ListGroupMembers(w, r)
}

func (a apiGenAdapter) AddGroupMember(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.server.accessModule.HTTP().AddGroupMember(w, r)
}

func (a apiGenAdapter) RemoveGroupMember(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.server.accessModule.HTTP().RemoveGroupMember(w, r)
}

func (a apiGenAdapter) ListRoleBindings(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListRoleBindingsParams) {
	a.server.accessModule.HTTP().ListRoleBindings(w, r)
}

func (a apiGenAdapter) CreateRoleBinding(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenCreateRoleBindingHeaders) {
	a.server.accessModule.HTTP().CreateRoleBinding(w, r)
}

func (a apiGenAdapter) GetRoleBinding(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().GetRoleBinding(w, r)
}

func (a apiGenAdapter) UpdateRoleBinding(w http.ResponseWriter, r *http.Request, _, _ string, headers apigenapi.GenUpdateRoleBindingHeaders) {
	a.server.accessModule.UpdateRoleBinding(w, r, headers)
}

func (a apiGenAdapter) DeleteRoleBinding(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.server.accessModule.HTTP().DeleteRoleBinding(w, r)
}

func (a apiGenAdapter) ListAuditEvents(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListAuditEventsParams) {
	a.server.accessModule.HTTP().ListAuditEvents(w, r)
}

func (a apiGenAdapter) ListQueryEvents(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListQueryEventsParams) {
	a.server.queryAuditEvents(w, r)
}
