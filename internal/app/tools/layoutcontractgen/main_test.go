package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCopiesValidatedContractForBrowserBuilds(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "contracts.json")
	target := filepath.Join(temp, "generated", "contracts.json")
	if err := os.WriteFile(source, []byte(`{"version":1,"widgets":{"kpi":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate(source, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\"version\":1,\"widgets\":{\"kpi\":{}}}\n"; string(got) != want {
		t.Fatalf("generated contract = %q, want %q", got, want)
	}
}

func TestGenerateRejectsInvalidContract(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "contracts.json")
	if err := os.WriteFile(source, []byte(`{"version":2,"widgets":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate(source, filepath.Join(temp, "generated.json")); err == nil {
		t.Fatal("expected invalid contract error")
	}
}
