package report

import (
	"strings"
	"testing"
)

func TestValidateVisualPresentationAcceptsCartesianDecisionContext(t *testing.T) {
	t.Parallel()

	minimum := 0.0
	maximum := 100.0
	target := 80.0
	visual := Visual{
		Type: "line",
		Presentation: VisualPresentation{
			Axes: []VisualAxis{
				{ID: "x", Title: "Month", TickDensity: "sparse"},
				{ID: "primary_y", Title: "Revenue", Scale: "linear", Zero: "include", Minimum: &minimum, Maximum: &maximum, Unit: "USD"},
			},
			ReferenceLines: []VisualReferenceLine{{
				ID: "target", Axis: "primary_y", Value: VisualReferenceValue{Number: &target}, Label: "Target", Tone: "success",
			}},
			ReferenceBands: []VisualReferenceBand{{
				ID: "healthy", Axis: "primary_y",
				From:  VisualReferenceValue{Number: numberPointer(70)},
				To:    VisualReferenceValue{Number: numberPointer(90)},
				Label: "Healthy range", Tone: "success",
			}},
			EventAnnotations: []VisualEventAnnotation{{
				ID: "launch", Axis: "x", Value: VisualReferenceValue{Text: "2026-03-01"}, Label: "Launch", Description: "New pricing launched",
			}},
			Tooltip: []string{"month", "revenue"},
		},
	}

	if err := validateVisualPresentation("revenue", visual); err != nil {
		t.Fatalf("validateVisualPresentation() error = %v", err)
	}
}

func TestValidateVisualPresentationRejectsInvalidDecisionContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		visualType   string
		presentation VisualPresentation
		want         string
	}{
		{
			name: "unsupported visual", visualType: "pie",
			presentation: VisualPresentation{Axes: []VisualAxis{{ID: "x"}}},
			want:         "only valid for cartesian",
		},
		{
			name: "duplicate axis", visualType: "line",
			presentation: VisualPresentation{Axes: []VisualAxis{{ID: "x"}, {ID: "x"}}},
			want:         `duplicate axis "x"`,
		},
		{
			name: "invalid log zero policy", visualType: "line",
			presentation: VisualPresentation{Axes: []VisualAxis{{ID: "primary_y", Scale: "log", Zero: "include"}}},
			want:         "log scale cannot include zero",
		},
		{
			name: "inverted domain", visualType: "line",
			presentation: VisualPresentation{Axes: []VisualAxis{{ID: "primary_y", Minimum: numberPointer(10), Maximum: numberPointer(5)}}},
			want:         "minimum must be less than maximum",
		},
		{
			name: "ambiguous value", visualType: "line",
			presentation: VisualPresentation{ReferenceLines: []VisualReferenceLine{{
				ID: "target", Axis: "primary_y", Value: VisualReferenceValue{Number: numberPointer(10), Field: "target"},
			}}},
			want: "requires exactly one",
		},
		{
			name: "duplicate annotation identity", visualType: "line",
			presentation: VisualPresentation{
				ReferenceLines: []VisualReferenceLine{{ID: "target", Axis: "primary_y", Value: VisualReferenceValue{Number: numberPointer(10)}}},
				ReferenceBands: []VisualReferenceBand{{ID: "target", Axis: "primary_y", From: VisualReferenceValue{Number: numberPointer(5)}, To: VisualReferenceValue{Number: numberPointer(15)}}},
			},
			want: `duplicate decision context ID "target"`,
		},
		{
			name: "event on value axis", visualType: "line",
			presentation: VisualPresentation{EventAnnotations: []VisualEventAnnotation{{
				ID: "launch", Axis: "primary_y", Value: VisualReferenceValue{Text: "2026-03-01"},
			}}},
			want: "event annotation axis must be x",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateVisualPresentation("visual", Visual{Type: test.visualType, Presentation: test.presentation})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateVisualPresentation() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func numberPointer(value float64) *float64 {
	return &value
}
