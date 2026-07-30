package compiler

import (
	"reflect"
	"testing"

	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

func TestCompiledLabelPolicyPreservesLegacyAndAuthoredIntent(t *testing.T) {
	t.Parallel()

	hidden := compiledLabelPolicy(reportdef.VisualPresentation{}, "line")
	if hidden.Density != visualizationir.VisualizationLabelDensityHidden || len(hidden.Priority) != 0 || !hidden.TooltipFallback {
		t.Fatalf("default policy = %#v, want hidden with tooltip fallback", hidden)
	}

	gauge := compiledLabelPolicy(reportdef.VisualPresentation{}, "gauge")
	if gauge.Density != visualizationir.VisualizationLabelDensityAutomatic || !gauge.TooltipFallback {
		t.Fatalf("gauge default policy = %#v, want automatic with tooltip fallback", gauge)
	}

	legacy := compiledLabelPolicy(reportdef.VisualPresentation{ShowLabels: true}, "line")
	if legacy.Density != visualizationir.VisualizationLabelDensityAutomatic {
		t.Fatalf("show_labels policy = %#v, want automatic collision management", legacy)
	}

	maxCharacters, minimumSpacing, tooltipFallback := 36, 3, true
	authored := compiledLabelPolicy(reportdef.VisualPresentation{Labels: reportdef.VisualLabelPolicy{
		Density:         "dense",
		Priority:        []string{"threshold", "selected"},
		MaxCharacters:   &maxCharacters,
		MinimumSpacing:  &minimumSpacing,
		TooltipFallback: &tooltipFallback,
	}}, "line")
	if authored.Density != visualizationir.VisualizationLabelDensityDense ||
		authored.MaxCharacters != 36 || authored.MinimumSpacing != 3 || !authored.TooltipFallback {
		t.Fatalf("authored policy = %#v", authored)
	}
	wantPriority := []visualizationir.VisualizationLabelPriority{
		visualizationir.VisualizationLabelPriorityThreshold,
		visualizationir.VisualizationLabelPrioritySelected,
	}
	if !reflect.DeepEqual(authored.Priority, wantPriority) {
		t.Fatalf("priority = %#v, want %#v", authored.Priority, wantPriority)
	}
}
