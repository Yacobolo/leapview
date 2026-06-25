package configschema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateBytesRejectsUnknownField(t *testing.T) {
	err := ValidateBytes(KindCatalog, "catalog.yaml", []byte(`
workspace:
  id: libredash
semantic_models:
  - id: olist
    title: Olist
    path: model.yaml
dashboards: []
surprise: true
`))
	assertDiagnostic(t, err, "schema.unknown_field", "field not allowed")
}

func TestValidateBytesRejectsWrongType(t *testing.T) {
	err := ValidateBytes(KindCatalog, "catalog.yaml", []byte(`
semantic_models:
  - id: 12
    title: Olist
    path: model.yaml
dashboards: []
`))
	assertDiagnostic(t, err, "schema.type", "mismatched types")
}

func TestValidateBytesRejectsUnsupportedEnum(t *testing.T) {
	err := ValidateBytes(KindDashboard, "dashboard.yaml", []byte(`
id: sales
title: Sales
semantic_model: olist
visuals:
  revenue:
    type: volcano
    query:
      measures:
        revenue:
pages:
  - id: overview
    title: Overview
    visuals: []
`))
	assertDiagnostic(t, err, "schema.enum", "type")
}

func TestValidateBytesRejectsInvalidIdentifierKey(t *testing.T) {
	err := ValidateBytes(KindSemanticModel, "model.yaml", []byte(`
name: olist
sources:
  invalid-name:
    connection: olist
    path: orders.csv
models:
  orders:
    source: invalid-name
    primary_key: order_id
semantic_models:
  olist:
    base_table: orders
    tables: [orders]
`))
	assertDiagnostic(t, err, "schema.contract", "invalid-name")
}

func TestValidateFileAcceptsOlistContracts(t *testing.T) {
	root := filepath.Join("..", "..")
	tests := []struct {
		kind Kind
		path string
	}{
		{KindCatalog, filepath.Join(root, "dashboards", "catalog.yaml")},
		{KindSemanticModel, filepath.Join(root, "dashboards", "olist", "model.yaml")},
		{KindDashboard, filepath.Join(root, "dashboards", "olist", "executive-sales.yaml")},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if err := ValidateFile(tt.kind, tt.path); err != nil {
				t.Fatalf("ValidateFile() error = %v", err)
			}
		})
	}
}

func TestJSONSchemaFilesAreFresh(t *testing.T) {
	files, err := JSONSchemaFiles()
	if err != nil {
		t.Fatalf("JSONSchemaFiles() error = %v", err)
	}
	for name, content := range files {
		path := filepath.Join("..", "..", "schemas", "json", name)
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read generated schema %s: %v", name, err)
		}
		if string(onDisk) != string(content) {
			t.Fatalf("%s is stale; run libredash schema export --format json-schema --out schemas/json", path)
		}
	}
}

func assertDiagnostic(t *testing.T, err error, code, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ValidateBytes() error = nil, want %s", code)
	}
	var schemaErr *Error
	if !errors.As(err, &schemaErr) {
		t.Fatalf("error type = %T, want *Error: %v", err, err)
	}
	if len(schemaErr.Diagnostics) == 0 {
		t.Fatal("diagnostics empty")
	}
	got := schemaErr.Diagnostics[0]
	if got.Code != code {
		t.Fatalf("diagnostic code = %q, want %q: %#v", got.Code, code, schemaErr.Diagnostics)
	}
	if got.File == "" || got.Line == 0 || got.Column == 0 {
		t.Fatalf("diagnostic lacks source position: %#v", got)
	}
	if !strings.Contains(got.Message, contains) {
		t.Fatalf("diagnostic message = %q, want containing %q", got.Message, contains)
	}
}
