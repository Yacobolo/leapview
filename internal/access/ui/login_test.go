package ui

import (
	"html"
	"strings"
	"testing"
)

func TestLoginPageUsesProductBranding(t *testing.T) {
	var output strings.Builder
	if err := LoginPage().Render(&output); err != nil {
		t.Fatal(err)
	}
	rendered := html.UnescapeString(output.String())
	for _, expected := range []string{
		"<title>LeapView Login</title>",
		`<link rel="icon" href="/static/favicon.svg?v=dev" type="image/svg+xml">`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("document does not contain %q", expected)
		}
	}
}

func TestLoginBootstrapUsesProductName(t *testing.T) {
	page := LoginBootstrapSignalsForOptions(LoginPageOptions{})["page"].(LoginPageSignal)
	if page.Title != "LeapView" || page.Kind != "login" {
		t.Fatalf("login page signal = %#v", page)
	}
}
