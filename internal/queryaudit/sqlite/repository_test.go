package sqlite

import (
	"context"
	"testing"

	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/queryaudit"
)

func TestRepositoryRecordsAndFiltersQueryEvents(t *testing.T) {
	ctx := context.Background()
	store, err := platform.Open(ctx, t.TempDir()+"/libredash.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	repo := NewRepository(store.SQLDB())
	events := []queryaudit.EventInput{
		{WorkspaceID: "sales", PrincipalID: "p1", Surface: "api", Operation: "api_query", QueryKind: "semantic_aggregate", ModelID: "sales", Target: "orders", Status: "success", RowsReturned: 10, SQL: "select * from orders", QueryJSON: `{"target":"orders"}`},
		{WorkspaceID: "sales", PrincipalID: "p2", Surface: "data_explorer", Operation: "preview_window", QueryKind: "model_table_rows", ModelID: "sales", Target: "customers", Status: "error", Error: "missing table", QueryJSON: `{"target":"customers"}`},
		{WorkspaceID: "operations", PrincipalID: "p1", Surface: "agent", Operation: "agent_query", QueryKind: "semantic_rows", ModelID: "operations", Target: "reviews", Status: "success", QueryJSON: `{"target":"reviews"}`},
	}
	for _, event := range events {
		if err := repo.RecordQueryEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	sales, err := repo.ListQueryEvents(ctx, queryaudit.Filter{WorkspaceID: "sales"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sales) != 2 {
		t.Fatalf("sales events = %d, want 2", len(sales))
	}

	errors, err := repo.ListQueryEvents(ctx, queryaudit.Filter{Status: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if len(errors) != 1 || errors[0].Target != "customers" {
		t.Fatalf("error events = %#v, want customers", errors)
	}

	search, err := repo.ListQueryEvents(ctx, queryaudit.Filter{Search: "orders"})
	if err != nil {
		t.Fatal(err)
	}
	if len(search) != 1 || search[0].Target != "orders" {
		t.Fatalf("search events = %#v, want orders", search)
	}
}
