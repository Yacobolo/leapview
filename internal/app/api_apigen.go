package app

import (
	"net/http"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	apiaggregate "github.com/Yacobolo/leapview/internal/app/api/aggregate"
	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
	"github.com/Yacobolo/leapview/internal/platform/buildinfo"
	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
	"github.com/go-chi/chi/v5"
)

func agentAPIGenOperations() []agentmodule.APIGenOperation {
	generated := apiaggregate.GetAPIGenOperationContracts()
	contracts := make(map[string]agentmodule.APIGenOperationContract, len(generated))
	for operationID, contract := range generated {
		contracts[operationID] = agentmodule.APIGenOperationContract{
			OperationID: contract.OperationID, Method: contract.Method, Path: contract.Path,
			Protected: contract.Protected, AuthzMode: contract.AuthzMode, Manual: contract.Manual,
			Extensions: contract.Extensions,
		}
	}
	return agentmodule.BuildAPIGenOperations(contracts, apiaggregate.GetAPIGenToolContracts())
}

func accessAPIGenOperationContracts() map[string]accessmodule.APIGenOperationContract {
	generated := apiaggregate.GetAPIGenOperationContracts()
	contracts := make(map[string]accessmodule.APIGenOperationContract, len(generated))
	for operationID, contract := range generated {
		contracts[operationID] = accessmodule.APIGenOperationContract{
			OperationID: contract.OperationID, Path: contract.Path, Protected: contract.Protected,
			AuthzMode: contract.AuthzMode, Extensions: contract.Extensions,
		}
	}
	return contracts
}

func registerAPIGenRoutes(routes *capabilityRoutes, runtime *runtimeServices, platform *platformServices, policy *httpPolicy, r chi.Router) {
	apiaggregate.RegisterAPIGenRoutes(r, platform.apiGenServers)
}

type apiGenDispatcher struct {
	dashboardModule    *dashboardmodule.Module
	deploymentModule   *deploymentmodule.Module
	managedDataModule  *manageddatamodule.Module
	refreshModule      *refreshmodule.Module
	releaseModule      *releasemodule.Module
	workspaceModule    *workspacemodule.Module
	defaultEnvironment string
	buildIdentity      buildinfo.Identity
	managedDataTus     http.Handler
	queryAuditEvents   http.HandlerFunc
}

func (a apiGenDispatcher) GetInstance(w http.ResponseWriter, _ *http.Request) {
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.InstanceResponse{Environment: a.defaultEnvironment})
}

func (a apiGenDispatcher) GetActiveManagedDataRevision(w http.ResponseWriter, r *http.Request, project, connection string) {
	a.managedDataModule.HTTP().GetActiveManagedDataRevision(w, r, project, connection)
}

