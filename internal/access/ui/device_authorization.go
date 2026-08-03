package ui

import (
	"strings"

	webpage "github.com/flidai/leapview/internal/platform/web/page"
	"github.com/flidai/leapview/internal/platform/web/staticasset"
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

type DeviceAuthorizationPageOptions struct {
	UserCode     string
	CSRFToken    string
	Presentation webpage.Presentation
	Assets       staticasset.Resolver
}

func DeviceAuthorizationPage(options DeviceAuthorizationPageOptions) g.Node {
	presentation := normalizedDevicePresentation(options.Presentation)
	return c.HTML5(c.HTML5Props{
		Title: "Authorize CLI · " + presentation.ProductName, Language: "en",
		Head: deviceAuthorizationHead(presentation, options.Assets),
		Body: g.Group{
			h.Main(h.Class("min-h-svh bg-app text-fg-default flex items-center justify-center p-6"),
				h.Section(h.Class("w-full max-w-lg rounded-xl border border-border-default bg-canvas-default p-6 shadow-lg"),
					h.H1(h.Class("text-xl font-semibold"), g.Text("Authorize LeapView CLI")),
					h.P(h.Class("mt-3 text-sm text-fg-muted"),
						g.Text("Approve project publishing from the terminal where you started this sign-in.")),
					h.Form(h.Method("post"), h.Action("/device"),
						h.Div(h.Class("mt-5 rounded-md border border-border-muted bg-canvas-subtle p-4"),
							h.Label(h.For("user_code"), h.Class("text-sm font-medium"), g.Text("Device code")),
							h.Input(h.ID("user_code"), h.Name("user_code"), h.Value(strings.TrimSpace(options.UserCode)),
								h.AutoComplete("one-time-code"), h.Class("mt-2 w-full rounded-md border border-border-default bg-canvas-default px-3 py-2 font-mono text-lg tracking-wide")),
							h.P(h.Class("mt-3 text-xs text-fg-subtle"),
								g.Text("Approval grants only the target, project, actions, and lifetime requested by the CLI.")),
						),
						h.Input(h.Type("hidden"), h.Name("gorilla.csrf.Token"), h.Value(options.CSRFToken)),
						h.Div(h.Class("mt-6 flex justify-end gap-3"),
							h.Button(h.Type("submit"), h.Name("decision"), h.Value("deny"), h.Class("btn"), g.Text("Deny")),
							h.Button(h.Type("submit"), h.Name("decision"), h.Value("approve"), h.Class("btn btn-accent"), g.Text("Authorize")),
						),
					),
				),
			),
		},
	})
}

func DeviceAuthorizationResultPage(approved bool, presentation webpage.Presentation, assets staticasset.Resolver) g.Node {
	presentation = normalizedDevicePresentation(presentation)
	title := "CLI request denied"
	message := "The request was denied. Return to your terminal to finish."
	if approved {
		title = "CLI authorized"
		message = "Authorization is complete. Return to your terminal to continue."
	}
	return c.HTML5(c.HTML5Props{
		Title: title + " · " + presentation.ProductName, Language: "en",
		Head: deviceAuthorizationHead(presentation, assets),
		Body: g.Group{
			h.Main(h.Class("min-h-svh bg-app text-fg-default flex items-center justify-center p-6"),
				h.Section(h.Class("w-full max-w-lg rounded-xl border border-border-default bg-canvas-default p-6 shadow-lg"),
					h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
					h.P(h.Class("mt-3 text-sm text-fg-muted"), g.Text(message)),
				),
			),
		},
	})
}

func normalizedDevicePresentation(presentation webpage.Presentation) webpage.Presentation {
	if strings.TrimSpace(presentation.ProductName) == "" {
		presentation.ProductName = "LeapView"
	}
	if strings.TrimSpace(presentation.FaviconPath) == "" {
		presentation.FaviconPath = "/static/favicon.svg"
	}
	return presentation
}

func deviceAuthorizationHead(presentation webpage.Presentation, assets staticasset.Resolver) g.Group {
	return g.Group{
		h.Link(h.Rel("icon"), h.Href(assets.URL(presentation.FaviconPath)), h.Type("image/svg+xml")),
		h.Link(h.Rel("stylesheet"), h.Href(assets.URL("/static/app.css"))),
	}
}
