package module

import (
	"net/http"

	"github.com/Yacobolo/leapview/internal/workspace/ui"
	"github.com/gorilla/csrf"
)

// Login renders the access-owned authentication entrypoint.
func (m *Module) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := ui.LoginPage(m.LoginPageOptions(r)).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *Module) LoginPageOptions(r *http.Request) ui.LoginPageOptions {
	options := ui.LoginPageOptions{
		LocalAuth:     m != nil && m.auth != nil && m.auth.LocalAuthEnabled(),
		SSOAuth:       m == nil || m.auth == nil || m.auth.SSOConfigured(),
		ProviderLabel: "Sign in with Azure Active Directory",
	}
	if m == nil || m.auth == nil {
		return options
	}
	options.CSRFToken = csrf.Token(r)
	if principal, _, ok := m.auth.Authenticate(r); ok {
		options.MustChangePassword = m.auth.MustChangeLocalPassword(r, principal.ID)
	}
	return options
}

func (m *Module) Begin(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.auth == nil {
		http.NotFound(w, r)
		return
	}
	m.auth.Begin(w, r)
}

func (m *Module) Callback(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.auth == nil {
		http.NotFound(w, r)
		return
	}
	m.auth.Callback(w, r)
}

func (m *Module) LocalLogin(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.auth == nil {
		http.NotFound(w, r)
		return
	}
	m.auth.LocalLogin(w, r)
}

func (m *Module) LocalPassword(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.auth == nil {
		http.NotFound(w, r)
		return
	}
	m.auth.LocalPassword(w, r)
}

func (m *Module) Logout(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.auth == nil {
		http.NotFound(w, r)
		return
	}
	m.auth.Logout(w, r)
}
