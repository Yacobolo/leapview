package module

import "testing"

func TestBuildRequiresDatabase(t *testing.T) {
	if _, err := Build(t.Context(), Config{}); err == nil {
		t.Fatal("serving-state module accepted a missing database")
	}
}
