package compiler

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/semantic"
)

func TestSemanticModelDesignWorkspaceContract(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspace(t)

	workspace, err := CompileDefinition(catalogPath)
	if err != nil {
		t.Fatalf("CompileDefinition() error = %v, want semantic-model-first workspace to load", err)
	}
	model := workspace.Models["olist"]
	if model == nil {
		t.Fatal("semantic model olist was not loaded")
	}
	if _, ok := model.Sources["olist_orders"]; !ok {
		t.Fatalf("raw source olist_orders missing: %#v", model.Sources)
	}
	if _, ok := model.Tables["orders"]; !ok {
		t.Fatalf("model table orders missing: %#v", model.Tables)
	}
	if _, ok := workspace.Dashboards["sales"]; !ok {
		t.Fatalf("dashboard sales missing: %#v", workspace.Dashboards)
	}
}

func TestSemanticModelDesignMeasureDefaultsAndOwnership(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspace(t)

	workspace, err := CompileDefinition(catalogPath)
	if err != nil {
		t.Fatalf("CompileDefinition() error = %v, want semantic model measures to load", err)
	}
	model := workspace.Models["olist"]
	if model == nil {
		t.Fatal("semantic model olist missing")
	}

	revenue, err := model.ResolveMeasure("revenue")
	if err != nil {
		t.Fatalf("ResolveMeasure(revenue) error = %v, want semantic-model measure", err)
	}
	if revenue.Table != "orders" {
		t.Fatalf("revenue table = %q, want inherited default table orders", revenue.Table)
	}
	if revenue.Expression != "SUM(orders.revenue)" {
		t.Fatalf("revenue expression = %q, want SUM(orders.revenue)", revenue.Expression)
	}
}

func TestSemanticModelDesignRequiresBaseTable(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    source: olist_orders
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
semantic_models:
  olist:
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "requires base_table") {
		t.Fatalf("CompileDefinition() error = %v, want missing base_table rejection", err)
	}
}

func TestSemanticModelDesignRejectsUnknownBaseTable(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    source: olist_orders
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
semantic_models:
  olist:
    base_table: missing
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), `base_table "missing" references unknown table`) {
		t.Fatalf("CompileDefinition() error = %v, want unknown base_table rejection", err)
	}
}

func TestSemanticModelDesignRejectsSourceSemantics(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
    measures:
      revenue:
        expr: SUM(revenue)
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "sources do not define business semantics") {
		t.Fatalf("CompileDefinition() error = %v, want raw source semantics rejection", err)
	}
}

