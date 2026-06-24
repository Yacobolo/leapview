package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/internal/workspace"
)

func TestExtractLineageCleanOwnership(t *testing.T) {
	compiled := compileLineageWorkspace(t)
	graph := compiled.Workspace.Graph

	catalog := requireLineageAsset(t, graph, workspace.AssetTypeCatalog, "libredash")
	connection := requireLineageAsset(t, graph, workspace.AssetTypeConnection, "olist.olist")
	source := requireLineageAsset(t, graph, workspace.AssetTypeSource, "olist.orders")
	modelTable := requireLineageAsset(t, graph, workspace.AssetTypeModelTable, "olist.orders")
	model := requireLineageAsset(t, graph, workspace.AssetTypeSemanticModel, "olist")
	semanticTable := requireLineageAsset(t, graph, workspace.AssetTypeSemanticTable, "olist.orders")
	relationship := requireLineageAsset(t, graph, workspace.AssetTypeRelationship, "olist.orders_customers")
	measure := requireLineageAsset(t, graph, workspace.AssetTypeMeasure, "olist.revenue")
	field := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.orders.revenue")
	dashboard := requireLineageAsset(t, graph, workspace.AssetTypeDashboard, "sales")
	page := requireLineageAsset(t, graph, workspace.AssetTypePage, "sales.overview")
	filter := requireLineageAsset(t, graph, workspace.AssetTypeFilter, "sales.status")
	visual := requireLineageAsset(t, graph, workspace.AssetTypeVisual, "sales.revenue_by_status")
	table := requireLineageAsset(t, graph, workspace.AssetTypeTable, "sales.order_rows")
	pageFilter := requireLineageAsset(t, graph, workspace.AssetTypePageItem, "sales.overview.status_card")
	pageVisual := requireLineageAsset(t, graph, workspace.AssetTypePageItem, "sales.overview.revenue_card")
	pageTable := requireLineageAsset(t, graph, workspace.AssetTypePageItem, "sales.overview.orders_table")

	requireLineageEdge(t, graph, catalog, connection, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, catalog, source, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, catalog, modelTable, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, catalog, model, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, catalog, dashboard, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, model, semanticTable, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, model, relationship, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, model, measure, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, semanticTable, field, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, dashboard, page, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, dashboard, filter, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, dashboard, visual, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, dashboard, table, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, page, pageFilter, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, page, pageVisual, workspace.AssetEdgeContains)
	requireLineageEdge(t, graph, page, pageTable, workspace.AssetEdgeContains)
}

