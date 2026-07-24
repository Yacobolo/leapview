package ui

import (
	"net/url"
	"strings"

	"github.com/Yacobolo/leapview/internal/platform/web/staticasset"
	"github.com/Yacobolo/leapview/pkg/pagestream"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type LoginPageOptions struct {
	LocalAuth          bool
	SSOAuth            bool
	MustChangePassword bool
	ProviderLabel      string
	CSRFToken          string
}

type LoginPageSignal struct {
	BackgroundModuleSrc string `json:"backgroundModuleSrc"`
	Kind                string `json:"kind"`
	LocalAuth           bool   `json:"localAuth"`
	MustChangePassword  bool   `json:"mustChangePassword"`
	ProviderLabel       string `json:"providerLabel"`
	SSOAuth             bool   `json:"ssoAuth"`
	Title               string `json:"title"`
}

type StatusSignal struct {
	Error           string  `json:"error"`
	Generation      int64   `json:"generation"`
	LastUpdated     string  `json:"lastUpdated"`
	Loading         bool    `json:"loading"`
	ProgressPercent float64 `json:"progressPercent"`
	RefreshID       string  `json:"refreshId"`
	SetupRequired   bool    `json:"setupRequired"`
}

func LoginPage(options ...LoginPageOptions) g.Node {
	opts := normalizedLoginOptions(options)
	return pagestream.RenderPage(pagestream.PageSpec{
		Title:             "LeapView Login",
		DatastarScriptURL: staticasset.URL(staticasset.DatastarScriptPath),
		HTMLAttrs: []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		},
		Head: []g.Node{
			csrfMeta(opts.CSRFToken),
			h.Link(h.Rel("icon"), h.Href(staticasset.URL("/static/favicon.svg")), h.Type("image/svg+xml")),
			h.Link(h.Rel("stylesheet"), h.Href(staticasset.URL("/static/app.css"))),
			h.Script(h.Src(staticasset.URL("/static/theme.js"))),
			h.Script(h.Type("module"), h.Src(staticasset.URL("/static/login-page.js"))),
			h.Script(h.Type("module"), h.Src(staticasset.URL("/static/login-background-loader.js"))),
			inspectorScript(),
		},
		MainAttrs:  []g.Node{h.Class("min-h-svh bg-app text-fg-default")},
		UpdatesURL: loginUpdatesURL(),
		Body: []g.Node{
			g.El("lv-login-page", g.Attr("background-module-src", staticasset.URL("/static/topology-background.js"))),
			inspectorElement(),
		},
	})
}

func LoginBootstrapSignalsForOptions(options LoginPageOptions) map[string]any {
	opts := normalizedLoginOptions([]LoginPageOptions{options})
	return map[string]any{
		"page": LoginPageSignal{
			BackgroundModuleSrc: staticasset.URL("/static/topology-background.js"),
			Kind:                "login", LocalAuth: opts.LocalAuth, MustChangePassword: opts.MustChangePassword,
			ProviderLabel: opts.ProviderLabel, SSOAuth: opts.SSOAuth, Title: "LeapView",
		},
		"status": StatusSignal{},
	}
}

func normalizedLoginOptions(options []LoginPageOptions) LoginPageOptions {
	opts := LoginPageOptions{SSOAuth: true, ProviderLabel: "Sign in with Azure Active Directory"}
	if len(options) > 0 {
		opts = options[0]
		if strings.TrimSpace(opts.ProviderLabel) == "" {
			opts.ProviderLabel = "Sign in with Azure Active Directory"
		}
	}
	return opts
}

func loginUpdatesURL() string {
	values := url.Values{}
	values.Set("route", "login")
	return "/updates?" + values.Encode()
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