func TestSemanticModelDesignRequiresExplicitModelTable(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithSemanticFragment(t, `
semantic_models:
  olist:
    base_table: orders
    tables:
      - missing
    measures:
      defaults: {table: missing, grain: id}
      count: {expr: COUNT(DISTINCT missing.id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), `semantic model table "missing" references unknown model`) {
		t.Fatalf("CompileDefinition() error = %v, want missing model rejection", err)
	}
}

func TestSemanticModelDesignExplicitPassThroughModelSucceeds(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithSemanticFragment(t, `
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
      - customers
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      customer_count: {expr: COUNT(DISTINCT orders.customer_id)}
      revenue: {expr: COUNT(DISTINCT orders.order_id)}
`)

	if _, err := CompileDefinition(catalogPath); err != nil {
		t.Fatalf("CompileDefinition() error = %v, want explicit passthrough model to load", err)
	}
}

func TestSemanticModelDesignRejectsUnknownRelationshipEndpoint(t *testing.T) {
	tests := map[string]string{
		"table": `
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    relationships:
      - from: orders.order_id
        to: missing.id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`,
		"field": `
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
      - customers
    relationships:
      - from: orders.missing_customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`,
	}
	for name, semanticFragment := range tests {
		t.Run(name, func(t *testing.T) {
			catalogPath := writeSemanticModelDesignWorkspaceWithSemanticFragment(t, semanticFragment)
			_, err := CompileDefinition(catalogPath)
			if err == nil || !strings.Contains(err.Error(), "relationship") {
				t.Fatalf("CompileDefinition() error = %v, want relationship endpoint rejection", err)
			}
		})
	}
}

func TestSemanticModelDesignAllowsDisconnectedFactTables(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.yaml")
	mustWriteFile(t, modelPath, `
name: olist
connections:
  olist:
    kind: local
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
  olist_customers:
    connection: olist
    path: customers.csv
    format: csv
  olist_refunds:
    connection: olist
    path: refunds.csv
    format: csv

models:
  orders:
    source: olist_orders
    primary_key: order_id
    fields:
      customer_id: {label: Customer ID}
      revenue: {label: Revenue}
  customers:
    source: olist_customers
    primary_key: customer_id
    fields:
      customer_id: {label: Customer ID}
  refunds:
    source: olist_refunds
    primary_key: refund_id
    fields:
      refund_id: {label: Refund ID}
      amount: {label: Amount}

semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
      - customers
      - refunds
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
      refunds:
        table: refunds
        grain: refund_id
        expr: SUM(refunds.amount)
`)

	model, err := semantic.Load(modelPath)
	if err != nil {
		t.Fatalf("semantic.Load() error = %v, want disconnected facts to load", err)
	}
	if model.Tables["orders"].PrimaryKey == "" || model.Tables["refunds"].PrimaryKey == "" {
		t.Fatalf("loaded model = %#v, want orders and refunds fact tables", model)
	}
}

func TestSemanticModelDesignVisualQueryTableCanTargetSeparateFact(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "catalog.yaml"), `
workspace:
  id: libredash
semantic_models:
  - id: sales
    title: Sales
    path: model.yaml
dashboards:
  - id: overview
    title: Overview
    path: dashboard.yaml
`)
	mustWriteFile(t, filepath.Join(dir, "model.yaml"), `
name: sales
connections:
  local: {kind: local}
sources:
  orders:
    connection: local
    path: orders.csv
    format: csv
  invoices:
    connection: local
    path: invoices.csv
    format: csv
models:
  orders:
    source: orders
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
      amount: {label: Amount}
  invoices:
    source: invoices
    primary_key: invoice_id
    fields:
      invoice_id: {label: Invoice ID}
      billed_amount: {label: Billed Amount}
semantic_models:
  sales:
    base_table: orders
    tables:
      - orders
      - invoices
    measures:
      defaults: {table: orders}
      order_amount:
        table: orders
        grain: order_id
        expr: SUM(orders.amount)
      billed_amount:
        table: invoices
        grain: invoice_id
        expr: SUM(invoices.billed_amount)
`)
	mustWriteFile(t, filepath.Join(dir, "dashboard.yaml"), `
id: overview
title: Overview
semantic_model: sales
filters: {}
visuals:
  billed:
    title: Billed
    type: bar
    query:
      table: invoices
      dimensions:
        invoice: invoices.invoice_id
      measures:
        billed_amount:
tables: {}
pages:
  - id: overview
    title: Overview
    visuals: []
`)

	if _, err := CompileDefinition(filepath.Join(dir, "catalog.yaml")); err != nil {
		t.Fatalf("CompileDefinition() error = %v, want visual query.table to target separate fact", err)
	}
}

func TestSemanticModelDesignRejectsVisualQueryTableWithForeignMeasure(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "catalog.yaml"), `
workspace:
  id: libredash
semantic_models:
  - id: sales
    title: Sales
    path: model.yaml
dashboards:
  - id: overview
    title: Overview
    path: dashboard.yaml
`)
	mustWriteFile(t, filepath.Join(dir, "model.yaml"), `
name: sales
connections:
  local: {kind: local}
sources:
  orders: {connection: local, path: orders.csv, format: csv}
  invoices: {connection: local, path: invoices.csv, format: csv}
models:
  orders:
    source: orders
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
      amount: {label: Amount}
  invoices:
    source: invoices
    primary_key: invoice_id
    fields:
      invoice_id: {label: Invoice ID}
semantic_models:
  sales:
    base_table: orders
    tables: [orders, invoices]
    measures:
      defaults: {table: orders}
      order_amount:
        table: orders
        grain: order_id
        expr: SUM(orders.amount)
`)
	mustWriteFile(t, filepath.Join(dir, "dashboard.yaml"), `
id: overview
title: Overview
semantic_model: sales
filters: {}
visuals:
  bad:
    title: Bad
    type: bar
    query:
      table: invoices
      dimensions:
        invoice: invoices.invoice_id
      measures:
        order_amount:
tables: {}
pages:
  - id: overview
    title: Overview
    visuals: []
`)
	_, err := CompileDefinition(filepath.Join(dir, "catalog.yaml"))
	if err == nil || !strings.Contains(err.Error(), `visual "bad" query is invalid`) || !strings.Contains(err.Error(), "cross-fact measures") {
		t.Fatalf("CompileDefinition() error = %v, want query.table foreign measure rejection", err)
	}
}

func TestSemanticModelDesignRejectsVisualQueryTableUnreachableDimension(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "catalog.yaml"), `
workspace:
  id: libredash
semantic_models:
  - id: sales
    title: Sales
    path: model.yaml
dashboards:
  - id: overview
    title: Overview
    path: dashboard.yaml
`)
	mustWriteFile(t, filepath.Join(dir, "model.yaml"), `
name: sales
connections:
  local: {kind: local}
sources:
  orders: {connection: local, path: orders.csv, format: csv}
  invoices: {connection: local, path: invoices.csv, format: csv}
models:
  orders:
    source: orders
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
  invoices:
    source: invoices
    primary_key: invoice_id
    fields:
      invoice_id: {label: Invoice ID}
      amount: {label: Amount}
semantic_models:
  sales:
    base_table: orders
    tables: [orders, invoices]
    measures:
      defaults: {table: invoices}
      billed_amount:
        table: invoices
        grain: invoice_id
        expr: SUM(invoices.amount)
`)
	mustWriteFile(t, filepath.Join(dir, "dashboard.yaml"), `
id: overview
title: Overview
semantic_model: sales
filters: {}
visuals:
  bad:
    title: Bad
    type: bar
    query:
      table: invoices
      dimensions:
        order: orders.order_id
      measures:
        billed_amount:
tables: {}
pages:
  - id: overview
    title: Overview
    visuals: []
`)
	_, err := CompileDefinition(filepath.Join(dir, "catalog.yaml"))
	if err == nil || !strings.Contains(err.Error(), `visual "bad" query is invalid`) || !strings.Contains(err.Error(), "no safe relationship path") {
		t.Fatalf("CompileDefinition() error = %v, want unreachable query.table dimension rejection", err)
	}
}

func TestSemanticModelDesignAllowsUnrelatedFactsInSeparateSemanticModels(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "catalog.yaml"), `
workspace:
  id: libredash
  title: LibreDash Workspace
semantic_models:
  - id: orders
    title: Orders
    path: orders-model.yaml
  - id: refunds
    title: Refunds
    path: refunds-model.yaml
dashboards:
  - id: sales
    title: Sales
    path: dashboard.yaml
`)
	mustWriteFile(t, filepath.Join(dir, "orders-model.yaml"), `
name: orders
title: Orders
connections:
  local:
    kind: local
sources:
  orders:
    connection: local
    path: orders.csv
    format: csv
models:
  orders:
    source: orders
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
      revenue: {label: Revenue}
semantic_models:
  orders:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`)
	mustWriteFile(t, filepath.Join(dir, "refunds-model.yaml"), `
name: refunds
title: Refunds
connections:
  local:
    kind: local
sources:
  refunds:
    connection: local
    path: refunds.csv
    format: csv
models:
  refunds:
    source: refunds
    primary_key: refund_id
    fields:
      refund_id: {label: Refund ID}
      amount: {label: Amount}
semantic_models:
  refunds:
    base_table: refunds
    tables:
      - refunds
    measures:
      defaults: {table: refunds, grain: refund_id}
      refund_amount: {expr: SUM(refunds.amount)}
`)
	mustWriteFile(t, filepath.Join(dir, "dashboard.yaml"), `
id: sales
title: Sales
semantic_model: orders
filters: {}
visuals:
  revenue:
    title: Revenue
    kind: kpi
    query:
      measures:
        revenue:
tables: {}
pages:
  - id: overview
    title: Overview
    visuals: []
`)

	workspace, err := CompileDefinition(filepath.Join(dir, "catalog.yaml"))
	if err != nil {
		t.Fatalf("CompileDefinition() error = %v, want unrelated facts split by semantic model to load", err)
	}
	if workspace.Models["orders"] == nil || workspace.Models["refunds"] == nil {
		t.Fatalf("workspace models = %#v, want orders and refunds semantic models", workspace.Models)
	}
}

func TestSemanticModelDesignSQLModelRequiresExplicitSources(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT order_id FROM source.olist_orders
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
      revenue: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "requires sources") {
		t.Fatalf("CompileDefinition() error = %v, want SQL sources rejection", err)
	}
}

func TestSemanticModelDesignSQLModelWithExplicitSourcesSucceeds(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
  olist_customers:
    connection: olist
    path: customers.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
      customer_id: {label: Customer ID}
    transform:
      sql: SELECT order_id, customer_id FROM source.olist_orders
  customers:
    source: olist_customers
    primary_key: customer_id
    fields:
      customer_id: {label: Customer ID}
      state: {label: State}
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
      - customers
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
      revenue: {expr: COUNT(DISTINCT orders.order_id)}
`)

	if _, err := CompileDefinition(catalogPath); err != nil {
		t.Fatalf("CompileDefinition() error = %v, want SQL model with explicit sources to load", err)
	}
}

