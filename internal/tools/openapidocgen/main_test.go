package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWritesOpenAPIBackedMarkdownPages(t *testing.T) {
	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "openapi.yaml")
	outDir := filepath.Join(tempDir, "docs")
	spec := `openapi: 3.0.0
info:
  title: Example API
  version: 1.2.3
  description: API used in the example.
tags:
  - name: Things
    description: Manage things.
paths:
  /v1/things:
    get:
      operationId: listThings
      summary: List things
      tags: [Things]
      parameters:
        - name: limit
          in: query
          required: false
          description: Maximum results.
          schema:
            type: integer
      responses:
        '200':
          description: Things returned.
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec fixture: %v", err)
	}

	if err := generate(specPath, outDir); err != nil {
		t.Fatalf("generate documentation: %v", err)
	}

	index := readGeneratedFile(t, filepath.Join(outDir, "index.md"))
	if strings.HasSuffix(index, "\n\n") {
		t.Errorf("index ends with an extra blank line: %q", index)
	}
	for _, want := range []string{"# API reference", "[Download the OpenAPI schema](/docs/openapi.yaml)", "[Things](/docs/api/things)"} {
		if !strings.Contains(index, want) {
			t.Errorf("index missing %q:\n%s", want, index)
		}
	}

	article := readGeneratedFile(t, filepath.Join(outDir, "things.md"))
	if strings.HasSuffix(article, "\n\n") {
		t.Errorf("article ends with an extra blank line: %q", article)
	}
	for _, want := range []string{"# Things", "## List things", "`GET /v1/things`", "| `limit` | query | No | integer | Maximum results. |", "| `200` | Things returned. |"} {
		if !strings.Contains(article, want) {
			t.Errorf("article missing %q:\n%s", want, article)
		}
	}

	catalog := readGeneratedFile(t, filepath.Join(outDir, "catalog.json"))
	if !strings.Contains(catalog, `"slug": "things"`) {
		t.Errorf("catalog missing Things document:\n%s", catalog)
	}
	if got := readGeneratedFile(t, filepath.Join(outDir, "openapi.yaml")); got != spec {
		t.Errorf("generated OpenAPI copy = %q, want source spec", got)
	}
}

func readGeneratedFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
