package page

import (
	"strings"

	"github.com/Yacobolo/leapview/internal/platform/web/staticasset"
	"github.com/Yacobolo/leapview/pkg/pagestream"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

const RootClass = "min-h-svh bg-app text-fg-default"

// Context is the product-agnostic information a layout needs to present the
// current route. Capabilities provide the values; application composition
// decides how they are rendered.
type Context struct {
	Active       string
	ScopeID      string
	ScopeTitle   string
	SectionID    string
	SectionTitle string
	PageID       string
	PageTitle    string
	RelatedID    string
	RelatedTitle string
	HistoryID    string
	Compact      bool
}

type Presentation struct {
	ProductName string
	FaviconPath string
}

// Layout is injected by application composition. Signal is deliberately
// opaque: platform web owns the mechanism, while app owns the product chrome.
type Layout struct {
	Presentation Presentation
	Signal       any
	Scripts      []string
	Mount        func(g.Node, ...g.Node) g.Node
}

type Provider func(Context) Layout

func AssetURL(path string) string {
	return staticasset.URL(path)
}

type Spec struct {
	Title        string
	CSRFToken    string
	Stylesheets  []string
	Scripts      []string
	Head         []g.Node
	HTMLAttrs    []g.Node
	MainAttrs    []g.Node
	UpdatesURL   string
	Content      g.Node
	ContentAttrs []g.Node
	BodyBefore   []g.Node
	BodyAfter    []g.Node
}

func Resolve(provider Provider, context Context) Layout {
	if provider == nil {
		return Layout{}
	}
	return provider(context)
}

func WithSignal(layout Layout, signals map[string]any) map[string]any {
	if layout.Signal != nil {
		signals["chrome"] = layout.Signal
	}
	return signals
}

func Render(layout Layout, spec Spec) g.Node {
	title := strings.TrimSpace(spec.Title)
	if title == "" {
		title = strings.TrimSpace(layout.Presentation.ProductName)
	}
	head := make([]g.Node, 0, 8+len(layout.Scripts)+len(spec.Stylesheets)+len(spec.Scripts)+len(spec.Head))
	if favicon := strings.TrimSpace(layout.Presentation.FaviconPath); favicon != "" {
		head = append(head, h.Link(h.Rel("icon"), h.Href(staticasset.URL(favicon)), h.Type("image/svg+xml")))
	}
	head = append(head,
		h.Link(h.Rel("stylesheet"), h.Href(staticasset.URL("/static/app.css"))),
		h.Script(h.Src(staticasset.URL("/static/theme.js"))),
		h.Script(h.Type("module"), h.Src(staticasset.URL("/static/command.js"))),
	)
	head = append(head, csrfMeta(spec.CSRFToken))
	for _, path := range spec.Stylesheets {
		head = append(head, h.Link(h.Rel("stylesheet"), h.Href(staticasset.URL(path))))
	}
	for _, path := range append(append([]string(nil), layout.Scripts...), spec.Scripts...) {
		head = append(head, h.Script(h.Type("module"), h.Src(staticasset.URL(path))))
	}
	head = append(head, spec.Head...)
	head = append(head, inspectorScript())

	content := spec.Content
	if layout.Mount != nil {
		content = layout.Mount(content, spec.ContentAttrs...)
	} else if len(spec.ContentAttrs) > 0 && content != nil {
		content = g.Group{g.El("div", append(spec.ContentAttrs, content)...)}
	}
	body := append([]g.Node(nil), spec.BodyBefore...)
	body = append(body, content)
	body = append(body, spec.BodyAfter...)
	body = append(body, inspectorElement())
	htmlAttrs := spec.HTMLAttrs
	if len(htmlAttrs) == 0 {
		htmlAttrs = []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		}
	}
	mainAttrs := spec.MainAttrs
	if len(mainAttrs) == 0 {
		mainAttrs = []g.Node{h.Class(RootClass)}
	}
	return pagestream.RenderPage(pagestream.PageSpec{
		Title: title, DatastarScriptURL: staticasset.URL(staticasset.DatastarScriptPath),
		HTMLAttrs: htmlAttrs, Head: head, MainAttrs: mainAttrs,
		UpdatesURL: spec.UpdatesURL, Body: body,
	})
}

func csrfMeta(token string) g.Node {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return h.Meta(h.Name("csrf-token"), h.Content(token))
}

func inspectorScript() g.Node {
	if staticasset.Production() {
		return nil
	}
	return h.Script(h.Type("module"), h.Src(staticasset.URL("/static/datastar-inspector.js")))
}

func inspectorElement() g.Node {
	if staticasset.Production() {
		return nil
	}
	return g.El("datastar-inspector", g.Attr("signals-url", "/__dev/pagestream/signals"))
}