func TestSemanticModelDesignSQLModelWithQuotedSourceReferenceSucceeds(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
  olist_customers:
    connection: olist
    path: customers.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
      customer_id: {label: Customer ID}
    transform:
      sql: SELECT order_id, customer_id FROM "source"."olist_orders"
  customers:
    source: olist_customers
    primary_key: customer_id
    fields:
      customer_id: {label: Customer ID}
      state: {label: State}
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
      - customers
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
      revenue: {expr: COUNT(DISTINCT orders.order_id)}
`)

	if _, err := CompileDefinition(catalogPath); err != nil {
		t.Fatalf("CompileDefinition() error = %v, want quoted source reference to load", err)
	}
}

func TestSemanticModelDesignSQLModelRejectsRawNamespace(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT order_id FROM raw.olist_orders
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
      revenue: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "model SQL must reference sources through source.<name>; raw.<name> is internal") {
		t.Fatalf("CompileDefinition() error = %v, want raw namespace rejection", err)
	}
}

func TestSemanticModelDesignSQLModelRejectsQuotedRawNamespace(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT order_id FROM "raw"."olist_orders"
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
      revenue: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "model SQL must reference sources through source.<name>; raw.<name> is internal") {
		t.Fatalf("CompileDefinition() error = %v, want quoted raw namespace rejection", err)
	}
}

func TestSemanticModelDesignSQLScannerIgnoresCommentsAndStrings(t *testing.T) {
	model := &semanticmodel.Model{Sources: map[string]semanticmodel.Source{"olist_orders": {}, "order_id": {}}}
	sourceRefs, rawRefs, unqualifiedRefs := model.SQLSourceRefs(`
		-- raw.orders and source.fake are comments
		SELECT 'raw.orders', 'source.fake', source.order_id
		FROM source.olist_orders
		/* raw.other is also a comment */
	`)
	if !reflect.DeepEqual(sourceRefs, []string{"olist_orders"}) {
		t.Fatalf("source refs = %#v, want only executable source ref", sourceRefs)
	}
	if len(rawRefs) != 0 {
		t.Fatalf("raw refs = %#v, want comments and strings ignored", rawRefs)
	}
	if len(unqualifiedRefs) != 0 {
		t.Fatalf("unqualified refs = %#v, want dotted columns outside relation contexts ignored", unqualifiedRefs)
	}
}

func TestSemanticModelDesignSQLModelSourceMismatchFails(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
  olist_customers:
    connection: olist
    path: customers.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT order_id FROM source.olist_customers
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "do not match declared sources") {
		t.Fatalf("CompileDefinition() error = %v, want SQL source mismatch rejection", err)
	}
}

func TestSemanticModelDesignSQLModelRejectsUnqualifiedSourceRead(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT order_id FROM olist_orders
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "SQL must reference sources through source.<name>") {
		t.Fatalf("CompileDefinition() error = %v, want unqualified source rejection", err)
	}
}

func TestSemanticModelDesignSQLModelRejectsUnqualifiedExternalRelation(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT o.order_id FROM source.olist_orders o JOIN leaked_table l ON l.order_id = o.order_id
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), `found unqualified relation "leaked_table"`) {
		t.Fatalf("CompileDefinition() error = %v, want hidden unqualified relation rejection", err)
	}
}

func TestSemanticModelDesignSQLModelRejectsMissingSourceRefs(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT 1 AS order_id
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "do not match declared sources") {
		t.Fatalf("CompileDefinition() error = %v, want missing source reference rejection", err)
	}
}

func TestSemanticModelDesignSQLModelAllowsCTERelation(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.yaml")
	if err := os.WriteFile(modelPath, []byte(`
name: olist
connections:
  olist: {kind: local}
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: |
        WITH cleaned AS (
          SELECT order_id FROM source.olist_orders
        )
        SELECT order_id FROM cleaned
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := semantic.Load(modelPath); err != nil {
		t.Fatalf("semantic.Load() error = %v, want CTE relation to load", err)
	}
}

