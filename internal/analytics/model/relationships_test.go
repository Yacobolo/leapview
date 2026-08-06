package model

import (
	"strings"
	"testing"
)

func TestRelationshipCardinalityMatrixRequiresKeyedOneEndpoints(t *testing.T) {
	tests := []struct {
		name         string
		relationship Relationship
		wantErr      string
	}{
		{
			name: "many to one targets primary key",
			relationship: Relationship{
				ID: "orders_customers", From: "orders.customer_id", To: "customers.customer_id", Cardinality: "many_to_one",
			},
		},
		{
			name: "many to one rejects non key target",
			relationship: Relationship{
				ID: "orders_customers_by_region", From: "orders.region", To: "customers.region", Cardinality: "many_to_one",
			},
			wantErr: `relationship "orders_customers_by_region" many_to_one endpoint "customers.region" must be primary key "customer_id"`,
		},
		{
			name: "one to one requires both primary keys",
			relationship: Relationship{
				ID: "customers_profiles", From: "customers.customer_id", To: "profiles.customer_id", Cardinality: "one_to_one",
			},
		},
		{
			name: "one to one rejects non key source",
			relationship: Relationship{
				ID: "customers_profiles_by_region", From: "customers.region", To: "profiles.customer_id", Cardinality: "one_to_one",
			},
			wantErr: `relationship "customers_profiles_by_region" one_to_one endpoint "customers.region" must be primary key "customer_id"`,
		},
		{
			name: "one to one rejects non key target",
			relationship: Relationship{
				ID: "customers_profiles_by_tier", From: "customers.customer_id", To: "profiles.tier", Cardinality: "one_to_one",
			},
			wantErr: `relationship "customers_profiles_by_tier" one_to_one endpoint "profiles.tier" must be primary key "customer_id"`,
		},
		{
			name: "one to many is unsafe",
			relationship: Relationship{
				ID: "customers_orders", From: "customers.customer_id", To: "orders.customer_id", Cardinality: "one_to_many",
			},
			wantErr: `relationship "customers_orders" has unsafe cardinality "one_to_many"`,
		},
		{
			name: "many to many is unsafe",
			relationship: Relationship{
				ID: "orders_tags", From: "orders.order_id", To: "tags.order_id", Cardinality: "many_to_many",
			},
			wantErr: `relationship "orders_tags" has unsafe cardinality "many_to_many"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := relationshipMatrixModel()
			model.Relationships = []Relationship{test.relationship}
			err := model.validateSemanticGraph()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateSemanticGraph() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateSemanticGraph() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestSafeRelationshipPathRejectsCyclicAlternativePaths(t *testing.T) {
	model := relationshipMatrixModel()
	model.Relationships = []Relationship{
		{ID: "customers_profiles", From: "customers.customer_id", To: "profiles.customer_id", Cardinality: "one_to_one"},
		{ID: "profiles_accounts", From: "profiles.customer_id", To: "accounts.customer_id", Cardinality: "one_to_one"},
		{ID: "accounts_customers", From: "accounts.customer_id", To: "customers.customer_id", Cardinality: "one_to_one"},
	}

	_, err := model.SafeRelationshipPath("customers", "accounts")
	if err == nil || !strings.Contains(err.Error(), "ambiguous relationship path") {
		t.Fatalf("SafeRelationshipPath() error = %v, want cyclic ambiguity rejection", err)
	}
	_, err = model.SafeRelationshipPath("customers", "tags")
	if err == nil || !strings.Contains(err.Error(), "no safe relationship path") {
		t.Fatalf("SafeRelationshipPath() error = %v, want unreachable target rejection", err)
	}
}

func relationshipMatrixModel() *Model {
	return &Model{
		Name: "fanout_matrix",
		Tables: map[string]Table{
			"orders": {
				PrimaryKey: "order_id",
				Dimensions: map[string]MetricDimension{
					"order_id": {Type: "string"}, "customer_id": {Type: "string"}, "region": {Type: "string"},
				},
			},
			"customers": {
				PrimaryKey: "customer_id",
				Dimensions: map[string]MetricDimension{
					"customer_id": {Type: "string"}, "region": {Type: "string"},
				},
			},
			"profiles": {
				PrimaryKey: "customer_id",
				Dimensions: map[string]MetricDimension{
					"customer_id": {Type: "string"}, "tier": {Type: "string"},
				},
			},
			"accounts": {
				PrimaryKey: "customer_id",
				Dimensions: map[string]MetricDimension{"customer_id": {Type: "string"}},
			},
			"tags": {
				PrimaryKey: "tag_id",
				Dimensions: map[string]MetricDimension{
					"tag_id": {Type: "string"}, "order_id": {Type: "string"},
				},
			},
		},
		Dimensions: map[string]SemanticDimension{},
		Measures:   map[string]MetricMeasure{},
		Metrics:    map[string]Metric{},
	}
}
