package ui

import (
	"html"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/access/http/mcpoauth"
)

func TestOAuthConsentPageUsesProductBranding(t *testing.T) {
	var output strings.Builder
	if err := OAuthConsentPage(mcpoauth.Consent{ClientName: "Agent", Resource: "https://example.test"}, nil, "csrf").Render(&output); err != nil {
		t.Fatal(err)
	}
	rendered := html.UnescapeString(output.String())
	for _, expected := range []string{
		"<title>Authorize MCP access · LeapView</title>",
		`<link rel="icon" href="/static/favicon.svg?v=dev" type="image/svg+xml">`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("document does not contain %q", expected)
		}
	}
}