func TestSemanticModelDesignSQLModelRejectsNonQuerySQL(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: UPDATE source.olist_orders SET order_id = order_id
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`)

	_, err := CompileDefinition(catalogPath)
	if err == nil || !strings.Contains(err.Error(), "must be a read-only SELECT or WITH query") {
		t.Fatalf("CompileDefinition() error = %v, want non-query SQL rejection", err)
	}
}

func TestSemanticModelDesignSQLModelScansSubquerySourceRefs(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.yaml")
	if err := os.WriteFile(modelPath, []byte(`
name: olist
connections:
  olist: {kind: local}
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
models:
  orders:
    sources: [olist_orders]
    primary_key: order_id
    fields:
      order_id: {label: Order ID}
    transform:
      sql: SELECT order_id FROM (SELECT order_id FROM source.olist_orders) orders
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
    measures:
      defaults: {table: orders, grain: order_id}
      order_count: {expr: COUNT(DISTINCT orders.order_id)}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := semantic.Load(modelPath); err != nil {
		t.Fatalf("semantic.Load() error = %v, want subquery source reference to load", err)
	}
}

func TestSemanticModelDesignAllowsIsolatedSemanticTable(t *testing.T) {
	catalogPath := writeSemanticModelDesignWorkspaceWithSemanticFragment(t, `
semantic_models:
  olist:
    base_table: orders
    tables:
      - orders
      - customers
      - items
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`)

	if _, err := CompileDefinition(catalogPath); err != nil {
		t.Fatalf("CompileDefinition() error = %v, want isolated table to load", err)
	}
}

