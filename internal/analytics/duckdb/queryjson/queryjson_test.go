package queryjson

import (
	"reflect"
	"testing"
)

func TestAnalyzeSQLFindsSourceRefsAliasesAndRawRefs(t *testing.T) {
	input := []byte(`{
		"error": false,
		"statements": [{
			"node": {
				"type": "SELECT_NODE",
				"cte_map": {
					"map": [{
						"key": "revenue",
						"value": {
							"query": {
								"node": {
									"type": "SELECT_NODE",
									"from_table": {
										"type": "BASE_TABLE",
										"schema_name": "source",
										"table_name": "payments",
										"alias": "p"
									}
								}
							}
						}
					}]
				},
				"from_table": {
					"type": "JOIN",
					"left": {
						"type": "BASE_TABLE",
						"schema_name": "source",
						"table_name": "orders",
						"alias": "o"
					},
					"right": {
						"type": "BASE_TABLE",
						"schema_name": "raw",
						"table_name": "legacy_orders"
					}
				}
			}
		}]
	}`)

	got, err := AnalyzeSQL(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.SourceRefs, []string{"orders", "payments"}) {
		t.Fatalf("source refs = %#v, want orders/payments", got.SourceRefs)
	}
	if !reflect.DeepEqual(got.RawRefs, []string{"legacy_orders"}) {
		t.Fatalf("raw refs = %#v, want legacy_orders", got.RawRefs)
	}
	if !reflect.DeepEqual(got.CTEs, []string{"revenue"}) {
		t.Fatalf("ctes = %#v, want revenue", got.CTEs)
	}
	if got.Aliases["o"].Table != "orders" || got.Aliases["p"].Table != "payments" {
		t.Fatalf("aliases = %#v, want o/orders and p/payments", got.Aliases)
	}
}

func TestAnalyzeSQLRejectsSerializerError(t *testing.T) {
	_, err := AnalyzeSQL([]byte(`{"error": true, "error_message": "syntax error"}`))
	if err == nil || err.Error() != "duckdb SQL JSON error: syntax error" {
		t.Fatalf("AnalyzeSQL() error = %v, want serializer error", err)
	}
}

func TestAnalyzeExplainNormalizesScansAndProjections(t *testing.T) {
	input := []byte(`[
		{
			"name": "PROJECTION",
			"children": [
				{
					"name": "SEQ_SCAN",
					"extra_info": {
						"Table": "memory.\"source\".orders",
						"Projections": ["status", "order_id", "customer_id"]
					}
				},
				{
					"name": "SEQ_SCAN",
					"extra_info": {
						"Table": "memory.\"source\".payments",
						"Projections": "order_id\npayment_value"
					}
				}
			]
		}
	]`)

	got, err := AnalyzeExplain(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []Scan{
		{Operator: "SEQ_SCAN", Catalog: "memory", Schema: "source", Table: "orders", Projections: []string{"status", "order_id", "customer_id"}},
		{Operator: "SEQ_SCAN", Catalog: "memory", Schema: "source", Table: "payments", Projections: []string{"order_id", "payment_value"}},
	}
	if !reflect.DeepEqual(got.Scans, want) {
		t.Fatalf("scans = %#v, want %#v", got.Scans, want)
	}
}

func TestAnalyzeExplainKeepsRowPresenceScanWithoutProjection(t *testing.T) {
	input := []byte(`[
		{
			"name": "UNGROUPED_AGGREGATE",
			"children": [{
				"name": "SEQ_SCAN",
				"extra_info": {
					"Table": "memory.source.orders",
					"Estimated Cardinality": "1"
				}
			}]
		}
	]`)

	got, err := AnalyzeExplain(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Scans) != 1 {
		t.Fatalf("scan count = %d, want 1", len(got.Scans))
	}
	if got.Scans[0].Table != "orders" || len(got.Scans[0].Projections) != 0 {
		t.Fatalf("scan = %#v, want orders with no projections", got.Scans[0])
	}
}
