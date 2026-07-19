package siteassets

import (
	"strings"
	"testing"
)

func TestIntegrationLogoReturnsTrustedVendoredSVG(t *testing.T) {
	logo, err := IntegrationLogo("apacheiceberg")
	if err != nil {
		t.Fatalf("read integration logo: %v", err)
	}
	for _, expected := range []string{"<svg", "main-text-secondary-color", "main-text-tertiary-color"} {
		if !strings.Contains(logo, expected) {
			t.Errorf("integration logo does not contain %q", expected)
		}
	}
}

func TestIntegrationLogoRejectsPathTraversal(t *testing.T) {
	if _, err := IntegrationLogo("../favicon"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}
