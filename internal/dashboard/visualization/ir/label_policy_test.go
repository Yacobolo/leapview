package ir

import (
	"strings"
	"testing"
)

func TestLabelPolicyValidationIsClosedAndDeterministic(t *testing.T) {
	t.Parallel()

	valid := func() VisualizationSpec {
		envelope := pointEnvelope(t, [][]any{{"o-1", 2.0, 80.0}})
		spec := envelope.Spec
		point := spec.Value.(*PointVisualizationSpec)
		point.Presentation.LabelPolicy = VisualizationLabelPolicy{
			Density: VisualizationLabelDensityAutomatic,
			Priority: []VisualizationLabelPriority{
				VisualizationLabelPrioritySelected,
				VisualizationLabelPriorityAnomaly,
				VisualizationLabelPriorityThreshold,
			},
			MaxCharacters: 24, MinimumSpacing: 6, TooltipFallback: true,
		}
		return spec
	}

	if err := ValidateSpec(valid()); err != nil {
		t.Fatalf("ValidateSpec() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*VisualizationLabelPolicy)
		want   string
	}{
		{name: "density", mutate: func(policy *VisualizationLabelPolicy) { policy.Density = "random" }, want: "unsupported label density"},
		{name: "priority", mutate: func(policy *VisualizationLabelPolicy) { policy.Priority = []VisualizationLabelPriority{"largest"} }, want: "unsupported label priority"},
		{name: "duplicate priority", mutate: func(policy *VisualizationLabelPolicy) {
			policy.Priority = []VisualizationLabelPriority{VisualizationLabelPrioritySelected, VisualizationLabelPrioritySelected}
		}, want: "duplicate label priority"},
		{name: "characters", mutate: func(policy *VisualizationLabelPolicy) { policy.MaxCharacters = 2 }, want: "max characters"},
		{name: "spacing", mutate: func(policy *VisualizationLabelPolicy) { policy.MinimumSpacing = 65 }, want: "minimum spacing"},
		{name: "hidden fallback", mutate: func(policy *VisualizationLabelPolicy) {
			policy.Density = VisualizationLabelDensityHidden
			policy.TooltipFallback = false
		}, want: "labels that can be suppressed require tooltip fallback"},
		{name: "automatic fallback", mutate: func(policy *VisualizationLabelPolicy) {
			policy.Density = VisualizationLabelDensityAutomatic
			policy.TooltipFallback = false
		}, want: "labels that can be suppressed require tooltip fallback"},
		{name: "dense fallback", mutate: func(policy *VisualizationLabelPolicy) {
			policy.Density = VisualizationLabelDensityDense
			policy.TooltipFallback = false
		}, want: "labels that can be suppressed require tooltip fallback"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec := valid()
			policy := &spec.Value.(*PointVisualizationSpec).Presentation.LabelPolicy
			test.mutate(policy)
			err := ValidateSpec(spec)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateSpec() error = %v, want containing %q", err, test.want)
			}
		})
	}

	spec := valid()
	policy := &spec.Value.(*PointVisualizationSpec).Presentation.LabelPolicy
	policy.Density = VisualizationLabelDensityAlways
	policy.TooltipFallback = false
	if err := ValidateSpec(spec); err != nil {
		t.Fatalf("always-visible label policy should not require tooltip fallback: %v", err)
	}
}
