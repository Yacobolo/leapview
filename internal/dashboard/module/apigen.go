package module

import (
	"log/slog"
	"net/http"

	dashboardhttp "github.com/Yacobolo/leapview/internal/dashboard/http"
)

type dashboardAPIGenHandler struct{ module *Module }

func (h dashboardAPIGenHandler) ListDashboardPublications(w http.ResponseWriter, r *http.Request, workspace string) {
	h.module.ListDashboardPublications(w, r, workspace)
}
func (h dashboardAPIGenHandler) GetDashboardPublication(w http.ResponseWriter, r *http.Request, workspace, publication string) {
	h.module.GetDashboardPublication(w, r, workspace, publication)
}
func (h dashboardAPIGenHandler) ResumeDashboardPublication(w http.ResponseWriter, r *http.Request, workspace, publication string) {
	h.module.ResumeDashboardPublication(w, r, workspace, publication)
}
func (h dashboardAPIGenHandler) RotateDashboardPublication(w http.ResponseWriter, r *http.Request, workspace, publication string) {
	h.module.RotateDashboardPublication(w, r, workspace, publication)
}
func (h dashboardAPIGenHandler) SuspendDashboardPublication(w http.ResponseWriter, r *http.Request, workspace, publication string) {
	h.module.SuspendDashboardPublication(w, r, workspace, publication)
}
func (h dashboardAPIGenHandler) ListDashboards(w http.ResponseWriter, r *http.Request) {
	h.module.HTTP().ListDashboards(w, r)
}
func (h dashboardAPIGenHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	h.module.HTTP().GetDashboard(w, r)
}
func (h dashboardAPIGenHandler) GetDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.module.HTTP().GetDashboardPage(w, r)
}
func (h dashboardAPIGenHandler) GetDashboardFilter(w http.ResponseWriter, r *http.Request) {
	h.module.HTTP().GetDashboardFilter(w, r)
}
func (h dashboardAPIGenHandler) ListDashboardFilterValues(w http.ResponseWriter, r *http.Request, workspace string) {
	h.module.ListDashboardFilterValues(w, r, workspace)
}
func (h dashboardAPIGenHandler) QueryDashboardPage(w http.ResponseWriter, r *http.Request, workspace string) {
	h.module.QueryDashboardPage(w, r, workspace)
}
func (h dashboardAPIGenHandler) GetDashboardVisual(w http.ResponseWriter, r *http.Request) {
	h.module.HTTP().GetDashboardVisual(w, r)
}
func (h dashboardAPIGenHandler) QueryDashboardVisualData(w http.ResponseWriter, r *http.Request, workspace string) {
	h.module.QueryDashboardVisualData(w, r, workspace)
}
func (h dashboardAPIGenHandler) ListSemanticModels(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ListSemanticModels(w, r)
}
func (h dashboardAPIGenHandler) GetSemanticModel(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().GetSemanticModel(w, r)
}
func (h dashboardAPIGenHandler) ListSemanticDatasets(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ListSemanticDatasets(w, r)
}
func (h dashboardAPIGenHandler) GetSemanticDataset(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().GetSemanticDataset(w, r)
}
func (h dashboardAPIGenHandler) ListSemanticFields(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ListSemanticFields(w, r)
}
func (h dashboardAPIGenHandler) PreviewSemanticDataset(w http.ResponseWriter, r *http.Request, workspace string) {
	h.module.PreviewSemanticDataset(w, r, workspace)
}
func (h dashboardAPIGenHandler) ExplainSemanticPreview(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ExplainSemanticPreview(w, r)
}
func (h dashboardAPIGenHandler) ListSemanticModelFields(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ListSemanticModelFields(w, r)
}
func (h dashboardAPIGenHandler) QuerySemanticModel(w http.ResponseWriter, r *http.Request, workspace string) {
	h.module.QuerySemanticModel(w, r, workspace)
}
func (h dashboardAPIGenHandler) ExplainSemanticModelQuery(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ExplainSemanticModelQuery(w, r)
}
func (h dashboardAPIGenHandler) ListSemanticRelationships(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ListSemanticRelationships(w, r)
}
func (h dashboardAPIGenHandler) ListSemanticSources(w http.ResponseWriter, r *http.Request) {
	h.module.SemanticAPI().ListSemanticSources(w, r)
}

func (m *Module) DispatchAPIGenOperation(operationID string, logger *slog.Logger, w http.ResponseWriter, r *http.Request) bool {
	return dashboardhttp.DispatchAPIGenOperation(operationID, dashboardAPIGenHandler{module: m}, logger, w, r)
}
