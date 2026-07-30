package ir

func testVisualizationPresentation(legend VisualizationLegendPosition) VisualizationPresentation {
	return VisualizationPresentation{
		Legend: legend,
		LabelPolicy: VisualizationLabelPolicy{
			Density: VisualizationLabelDensityHidden, Priority: []VisualizationLabelPriority{},
			MaxCharacters: 24, MinimumSpacing: 0, TooltipFallback: true,
		},
	}
}