func TestExtractLineageCleanDependencies(t *testing.T) {
	compiled := compileLineageWorkspace(t)
	graph := compiled.Workspace.Graph

	connection := requireLineageAsset(t, graph, workspace.AssetTypeConnection, "olist.olist")
	source := requireLineageAsset(t, graph, workspace.AssetTypeSource, "olist.orders")
	modelTable := requireLineageAsset(t, graph, workspace.AssetTypeModelTable, "olist.orders")
	model := requireLineageAsset(t, graph, workspace.AssetTypeSemanticModel, "olist")
	semanticTable := requireLineageAsset(t, graph, workspace.AssetTypeSemanticTable, "olist.orders")
	relationship := requireLineageAsset(t, graph, workspace.AssetTypeRelationship, "olist.orders_customers")
	orderCustomerField := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.orders.customer_id")
	customerIDField := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.customers.customer_id")
	revenueField := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.orders.revenue")
	statusField := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.orders.status")
	measure := requireLineageAsset(t, graph, workspace.AssetTypeMeasure, "olist.revenue")
	dashboard := requireLineageAsset(t, graph, workspace.AssetTypeDashboard, "sales")
	filter := requireLineageAsset(t, graph, workspace.AssetTypeFilter, "sales.status")
	visual := requireLineageAsset(t, graph, workspace.AssetTypeVisual, "sales.revenue_by_status")
	table := requireLineageAsset(t, graph, workspace.AssetTypeTable, "sales.order_rows")
	pageFilter := requireLineageAsset(t, graph, workspace.AssetTypePageItem, "sales.overview.status_card")
	pageVisual := requireLineageAsset(t, graph, workspace.AssetTypePageItem, "sales.overview.revenue_card")
	pageTable := requireLineageAsset(t, graph, workspace.AssetTypePageItem, "sales.overview.orders_table")

	requireLineageEdge(t, graph, source, connection, workspace.AssetEdgeUsesConnection)
	requireLineageEdge(t, graph, modelTable, source, workspace.AssetEdgeReadsSource)
	requireLineageEdge(t, graph, semanticTable, modelTable, workspace.AssetEdgeUsesModelTable)
	requireLineageEdge(t, graph, relationship, orderCustomerField, workspace.AssetEdgeUsesField)
	requireLineageEdge(t, graph, relationship, customerIDField, workspace.AssetEdgeUsesField)
	requireLineageEdge(t, graph, measure, semanticTable, workspace.AssetEdgeUsesSemanticTable)
	requireLineageEdge(t, graph, measure, revenueField, workspace.AssetEdgeUsesField)
	requireLineageEdge(t, graph, dashboard, model, workspace.AssetEdgeUsesSemanticModel)
	requireLineageEdge(t, graph, filter, statusField, workspace.AssetEdgeFiltersField)
	requireLineageEdge(t, graph, visual, measure, workspace.AssetEdgeUsesMeasure)
	requireLineageEdge(t, graph, visual, statusField, workspace.AssetEdgeUsesField)
	requireLineageEdge(t, graph, table, revenueField, workspace.AssetEdgeUsesField)
	requireLineageEdge(t, graph, pageFilter, filter, workspace.AssetEdgeUsesFilter)
	requireLineageEdge(t, graph, pageVisual, visual, workspace.AssetEdgeUsesVisual)
	requireLineageEdge(t, graph, pageTable, table, workspace.AssetEdgeUsesTable)

	for _, edge := range graph.Edges {
		if edge.FromAssetID != dashboard.ID {
			continue
		}
		switch edge.Type {
		case workspace.AssetEdgeUsesModelTable, workspace.AssetEdgeUsesMeasure, workspace.AssetEdgeUsesField:
			t.Fatalf("dashboard rollup edge persisted: %#v", edge)
		}
	}
	for _, edge := range graph.Edges {
		if edge.FromAssetID == table.ID && edge.ToAssetID == semanticTable.ID && edge.Type == workspace.AssetEdgeUsesSemanticTable {
			t.Fatalf("dashboard table semantic-table dependency persisted: %#v", edge)
		}
	}
}

func TestExtractLineageRelationshipPrimaryKeyEndpoints(t *testing.T) {
	dir := t.TempDir()
	writeCompilerFixture(t, filepath.Join(dir, "catalog.yaml"), validCompilerCatalogYAML())
	writeCompilerFixture(t, filepath.Join(dir, "model.yaml"), `
name: olist
title: Olist
connections:
  olist:
    kind: local
sources:
  orders:
    connection: olist
    path: orders.csv
    format: csv
  customers:
    connection: olist
    path: customers.csv
    format: csv
models:
  orders:
    source: orders
  customers:
    source: customers
semantic_models:
  olist:
    base_table: orders
    tables:
      orders:
        model: orders
        primary_key: order_id
        fields:
          customer_id: {expr: customer_id}
      customers:
        model: customers
        primary_key: customer_id
        fields:
          state: {expr: customer_state}
    relationships:
      - id: order_customer
        from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: COUNT(orders.order_id)}
`)
	writeCompilerFixture(t, filepath.Join(dir, "dashboard.yaml"), validCompilerDashboardYAML())

	compiled, err := Compile(filepath.Join(dir, "catalog.yaml"), Options{WorkspaceID: "libredash", DeploymentID: "dep_test"})
	if err != nil {
		t.Fatalf("Compile() error = %v, want primary-key relationship endpoint to compile", err)
	}
	graph := compiled.Workspace.Graph
	relationship := requireLineageAsset(t, graph, workspace.AssetTypeRelationship, "olist.order_customer")
	measure := requireLineageAsset(t, graph, workspace.AssetTypeMeasure, "olist.revenue")
	orderPK := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.orders.order_id")
	customerPK := requireLineageAsset(t, graph, workspace.AssetTypeField, "olist.customers.customer_id")
	requireLineageEdge(t, graph, relationship, customerPK, workspace.AssetEdgeUsesField)
	requireLineageEdge(t, graph, measure, orderPK, workspace.AssetEdgeUsesField)
}

