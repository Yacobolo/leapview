package ui

import (
	"html"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/access/http/mcpoauth"
	webpage "github.com/flidai/leapview/internal/platform/web/page"
	"github.com/flidai/leapview/internal/platform/web/staticasset"
)

func TestOAuthConsentPageUsesProductBranding(t *testing.T) {
	var output strings.Builder
	if err := OAuthConsentPage(mcpoauth.Consent{ClientName: "Agent", Resource: "https://example.test"}, nil, "csrf",
		webpage.Presentation{ProductName: "LeapView", FaviconPath: "/static/favicon.svg"}, staticasset.Resolver{}).Render(&output); err != nil {
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
