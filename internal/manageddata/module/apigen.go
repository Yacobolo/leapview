package module

import (
	"log/slog"
	"net/http"

	manageddataapi "github.com/flidai/leapview/internal/manageddata/api"
	manageddatahttp "github.com/flidai/leapview/internal/manageddata/http"
	apitransport "github.com/flidai/leapview/internal/platform/http/transport"
)

type ManagedConnectionCatalog interface {
	ListManagedConnections(http.ResponseWriter, *http.Request, string, *int32, *string)
	GetManagedConnection(http.ResponseWriter, *http.Request, string, string)
}

type managedDataAPIGenHandler struct {
	*manageddatahttp.Handler
	module  *Module
	catalog ManagedConnectionCatalog
}

func (h managedDataAPIGenHandler) ListManagedConnections(w http.ResponseWriter, r *http.Request, project string, params manageddataapi.PageParams) {
	if h.catalog == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "MANAGED_CONNECTION_CATALOG_UNAVAILABLE", "Managed connection catalog is unavailable", nil)
		return
	}
	h.catalog.ListManagedConnections(w, r, project, params.Limit, params.PageToken)
}

func (h managedDataAPIGenHandler) GetManagedConnection(w http.ResponseWriter, r *http.Request, project, connection string) {
	if h.catalog == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "MANAGED_CONNECTION_CATALOG_UNAVAILABLE", "Managed connection catalog is unavailable", nil)
		return
	}
	h.catalog.GetManagedConnection(w, r, project, connection)
}

func (h managedDataAPIGenHandler) ListManagedDataUploadSessionEvents(w http.ResponseWriter, r *http.Request, project, connection, uploadSession string, params manageddataapi.PageParams, headers manageddataapi.GenListManagedDataUploadSessionEventsHeaders) {
	h.module.ListUploadSessionEvents(w, r, project, connection, uploadSession, params, headers)
}

func (m *Module) DispatchAPIGenOperation(operationID string, catalog ManagedConnectionCatalog, logger *slog.Logger, w http.ResponseWriter, r *http.Request) bool {
	return manageddatahttp.DispatchAPIGenOperation(operationID, managedDataAPIGenHandler{
		Handler: m.HTTP(), module: m, catalog: catalog,
	}, logger, w, r)
}