func TestSemanticModelDesignAllowsDisconnectedNoMeasureModel(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.yaml")
	mustWriteFile(t, modelPath, `
name: inventory
connections:
  local:
    kind: local
sources:
  products:
    connection: local
    path: products.csv
    format: csv
  warehouses:
    connection: local
    path: warehouses.csv
    format: csv
models:
  products:
    source: products
    primary_key: product_id
    fields:
      product_id: {label: Product ID}
  warehouses:
    source: warehouses
    primary_key: warehouse_id
    fields:
      warehouse_id: {label: Warehouse ID}
semantic_models:
  inventory:
    base_table: products
    tables:
      - products
      - warehouses
`)

	if _, err := semantic.Load(modelPath); err != nil {
		t.Fatalf("semantic.Load() error = %v, want disconnected no-measure model to load", err)
	}
}

func TestSemanticModelDesignRejectsAmbiguousAndUnsafeRelationshipPaths(t *testing.T) {
	tests := map[string]struct {
		fragment string
		want     string
	}{
		"one_to_many": {
			fragment: `
	semantic_models:
	  olist:
	    base_table: orders
	    tables:
	      - orders
	      - items
    relationships:
      - from: orders.order_id
        to: items.order_id
        cardinality: one_to_many
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`,
			want: "unsafe relationship path",
		},
		"inactive": {
			fragment: `
	semantic_models:
	  olist:
	    base_table: orders
	    tables:
	      - orders
	      - customers
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: false
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`,
			want: "unsafe relationship path",
		},
		"ambiguous": {
			fragment: `
	semantic_models:
	  olist:
	    base_table: orders
	    tables:
	      - orders
	      - customers
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
      - from: orders.customer_id_alt
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue)}
`,
			want: "ambiguous relationship path",
		},
		"ambiguous_different_lengths": {
			fragment: `
	semantic_models:
	  olist:
	    base_table: orders
	    tables:
	      - orders
	      - items
	      - customers
	    relationships:
	      - from: orders.customer_id
	        to: customers.customer_id
	        cardinality: many_to_one
	        active: true
	      - from: orders.item_id
	        to: items.item_id
	        cardinality: many_to_one
	        active: true
	      - from: items.customer_id
	        to: customers.customer_id
	        cardinality: many_to_one
	        active: true
	    measures:
	      defaults: {table: orders, grain: order_id}
	      revenue: {expr: SUM(orders.revenue)}
`,
			want: "ambiguous relationship path",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			catalogPath := writeSemanticModelDesignWorkspaceWithSemanticFragment(t, tt.fragment)
			_, err := CompileDefinition(catalogPath)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("CompileDefinition() error = %v, want %q rejection", err, tt.want)
			}
		})
	}
}