func compileLineageWorkspace(t *testing.T) CompiledWorkspace {
	t.Helper()
	dir := t.TempDir()
	writeCompilerFixture(t, filepath.Join(dir, "catalog.yaml"), validCompilerCatalogYAML())
	writeCompilerFixture(t, filepath.Join(dir, "model.yaml"), `
name: olist
title: Olist
connections:
  olist:
    kind: local
sources:
  orders:
    connection: olist
    path: orders.csv
    format: csv
  customers:
    connection: olist
    path: customers.csv
    format: csv
models:
  orders:
    source: orders
  customers:
    source: customers
semantic_models:
  olist:
    base_table: orders
    tables:
      orders:
        model: orders
        primary_key: order_id
        fields:
          order_id: {expr: order_id}
          customer_id: {expr: customer_id}
          status: {expr: status}
          revenue: {expr: revenue}
      customers:
        model: customers
        primary_key: customer_id
        fields:
          customer_id: {expr: customer_id}
          state: {expr: customer_state}
    relationships:
      - id: orders_customers
        from: orders.customer_id
        to: customers.customer_id
        cardinality: many_to_one
        active: true
    measures:
      defaults: {table: orders, grain: order_id}
      revenue: {expr: SUM(orders.revenue), format: currency}
`)
	writeCompilerFixture(t, filepath.Join(dir, "dashboard.yaml"), `
id: sales
title: Sales
semantic_model: olist
filters:
  status:
    type: multi_select
    label: Status
    url_param: status
    operator: in
    field: orders.status
visuals:
  revenue_by_status:
    title: Revenue by status
    type: bar
    query:
      dimensions:
        status: orders.status
      measures:
        revenue:
tables:
  order_rows:
    title: Order rows
    query:
      table: orders
      fields:
        - orders.status
        - orders.revenue
pages:
  - id: overview
    title: Overview
    visuals:
      - id: status_card
        kind: filter_card
        filter: status
        placement: {col: 1, row: 1, col_span: 3, row_span: 2}
      - id: revenue_card
        kind: bar_chart
        visual: revenue_by_status
        placement: {col: 4, row: 1, col_span: 5, row_span: 4}
      - id: orders_table
        kind: table
        table: order_rows
        placement: {col: 1, row: 5, col_span: 8, row_span: 4}
`)

	compiled, err := Compile(filepath.Join(dir, "catalog.yaml"), Options{WorkspaceID: "libredash", DeploymentID: "dep_test"})
	if err != nil {
		t.Fatalf("Compile() error = %v, want lineage workspace to compile", err)
	}
	return compiled
}

func requireLineageAsset(t *testing.T, graph workspace.AssetGraph, typ workspace.AssetType, key string) workspace.Asset {
	t.Helper()
	for _, asset := range graph.Assets {
		if asset.Type == typ && asset.Key == key {
			return asset
		}
	}
	t.Fatalf("missing asset %s %q; got %s", typ, key, lineageAssetKeys(graph))
	return workspace.Asset{}
}

func requireLineageEdge(t *testing.T, graph workspace.AssetGraph, from workspace.Asset, to workspace.Asset, typ workspace.AssetEdgeType) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.FromAssetID == from.ID && edge.ToAssetID == to.ID && edge.Type == typ {
			return
		}
	}
	t.Fatalf("missing edge %s %q -> %q; got %s", typ, from.Key, to.Key, lineageEdgeKeys(graph))
}

func lineageAssetKeys(graph workspace.AssetGraph) string {
	values := make([]string, 0, len(graph.Assets))
	for _, asset := range graph.Assets {
		values = append(values, string(asset.Type)+":"+asset.Key)
	}
	return strings.Join(values, ", ")
}

func lineageEdgeKeys(graph workspace.AssetGraph) string {
	assetByID := map[workspace.AssetID]workspace.Asset{}
	for _, asset := range graph.Assets {
		assetByID[asset.ID] = asset
	}
	values := make([]string, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		from := assetByID[edge.FromAssetID]
		to := assetByID[edge.ToAssetID]
		values = append(values, from.Key+" -"+string(edge.Type)+"-> "+to.Key)
	}
	return strings.Join(values, ", ")
}
