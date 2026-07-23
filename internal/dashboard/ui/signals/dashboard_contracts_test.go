package signals

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Yacobolo/leapview/internal/dashboard"
)

func TestDashboardContractConversionsPreserveJSON(t *testing.T) {
	t.Parallel()

	selections := []dashboard.InteractionSelection{{ID: "visual:orders:point", SourceKind: "visual", SourceID: "orders", InteractionKind: "point", Label: "42", Order: 1, Entries: []dashboard.InteractionSelectionEntry{{Label: "42", Mappings: []dashboard.InteractionSelectionMapping{{Field: "ratings.rating_bucket", Fact: "ratings", Value: float64(42), Label: "Rating"}}}}}}
	assertSameJSON(t, selections, DashboardInteractionSelectionsFromDashboard(selections))
	spatial := []dashboard.SpatialInteractionSelection{}
	assertSameJSON(t, spatial, DashboardSpatialSelectionsFromDashboard(spatial))
}

func assertSameJSON(t *testing.T, left, right any) {
	t.Helper()
	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal source: %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal contract: %v", err)
	}
	var leftValue, rightValue any
	if err := json.Unmarshal(leftJSON, &leftValue); err != nil {
		t.Fatalf("decode source: %v", err)
	}
	if err := json.Unmarshal(rightJSON, &rightValue); err != nil {
		t.Fatalf("decode contract: %v", err)
	}
	if !reflect.DeepEqual(leftValue, rightValue) {
		t.Fatalf("JSON differs:\nsource:   %s\ncontract: %s", leftJSON, rightJSON)
	}
}
