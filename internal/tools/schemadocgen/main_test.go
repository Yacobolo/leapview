package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWritesACompleteSchemaReference(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "schemas")
	exampleDir := filepath.Join(root, "examples")
	outDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(exampleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "project.schema.json"), []byte(`{
  "type": "object",
  "properties": {
    "apiVersion": {"const": "libredash.dev/v1"},
    "kind": {"const": "Project"},
    "metadata": {"$ref": "#/$defs/Metadata"}
  },
  "required": ["apiVersion", "kind", "metadata"],
  "$defs": {
    "Metadata": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exampleDir, "libredash.yaml"), []byte("apiVersion: libredash.dev/v1\nkind: Project\nmetadata:\n  name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := generate(schemaDir, exampleDir, outDir); err != nil {
		t.Fatalf("generate schema reference: %v", err)
	}

	article, err := os.ReadFile(filepath.Join(outDir, "project.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Project configuration", "## Example", "kind: Project", "## Fields", "`metadata`", "## Nested definitions", "### Metadata", "`name`"} {
		if !strings.Contains(string(article), want) {
			t.Errorf("generated article missing %q:\n%s", want, article)
		}
	}
	catalog, err := os.ReadFile(filepath.Join(outDir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(catalog), `"slug": "project"`) {
		t.Errorf("generated catalog missing project: %s", catalog)
	}
	if _, err := os.Stat(filepath.Join(outDir, "schemas", "project.schema.json")); err != nil {
		t.Errorf("generated schema download missing: %v", err)
	}
}