func writeSemanticModelDesignWorkspace(t *testing.T) string {
	t.Helper()
	return writeSemanticModelDesignWorkspaceWithSemanticFragment(t, `
	semantic_models:
	  olist:
	    base_table: orders
	    tables:
	      - orders
	      - customers
    relationships:
      - from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults:
        table: orders
        grain: order_id
        time: orders.purchase_timestamp
        grains: [day, week, month, quarter, year]
      revenue:
        expr: SUM(orders.revenue)
        format: currency
      order_count:
        expr: COUNT(DISTINCT orders.order_id)
        format: integer
`)
}

func writeSemanticModelDesignWorkspaceWithSemanticFragment(t *testing.T, semanticFragment string) string {
	t.Helper()
	return writeSemanticModelDesignWorkspaceWithModelFragment(t, `
sources:
  olist_orders:
    connection: olist
    path: orders.csv
    format: csv
	  olist_customers:
	    connection: olist
	    path: customers.csv
	    format: csv
	  olist_items:
	    connection: olist
	    path: items.csv
	    format: csv

	models:
	  orders:
	    sources: [olist_orders]
	    primary_key: order_id
	    fields:
	      order_id: {label: Order ID}
	      customer_id: {label: Customer ID}
	      customer_id_alt: {label: Alternate customer ID}
	      item_id: {label: Item ID}
	      purchase_timestamp: {label: Purchase timestamp}
	      revenue: {label: Revenue}
	    transform:
	      sql: |
	        SELECT order_id, customer_id, purchase_timestamp, revenue
	        FROM source.olist_orders
	  customers:
	    source: olist_customers
	    primary_key: customer_id
	    fields:
	      customer_id: {label: Customer ID}
	      state: {label: State}
	  items:
	    source: olist_items
	    primary_key: item_id
	    fields:
	      item_id: {label: Item ID}
	      order_id: {label: Order ID}
	      customer_id: {label: Customer ID}
	`+semanticFragment)
}

func writeSemanticModelDesignWorkspaceWithModelFragment(t *testing.T, modelFragment string) string {
	t.Helper()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "catalog.yaml"), `
workspace:
  id: libredash
  title: LibreDash Workspace
  description: Local BI workspace.

semantic_models:
  - id: olist
    title: Olist Commerce
    path: model.yaml
    description: Olist model.

dashboards:
  - id: sales
    title: Sales
    path: dashboard.yaml
    description: Sales dashboard.
`)
	mustWriteFile(t, filepath.Join(dir, "model.yaml"), `
name: olist
title: Olist Commerce
description: Olist semantic model.

connections:
  olist:
    kind: local
`+modelFragment)
	mustWriteFile(t, filepath.Join(dir, "dashboard.yaml"), `
id: sales
title: Sales
semantic_model: olist
filters: {}
visuals:
  revenue_by_state:
    title: Revenue by state
    type: bar
    query:
      dimensions:
        state: customers.state
      measures:
        revenue:
    encode:
      x: state
      y: revenue
tables:
  orders:
    title: Orders
    query:
      table: orders
      fields:
        - orders.order_id
        - customers.state
pages:
  - id: overview
    title: Overview
    visuals: []
`)
	return filepath.Join(dir, "catalog.yaml")
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	content = strings.ReplaceAll(content, "\t", "")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
