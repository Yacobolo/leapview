package ui

import (
	"github.com/flidai/leapview/internal/deployment"
	webpage "github.com/flidai/leapview/internal/platform/web/page"
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

func CandidatePage(candidate deployment.Candidate, providers ...webpage.Provider) g.Node {
	layout := webpage.Resolve(firstProvider(providers), webpage.Context{
		Active: "dashboards", SectionTitle: "Development", PageTitle: "Private candidate",
	})
	return c.HTML5(c.HTML5Props{
		Title: "Private candidate · " + layout.Presentation.ProductName, Language: "en",
		Head: g.Group{
			h.Link(h.Rel("icon"), h.Href(layout.Assets.URL(layout.Presentation.FaviconPath)), h.Type("image/svg+xml")),
			h.Link(h.Rel("stylesheet"), h.Href(layout.Assets.URL("/static/app.css"))),
			g.If(candidate.Status == deployment.CandidatePreparing,
				h.Meta(g.Attr("http-equiv", "refresh"), h.Content("1")),
			),
		},
		Body: g.Group{
			h.Main(h.Class("min-h-svh bg-app text-fg-default flex items-center justify-center p-6"),
				h.Section(h.Class("w-full max-w-lg rounded-xl border border-border-default bg-canvas-default p-6 shadow-lg"),
					g.Attr("aria-live", "polite"),
					h.H1(h.Class("text-xl font-semibold"), g.Text("Private candidate")),
					h.P(h.Class("mt-3 text-sm text-fg-muted"), g.Text(candidateStatusMessage(candidate.Status))),
					h.Dl(h.Class("mt-5 grid gap-3 text-sm"),
						h.Div(h.Dt(h.Class("text-fg-subtle"), g.Text("Candidate")), h.Dd(h.Class("font-mono"), g.Text(candidate.ID))),
						h.Div(h.Dt(h.Class("text-fg-subtle"), g.Text("Status")), h.Dd(g.Text(string(candidate.Status)))),
						g.If(candidate.FailureReason != "",
							h.Div(h.Dt(h.Class("text-fg-subtle"), g.Text("Diagnostic code")), h.Dd(h.Class("font-mono"), g.Text(candidate.FailureReason))),
						),
					),
				),
			),
		},
	})
}

func candidateStatusMessage(status deployment.CandidateStatus) string {
	switch status {
	case deployment.CandidatePreparing:
		return "LeapView is preparing this private preview. This page will become the governed dashboard runtime when preparation completes."
	case deployment.CandidateReady:
		return "This private candidate is ready. The governed runtime is being attached."
	case deployment.CandidateFailed:
		return "LeapView could not prepare this candidate. Your active dashboards were not changed."
	case deployment.CandidateCancelled:
		return "This private candidate was cancelled and cannot affect active dashboards."
	case deployment.CandidateExpired:
		return "This private candidate expired and cannot affect active dashboards."
	default:
		return "This private candidate is unavailable."
	}
}

func firstProvider(providers []webpage.Provider) webpage.Provider {
	if len(providers) == 0 {
		return nil
	}
	return providers[0]
}
