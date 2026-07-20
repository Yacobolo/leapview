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
			Kind:      "measure",
			ID:        "measure:orders.order_count",
			Title:     "Order count",
			ModelID:   "sales",
			DatasetID: "orders",
			FieldID:   "order_count",
		}},
	})

	for _, want := range []string{"libredash_turn_context", `"surface":"chat"`, `"kind":"measure"`, "Order count", "How is this calculated?"} {
		if !strings.Contains(input, want) {
			t.Fatalf("contextual input missing %q:\n%s", want, input)
		}
	}
}

func TestTurnContextNormalizationKeepsSameReferenceIDAcrossWorkspaces(t *testing.T) {
	normalized := (TurnContext{
		Surface: "chat",
		References: []TurnReference{
			{Kind: "field", ID: "orders.revenue", WorkspaceID: "sales"},
			{Kind: "field", ID: "orders.revenue", WorkspaceID: "visuals"},
		},
	}).normalized()

	if got := len(normalized.References); got != 2 {
		t.Fatalf("normalized references = %#v, want two workspace-qualified references", normalized.References)
	}
}
