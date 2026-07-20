package ir

// GetKind returns the closed geographic layer discriminator.
func (value VisualizationGeographicLayer) GetKind() string {
	switch layer := value.Value.(type) {
	case *VisualizationPointLayer:
		return layer.Kind
	case *VisualizationChoroplethLayer:
		return layer.Kind
	case *VisualizationHeatLayer:
		return layer.Kind
	case *VisualizationDensityLayer:
		return layer.Kind
	case *VisualizationReferenceLayer:
		return layer.Kind
	case *VisualizationPathLayer:
		return layer.Kind
	default:
		return ""
	}
}

func geographicLayerBase(value VisualizationGeographicLayer) *VisualizationGeographicLayerBase {
	switch layer := value.Value.(type) {
	case *VisualizationPointLayer:
		return &layer.VisualizationGeographicLayerBase
	case *VisualizationChoroplethLayer:
		return &layer.VisualizationGeographicLayerBase
	case *VisualizationHeatLayer:
		return &layer.VisualizationGeographicLayerBase
	case *VisualizationDensityLayer:
		return &layer.VisualizationGeographicLayerBase
	case *VisualizationReferenceLayer:
		return &layer.VisualizationGeographicLayerBase
	case *VisualizationPathLayer:
		return &layer.VisualizationGeographicLayerBase
	default:
		return nil
	}
}
