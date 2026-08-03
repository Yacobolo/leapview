package module

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/flidai/leapview/internal/access"
	accessui "github.com/flidai/leapview/internal/access/ui"
	"github.com/gorilla/csrf"
)

func (m *Module) DeviceAuthorizationPage(w http.ResponseWriter, r *http.Request) {
	if m == nil || m.handler.AuthoringAuth == nil {
		http.Error(w, "authoring authentication is unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		m.renderDeviceAuthorizationPage(w, r, r.URL.Query().Get("user_code"))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, credential, ok := m.browserAuthoringPrincipal(r)
	if !ok {
		http.Error(w, "authenticated browser session is required", http.StatusUnauthorized)
		return
	}
	if credential {
		http.Error(w, "device authorization requires a browser session", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid device authorization form", http.StatusBadRequest)
		return
	}
	userCode := strings.TrimSpace(r.Form.Get("user_code"))
	if userCode == "" {
		http.Error(w, "device code is required", http.StatusBadRequest)
		return
	}
	approved := r.Form.Get("decision") == "approve"
	actor := access.Principal{
		ID: principal.ID, Kind: access.PrincipalKindUser, Email: principal.Email, DisplayName: principal.DisplayName,
	}
	var err error
	if approved {
		err = m.handler.AuthoringAuth.ApproveDeviceAuthorization(r.Context(), actor, userCode)
	} else {
		err = m.handler.AuthoringAuth.DenyDeviceAuthorization(r.Context(), actor, userCode)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("device authorization failed: %v", err), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := accessui.DeviceAuthorizationResultPage(approved, m.presentation, m.assets).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *Module) renderDeviceAuthorizationPage(w http.ResponseWriter, r *http.Request, userCode string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := accessui.DeviceAuthorizationPage(accessui.DeviceAuthorizationPageOptions{
		UserCode: userCode, CSRFToken: csrf.Token(r), Presentation: m.presentation, Assets: m.assets,
	}).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *Module) browserAuthoringPrincipal(r *http.Request) (Principal, bool, bool) {
	if m.auth == nil {
		return LocalDeveloperPrincipal(), false, true
	}
	principal, ok := m.auth.Principal(r)
	if !ok {
		return Principal{}, false, false
	}
	_, bearer := m.auth.APICredential(r)
	return principal, bearer, true
}
