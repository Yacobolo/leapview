package module

import "testing"

func TestBuildConstructsOwnedHTTPHandlerWithoutStorageAdapters(t *testing.T) {
	module, err := Build(t.Context(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if module.HTTP() == nil {
		t.Fatal("expected managed-data module to construct its HTTP handler")
	}
}
