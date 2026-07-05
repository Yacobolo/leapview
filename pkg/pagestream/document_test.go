package pagestream

import (
	"bytes"
	"strings"
	"testing"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func TestRenderDocumentIncludesSignalsInitMainAttrsAndBody(t *testing.T) {
	var body bytes.Buffer
	err := RenderDocument(DocumentSpec{
		Title:     "Test Page",
		HTMLAttrs: []g.Node{g.Attr("data-color-mode", "auto")},
		Head:      []g.Node{h.Meta(g.Attr("name", "test-head"))},
		MainAttrs: []g.Node{h.ID("root"), h.Class("app-shell")},
		Signals:   map[string]any{"runtime": map[string]any{"updatesUrl": "/updates?route=test"}},
		Init:      []string{"$ready = true", "@get($runtime.updatesUrl)"},
		Body:      []g.Node{h.Div(h.ID("content"), g.Text("Hello"))},
	}).Render(&body)
	if err != nil {
		t.Fatalf("render document: %v", err)
	}
	html := body.String()
	for _, want := range []string{
		"<title>Test Page</title>",
		`data-color-mode="auto"`,
		`<main id="root" class="app-shell"`,
		`data-signals=`,
		`updatesUrl`,
		`data-init="$ready = true; @get($runtime.updatesUrl)"`,
		`<div id="content">Hello</div>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered document missing %q:\n%s", want, html)
		}
	}
}

func TestInitExpressionSkipsEmptyExpressions(t *testing.T) {
	if got, want := InitExpression("", "$a = 1", "", "@get('/updates')"), "$a = 1; @get('/updates')"; got != want {
		t.Fatalf("init expression = %q, want %q", got, want)
	}
}
