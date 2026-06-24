package materialize

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	analyticsduckdb "github.com/Yacobolo/libredash/internal/analytics/duckdb"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	_ "github.com/duckdb/duckdb-go/v2"
)

func TestRegistersCSVSourcesAndMaterializesModelTables(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "orders.csv", "order_id,revenue\no1,10.50\no2,20.25\n")
	db, err := sql.Open("duckdb", filepath.Join(dir, "test.duckdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	model := &semanticmodel.Model{
		Name:              "test",
		DefaultConnection: "local_files",
		Connections: map[string]semanticmodel.Connection{
			"local_files": {
				Kind:     "local",
				Defaults: semanticmodel.ConnectionDefaults{Options: map[string]any{"header": true}},
			},
		},
		Sources: map[string]semanticmodel.Source{
			"orders": {Path: "orders.csv", Connection: "local_files"},
		},
		BaseTable: "orders",
		Tables: map[string]semanticmodel.Table{
			"orders": {
				Kind: "fact", Sources: []string{"orders"},
				Transform: semanticmodel.Transform{SQL: `
					SELECT order_id, try_cast(revenue AS DOUBLE) AS revenue
					FROM source.orders
				`},
				PrimaryKey: "order_id",
				Grain:      "order_id",
				Dimensions: map[string]semanticmodel.MetricDimension{"order_id": {Expr: "order_id"}},
				Measures:   map[string]semanticmodel.MetricMeasure{"revenue": {Label: "Revenue", Expression: "SUM(orders.revenue)"}},
			},
		},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("validate model: %v", err)
	}
	if _, err := Refresh(context.Background(), db, model, dir, map[string]struct{}{}); err != nil {
		t.Fatalf("refresh materializations: %v", err)
	}

	var total float64
	if err := db.QueryRowContext(context.Background(), "SELECT SUM(revenue) FROM model.orders").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 30.75 {
		t.Fatalf("total revenue = %v, want 30.75", total)
	}
}

func TestRegistersDatabaseSourceTwice(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.sqlite")
	db, err := sql.Open("duckdb", filepath.Join(dir, "test.duckdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), "INSTALL sqlite"); err != nil {
		t.Skipf("sqlite extension unavailable: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "LOAD sqlite"); err != nil {
		t.Skipf("sqlite extension unavailable: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "ATTACH '"+analyticsduckdb.SQLString(sourcePath)+"' AS seed (TYPE sqlite)"); err != nil {
		t.Fatalf("attach seed sqlite: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "CREATE TABLE seed.accounts (id INTEGER, name VARCHAR)"); err != nil {
		t.Fatalf("create seed table: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "INSERT INTO seed.accounts VALUES (1, 'Acme')"); err != nil {
		t.Fatalf("insert seed table: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "DETACH seed"); err != nil {
		t.Fatalf("detach seed sqlite: %v", err)
	}

	model := &semanticmodel.Model{
		Name: "test",
		Connections: map[string]semanticmodel.Connection{
			"crm": {Kind: "sqlite", Options: map[string]any{"path": sourcePath}},
		},
		Sources: map[string]semanticmodel.Source{
			"accounts": {Connection: "crm", Object: "accounts"},
		},
		BaseTable: "accounts",
		Tables: map[string]semanticmodel.Table{
			"accounts": {
				Kind: "dimension", Source: "accounts", PrimaryKey: "id", Grain: "id",
				Dimensions: map[string]semanticmodel.MetricDimension{"id": {Expr: "id"}, "name": {Expr: "name"}},
			},
		},
	}
	attached := map[string]struct{}{}
	for i := 0; i < 2; i++ {
		if _, err := Refresh(context.Background(), db, model, dir, attached); err != nil {
			t.Fatalf("refresh pass %d: %v", i+1, err)
		}
	}

	var name string
	if err := db.QueryRowContext(context.Background(), "SELECT name FROM model.accounts WHERE id = 1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Acme" {
		t.Fatalf("name = %q, want Acme", name)
	}
}

func TestValidateFilesIgnoresRemoteSources(t *testing.T) {
	model := &semanticmodel.Model{
		Connections: map[string]semanticmodel.Connection{
			"lake": {Kind: "s3"},
		},
		Sources: map[string]semanticmodel.Source{
			"events": {Format: "parquet", Path: "s3://bucket/events/*.parquet", Connection: "lake"},
		},
	}
	if err := ValidateFiles(model, t.TempDir()); err != nil {
		t.Fatalf("validate files = %v, want nil", err)
	}
}

func TestValidateFilesUsesLocalConnectionRoot(t *testing.T) {
	dir := t.TempDir()
	model := &semanticmodel.Model{
		Connections: map[string]semanticmodel.Connection{
			"local_files": {Kind: "local", Root: "fixtures"},
		},
		Sources: map[string]semanticmodel.Source{
			"orders": {Format: "csv", Path: "orders.csv", Connection: "local_files"},
		},
	}
	err := ValidateFiles(model, dir)
	var missing *MissingDataError
	if !errors.As(err, &missing) {
		t.Fatalf("validate files error = %v, want MissingDataError", err)
	}
	want := filepath.Join(dir, "fixtures", "orders.csv")
	if len(missing.Missing) != 1 || missing.Missing[0] != want {
		t.Fatalf("missing files = %#v, want %q", missing.Missing, want)
	}
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
