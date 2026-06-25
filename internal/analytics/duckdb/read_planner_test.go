package duckdb

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	analyticsmaterialize "github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	_ "github.com/duckdb/duckdb-go/v2"
)

func TestPlanSourceReadsInfersJoinAndFilterColumns(t *testing.T) {
	ctx := context.Background()
	db := openPlanningRuntimeDB(t)
	defer db.Close()
	model := planningModel(map[string][]string{
		"orders":   {"order_id", "customer_id", "status"},
		"payments": {"order_id", "payment_value"},
	}, semanticmodel.Table{
		Sources:    []string{"orders", "payments"},
		PrimaryKey: "order_id",
		Dimensions: map[string]semanticmodel.MetricDimension{"order_id": {Label: "Order ID"}},
		Transform: semanticmodel.Transform{SQL: `
			SELECT o.order_id, o.customer_id, SUM(try_cast(p.payment_value AS DOUBLE)) AS revenue
			FROM source.orders o
			JOIN source.payments p USING (order_id)
			WHERE o.status = 'delivered'
			GROUP BY o.order_id, o.customer_id
		`},
	})
	if err := model.Validate(); err != nil {
		t.Fatal(err)
	}

	plans, err := PlanSourceReads(ctx, db, model, "", "orders", model.Tables["orders"])
	if err != nil {
		t.Fatal(err)
	}
	want := []analyticsmaterialize.SourceReadPlan{
		{Source: "orders", Fields: []string{"customer_id", "order_id", "status"}},
		{Source: "payments", Fields: []string{"order_id", "payment_value"}},
	}
	if !reflect.DeepEqual(plans, want) {
		t.Fatalf("plans = %#v, want %#v", plans, want)
	}
}

func TestPlanSourceReadsExpandsStar(t *testing.T) {
	ctx := context.Background()
	db := openPlanningRuntimeDB(t)
	defer db.Close()
	model := planningModel(map[string][]string{
		"orders": {"order_id", "customer_id", "status"},
	}, semanticmodel.Table{
		Sources:    []string{"orders"},
		PrimaryKey: "order_id",
		Dimensions: map[string]semanticmodel.MetricDimension{
			"order_id":    {Label: "Order ID"},
			"customer_id": {Label: "Customer ID"},
			"status":      {Label: "Status"},
		},
		Transform: semanticmodel.Transform{SQL: `SELECT * FROM source.orders`},
	})
	if err := model.Validate(); err != nil {
		t.Fatal(err)
	}

	plans, err := PlanSourceReads(ctx, db, model, "", "orders", model.Tables["orders"])
	if err != nil {
		t.Fatal(err)
	}
	want := []analyticsmaterialize.SourceReadPlan{{Source: "orders", Fields: []string{"customer_id", "order_id", "status"}}}
	if !reflect.DeepEqual(plans, want) {
		t.Fatalf("plans = %#v, want %#v", plans, want)
	}
}

func TestPlanSourceReadsUsesRowPresenceForCountStar(t *testing.T) {
	ctx := context.Background()
	db := openPlanningRuntimeDB(t)
	defer db.Close()
	model := planningModel(map[string][]string{
		"orders": {"order_id", "customer_id"},
	}, semanticmodel.Table{
		Sources:    []string{"orders"},
		PrimaryKey: "order_count",
		Dimensions: map[string]semanticmodel.MetricDimension{
			"order_count": {Label: "Order Count"},
		},
		Transform: semanticmodel.Transform{SQL: `SELECT COUNT(*) AS order_count FROM source.orders`},
	})
	if err := model.Validate(); err != nil {
		t.Fatal(err)
	}

	plans, err := PlanSourceReads(ctx, db, model, "", "orders", model.Tables["orders"])
	if err != nil {
		t.Fatal(err)
	}
	want := []analyticsmaterialize.SourceReadPlan{{Source: "orders", Fields: []string{}, RowPresenceOnly: true}}
	if !reflect.DeepEqual(plans, want) {
		t.Fatalf("plans = %#v, want %#v", plans, want)
	}
}

func TestPlanSourceReadsRejectsUndeclaredSource(t *testing.T) {
	db := openPlanningRuntimeDB(t)
	defer db.Close()
	model := planningModel(map[string][]string{
		"orders":   {"order_id"},
		"payments": {"order_id"},
	}, semanticmodel.Table{
		Sources:    []string{"orders"},
		PrimaryKey: "order_id",
		Dimensions: map[string]semanticmodel.MetricDimension{"order_id": {Label: "Order ID"}},
		Transform:  semanticmodel.Transform{SQL: `SELECT order_id FROM source.payments`},
	})
	err := model.Validate()
	if err == nil || !strings.Contains(err.Error(), "do not match declared sources") {
		t.Fatalf("Validate() error = %v, want source mismatch", err)
	}
}

func openPlanningRuntimeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func planningModel(sourceColumns map[string][]string, table semanticmodel.Table) *semanticmodel.Model {
	sources := map[string]semanticmodel.Source{}
	for name, columns := range sourceColumns {
		schemaColumns := make([]semanticmodel.ColumnSchema, 0, len(columns))
		for index, column := range columns {
			schemaColumns = append(schemaColumns, semanticmodel.ColumnSchema{Name: column, Ordinal: index + 1, PhysicalType: "VARCHAR"})
		}
		sources[name] = semanticmodel.Source{
			Connection: "local_files",
			Path:       name + ".csv",
			Format:     "csv",
			Schema:     semanticmodel.TableSchema{Columns: schemaColumns},
		}
	}
	return &semanticmodel.Model{
		Name:        "test",
		Connections: map[string]semanticmodel.Connection{"local_files": {Kind: "local"}},
		Sources:     sources,
		BaseTable:   "orders",
		Tables:      map[string]semanticmodel.Table{"orders": table},
		Measures:    map[string]semanticmodel.MetricMeasure{},
	}
}