func (a apiGenDispatcher) ListManagedDataRevisions(w http.ResponseWriter, r *http.Request, project, connection string, params apigenapi.GenListManagedDataRevisionsParams) {
	a.managedDataModule.HTTP().ListManagedDataRevisions(w, r, project, connection, manageddatamodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) GetManagedDataRevision(w http.ResponseWriter, r *http.Request, project, connection, revision string) {
	a.managedDataModule.HTTP().GetManagedDataRevision(w, r, project, connection, revision)
}

func (a apiGenDispatcher) CreateManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection string, headers apigenapi.GenCreateManagedDataUploadSessionHeaders) {
	a.managedDataModule.HTTP().CreateManagedDataUploadSession(w, r, project, connection, manageddatamodule.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (a apiGenDispatcher) GetManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string) {
	a.managedDataModule.HTTP().GetManagedDataUploadSession(w, r, project, connection, uploadSession)
}

func (a apiGenDispatcher) ListManagedDataUploadSessions(w http.ResponseWriter, r *http.Request, project, connection string, params apigenapi.GenListManagedDataUploadSessionsParams) {
	a.managedDataModule.HTTP().ListManagedDataUploadSessions(w, r, project, connection, manageddatamodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) CancelManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, headers apigenapi.GenCancelManagedDataUploadSessionHeaders) {
	a.managedDataModule.HTTP().CancelManagedDataUploadSession(w, r, project, connection, uploadSession, manageddatamodule.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (a apiGenDispatcher) FinalizeManagedDataUploadSession(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, headers apigenapi.GenFinalizeManagedDataUploadSessionHeaders) {
	a.managedDataModule.HTTP().FinalizeManagedDataUploadSession(w, r, project, connection, uploadSession, manageddatamodule.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (a apiGenDispatcher) CreateManagedDataS3MultipartUpload(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, headers apigenapi.GenCreateManagedDataS3MultipartUploadHeaders) {
	a.managedDataModule.HTTP().CreateManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, manageddatamodule.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (a apiGenDispatcher) AbortManagedDataS3MultipartUpload(w http.ResponseWriter, r *http.Request, project, connection, uploadSession, multipartUpload string, headers apigenapi.GenAbortManagedDataS3MultipartUploadHeaders) {
	a.managedDataModule.HTTP().AbortManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, multipartUpload, manageddatamodule.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (a apiGenDispatcher) CompleteManagedDataS3MultipartUpload(w http.ResponseWriter, r *http.Request, project, connection, uploadSession, multipartUpload string, headers apigenapi.GenCompleteManagedDataS3MultipartUploadHeaders) {
	a.managedDataModule.HTTP().CompleteManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, multipartUpload, manageddatamodule.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (a apiGenDispatcher) SignManagedDataS3MultipartPart(w http.ResponseWriter, r *http.Request, project, connection, uploadSession, multipartUpload string, partNumber int32, _ apigenapi.GenSignManagedDataS3MultipartPartHeaders) {
	a.managedDataModule.HTTP().SignManagedDataS3MultipartPart(w, r, project, connection, uploadSession, multipartUpload, partNumber)
}

func (a apiGenDispatcher) ListWorkspaces(w http.ResponseWriter, r *http.Request, _ apigenapi.GenListWorkspacesParams) {
	a.workspaceModule.HTTP().Workspaces(w, r)
}

func (a apiGenDispatcher) Search(w http.ResponseWriter, r *http.Request, params apigenapi.GenSearchParams) {
	var types *[]string
	if params.Type != nil {
		values := make([]string, len(*params.Type))
		for i, value := range *params.Type {
			values[i] = string(value)
		}
		types = &values
	}
	a.workspaceModule.SearchAPI(w, r, workspacemodule.SearchParams{
		Query: params.Q, Workspaces: params.Workspace, Types: types,
		ContextWorkspace: params.ContextWorkspace, ContextDashboard: params.ContextDashboard,
		ContextPage: params.ContextPage, Limit: params.Limit, PageToken: params.PageToken,
	})
}

func (a apiGenDispatcher) ListWorkspaceAssets(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListWorkspaceAssetsParams) {
	a.workspaceModule.HTTP().Assets(w, r)
}

func (a apiGenDispatcher) GetWorkspaceActiveAssetGraph(w http.ResponseWriter, r *http.Request, _ string) {
	a.workspaceModule.HTTP().ActiveDeploymentGraph(w, r)
}

func (a apiGenDispatcher) GetWorkspaceAsset(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.workspaceModule.HTTP().Asset(w, r)
}

func (a apiGenDispatcher) GetWorkspaceAssetLineage(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.workspaceModule.HTTP().AssetLineage(w, r)
}

func (a apiGenDispatcher) ListWorkspaceAssetEdges(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListWorkspaceAssetEdgesParams) {
	a.workspaceModule.HTTP().AssetEdges(w, r)
}

func (a apiGenDispatcher) ListDashboards(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListDashboardsParams) {
	a.dashboardModule.HTTP().ListDashboards(w, r)
}

func (a apiGenDispatcher) GetDashboard(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.dashboardModule.HTTP().GetDashboard(w, r)
}

func (a apiGenDispatcher) GetDashboardPage(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.dashboardModule.HTTP().GetDashboardPage(w, r)
}

func (a apiGenDispatcher) GetDashboardFilter(w http.ResponseWriter, r *http.Request, _, _, _, _ string) {
	a.dashboardModule.HTTP().GetDashboardFilter(w, r)
}

func (a apiGenDispatcher) GetDashboardVisual(w http.ResponseWriter, r *http.Request, _, _, _, _ string) {
	a.dashboardModule.HTTP().GetDashboardVisual(w, r)
}

func (a apiGenDispatcher) ListSemanticDatasets(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticDatasetsParams) {
	a.dashboardModule.SemanticAPI().ListSemanticDatasets(w, r)
}

func (a apiGenDispatcher) GetSemanticDataset(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.dashboardModule.SemanticAPI().GetSemanticDataset(w, r)
}

func (a apiGenDispatcher) ListSemanticModelFields(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticModelFieldsParams) {
	a.dashboardModule.SemanticAPI().ListSemanticModelFields(w, r)
}

func (a apiGenDispatcher) ListSemanticRelationships(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticRelationshipsParams) {
	a.dashboardModule.SemanticAPI().ListSemanticRelationships(w, r)
}

func (a apiGenDispatcher) ListSemanticSources(w http.ResponseWriter, r *http.Request, _, _ string, _ apigenapi.GenListSemanticSourcesParams) {
	a.dashboardModule.SemanticAPI().ListSemanticSources(w, r)
}

func (a apiGenDispatcher) QuerySemanticModel(w http.ResponseWriter, r *http.Request, workspaceID, _ string, headers apigenapi.GenQuerySemanticModelHeaders) {
	a.dashboardModule.QuerySemanticModel(w, r, workspaceID)
}

func (a apiGenDispatcher) ExplainSemanticModelQuery(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.dashboardModule.SemanticAPI().ExplainSemanticModelQuery(w, r)
}

func (a apiGenDispatcher) ListSemanticFields(w http.ResponseWriter, r *http.Request, _, _, _ string, _ apigenapi.GenListSemanticFieldsParams) {
	a.dashboardModule.SemanticAPI().ListSemanticFields(w, r)
}

func (a apiGenDispatcher) PreviewSemanticDataset(w http.ResponseWriter, r *http.Request, workspaceID, _, _ string, headers apigenapi.GenPreviewSemanticDatasetHeaders) {
	a.dashboardModule.PreviewSemanticDataset(w, r, workspaceID)
}

func (a apiGenDispatcher) ExplainSemanticPreview(w http.ResponseWriter, r *http.Request, _, _, _ string) {
	a.dashboardModule.SemanticAPI().ExplainSemanticPreview(w, r)
}

func (a apiGenDispatcher) QueryDashboardPage(w http.ResponseWriter, r *http.Request, workspaceID, _, _ string) {
	a.dashboardModule.QueryDashboardPage(w, r, workspaceID)
}

func (a apiGenDispatcher) QueryDashboardVisualData(w http.ResponseWriter, r *http.Request, workspaceID, _, _, _ string, headers apigenapi.GenQueryDashboardVisualDataHeaders) {
	a.dashboardModule.QueryDashboardVisualData(w, r, workspaceID)
}

func (a apiGenDispatcher) ListDashboardFilterValues(w http.ResponseWriter, r *http.Request, workspaceID, _, _, _ string, params apigenapi.GenListDashboardFilterValuesParams) {
	a.dashboardModule.ListDashboardFilterValues(w, r, workspaceID)
}

func (a apiGenDispatcher) CreateRefreshRun(w http.ResponseWriter, r *http.Request, workspaceID string, headers apigenapi.GenCreateRefreshRunHeaders) {
	a.refreshModule.CreateRefreshRun(w, r, workspaceID)
}

func (a apiGenDispatcher) ListRefreshRuns(w http.ResponseWriter, r *http.Request, workspaceID string, params apigenapi.GenListRefreshRunsParams) {
	a.refreshModule.ListRefreshRuns(w, r, workspaceID)
}

func (a apiGenDispatcher) GetRefreshRun(w http.ResponseWriter, r *http.Request, workspaceID, runID string) {
	a.refreshModule.GetRefreshRun(w, r, workspaceID, runID)
}

func (a apiGenDispatcher) ListSemanticModels(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListSemanticModelsParams) {
	a.dashboardModule.SemanticAPI().ListSemanticModels(w, r)
}

func (a apiGenDispatcher) GetSemanticModel(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.dashboardModule.SemanticAPI().GetSemanticModel(w, r)
}

func (a apiGenDispatcher) ListQueryEvents(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListQueryEventsParams) {
	a.queryAuditEvents(w, r)
}
