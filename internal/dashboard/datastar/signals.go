package datastar

import visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"

// VisualizationSignal is the dashboard-owned browser transport projection for
// a visualization envelope.
type VisualizationSignal struct {
	SchemaVersion    int32                                               `json:"schemaVersion"`
	VisualID         string                                              `json:"visualID"`
	RendererID       string                                              `json:"rendererID"`
	SpecRevision     string                                              `json:"specRevision"`
	Spec             visualizationir.VisualizationSpec                   `json:"spec"`
	DataRevision     int64                                               `json:"dataRevision"`
	DataState        visualizationir.VisualizationDataStateTransport     `json:"dataState"`
	Selection        []visualizationir.VisualizationSelectionEntry       `json:"selection"`
	SpatialSelection *visualizationir.VisualizationSpatialSelectionState `json:"spatialSelection,omitempty"`
	Status           visualizationir.VisualizationStatus                 `json:"status"`
	Diagnostics      []visualizationir.VisualizationDiagnostic           `json:"diagnostics"`
}
