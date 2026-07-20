package agent

import (
	"strings"
	"testing"
)

func TestContextualModelInputIncludesResolvedWorkspaceReferences(t *testing.T) {
	input := contextualModelInput("How is this calculated?", &TurnContext{
		Surface:     "chat",
		WorkspaceID: "sales",
		References: []TurnReference{{
			Reference: TurnReferenceKey{WorkspaceID: "sales", Type: "measure", ID: "orders.order_count"},
			Name:      "Order count",
			Workspace: TurnReferenceWorkspace{ID: "sales", Name: "Sales"},
			ModelID:   "sales",
			DatasetID: "orders",
			FieldID:   "order_count",
		}},
	})

	for _, want := range []string{"leapview_turn_context", `"surface":"chat"`, `"type":"measure"`, "Order count", "How is this calculated?"} {
		if !strings.Contains(input, want) {
			t.Fatalf("contextual input missing %q:\n%s", want, input)
		}
	}
}

func TestTurnContextNormalizationKeepsSameReferenceIDAcrossWorkspaces(t *testing.T) {
	normalized := (TurnContext{
		Surface: "chat",
		References: []TurnReference{
			{Reference: TurnReferenceKey{WorkspaceID: "sales", Type: "field", ID: "orders.revenue"}},
			{Reference: TurnReferenceKey{WorkspaceID: "visuals", Type: "field", ID: "orders.revenue"}},
		},
	}).normalized()

	if got := len(normalized.References); got != 2 {
		t.Fatalf("normalized references = %#v, want two workspace-qualified references", normalized.References)
	}
}
