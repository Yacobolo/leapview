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
	defaultEnvironment string
	buildIdentity      buildinfo.Identity
	managedDataTus     http.Handler
}

func (a apiGenDispatcher) GetInstance(w http.ResponseWriter, _ *http.Request) {
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.InstanceResponse{Environment: a.defaultEnvironment})
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
