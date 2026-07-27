package module

import "testing"

func TestBuildRejectsMissingOwnedPersistence(t *testing.T) {
	if _, err := Build(t.Context(), Config{}); err == nil {
		t.Fatal("managed-data module accepted missing database")
	}
}

func TestBuildCanExposeExplicitlyDisabledSurface(t *testing.T) {
	module, err := Build(t.Context(), Config{Disabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if module.HTTP() == nil {
		t.Fatal("disabled managed-data module did not expose its unavailable handler")
	}
}
