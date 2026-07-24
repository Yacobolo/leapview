package sqlite

import (
	"testing"

	"github.com/Yacobolo/leapview/internal/access"
)

func TestMatchExpressionCompilesSafePrefixAndPhraseTerms(t *testing.T) {
	tests := map[string]string{
		"orders by region":       `"orders"* AND "region"*`,
		`"orders by region"`:     `"orders by region"`,
		`orders "regional sales`: `"orders"* AND "regional"* AND "sales"*`,
		`the and to`:             "",
	}
	for query, want := range tests {
		if got := matchExpression(query); got != want {
			t.Errorf("matchExpression(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestFieldSecurityObjectDistinguishesModelFieldsFromTableColumns(t *testing.T) {
	model := document{workspaceID: "sales", assetType: "semantic_model", assetKey: "sales.commerce"}
	table := document{workspaceID: "sales", assetType: "semantic_table", assetKey: "sales.commerce.orders"}

	conformed := securityObject(document{
		workspaceID: "sales", assetType: "field", assetKey: "sales.commerce.order_date",
	}, []document{model})
	wantConformed := access.ItemObjectWithParent(
		access.SecurableSemanticField, "sales", "commerce/order_date",
		access.ItemObjectWithParent(access.SecurableSemanticModel, "sales", "commerce", access.WorkspaceObject("sales")),
	)
	if conformed != wantConformed {
		t.Fatalf("conformed field security object = %#v, want %#v", conformed, wantConformed)
	}

	column := securityObject(document{
		workspaceID: "sales", assetType: "field", assetKey: "sales.commerce.orders.status",
	}, []document{table, model})
	wantColumn := access.ItemObjectWithParent(
		access.SecurableColumn, "sales", "commerce/orders/status",
		access.ItemObjectWithParent(
			access.SecurableDataset, "sales", "commerce/orders",
			access.ItemObjectWithParent(access.SecurableSemanticModel, "sales", "commerce", access.WorkspaceObject("sales")),
		),
	)
	if column != wantColumn {
		t.Fatalf("table field security object = %#v, want %#v", column, wantColumn)
	}
}
