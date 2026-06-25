package model

import (
	"strings"
	"testing"
)

func TestValidateRejectsAuthoredSourceReads(t *testing.T) {
	model := &Model{
		Name:        "test",
		Connections: map[string]Connection{"local_files": {Kind: "local"}},
		Sources: map[string]Source{
			"orders": {Connection: "local_files", Path: "orders.csv", Format: "csv"},
		},
		BaseTable: "orders",
		Tables: map[string]Table{
			"orders": {
				Sources:     []string{"orders"},
				SourceReads: map[string][]string{"orders": {"order_id"}},
				PrimaryKey:  "order_id",
				Dimensions:  map[string]MetricDimension{"order_id": {Label: "Order ID"}},
				Transform:   Transform{SQL: "SELECT order_id FROM source.orders"},
			},
		},
	}

	err := model.Validate()
	if err == nil || !strings.Contains(err.Error(), "source_reads is no longer supported") {
		t.Fatalf("Validate() error = %v, want source_reads rejection", err)
	}
}
