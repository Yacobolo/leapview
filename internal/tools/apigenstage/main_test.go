package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageCreatesIsolatedProjectsWithCanonicalVisualizationImport(t *testing.T) {
	apiDir := t.TempDir()
	for _, path := range []string{"typespec/main.tsp", "signals/main.tsp", "visualization/main.tsp"} {
		fullPath := filepath.Join(apiDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("// "+path+"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := stage(apiDir, "ui-signals"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(apiDir, ".apigen", "ui-signals")
	entry, err := os.ReadFile(filepath.Join(root, "main.tsp"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry), `./signals/main.tsp`) {
		t.Fatalf("entrypoint = %s", entry)
	}
	if _, err := os.Stat(filepath.Join(root, "visualization", "main.tsp")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "typespec")); !os.IsNotExist(err) {
		t.Fatalf("public API leaked into signal stage: %v", err)
	}
}
