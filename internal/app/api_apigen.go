package app

import (
	"net/http"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	apiaggregate "github.com/Yacobolo/leapview/internal/app/api/aggregate"
	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
	"github.com/Yacobolo/leapview/internal/platform/buildinfo"
	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
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
	managedDataModule  *manageddatamodule.Module
	releaseModule      *releasemodule.Module
	defaultEnvironment string
	buildIdentity      buildinfo.Identity
	managedDataTus     http.Handler
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

func (a apiGenDispatcher) ListSemanticModels(w http.ResponseWriter, r *http.Request, _ string, _ apigenapi.GenListSemanticModelsParams) {
	a.dashboardModule.SemanticAPI().ListSemanticModels(w, r)
}

func (a apiGenDispatcher) GetSemanticModel(w http.ResponseWriter, r *http.Request, _, _ string) {
	a.dashboardModule.SemanticAPI().GetSemanticModel(w, r)
}
