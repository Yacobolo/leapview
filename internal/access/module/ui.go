package module

import (
	"net/http"

	"github.com/gorilla/csrf"
)

func (m *Module) CSRFToken(r *http.Request) string {
	if m == nil || m.auth == nil {
		return ""
	}
	return csrf.Token(r)
}

func (m *Module) CurrentRoleLabel(r *http.Request) string {
	if m == nil || m.auth == nil {
		return "Local"
	}
	principal, ok := m.auth.Principal(r)
	if !ok {
		return "Signed out"
	}
	if principal.DevBypass {
		return "Platform admin"
	}
	return "Platform access"
}
