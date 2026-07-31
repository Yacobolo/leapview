package module

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/flidai/leapview/internal/access"
)

const authoringDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

func (m *Module) AuthoringDeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	service, ok := m.authoringOAuthService(w)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeAuthoringOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid device authorization request")
		return
	}
	if r.Form.Get("client_id") != access.AuthoringCLIClientID {
		writeAuthoringOAuthError(w, http.StatusUnauthorized, "invalid_client", "unknown device client")
		return
	}
	privileges, err := authoringOAuthPrivileges(r.Form.Get("scope"))
	if err != nil {
		writeAuthoringOAuthError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	scope, err := access.NewAuthoringScope(service.InstanceID(), r.Form.Get("project_id"), privileges)
	if err != nil {
		writeAuthoringOAuthError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	response, err := service.BeginDeviceAuthorization(r.Context(), scope)
	if err != nil {
		writeAuthoringOAuthServiceError(w, err)
		return
	}
	setAuthoringOAuthNoStore(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               response.DeviceCode,
		"user_code":                 response.UserCode,
		"verification_uri":          response.VerificationURI,
		"verification_uri_complete": response.VerificationURIComplete,
		"expires_in":                response.ExpiresIn,
		"interval":                  response.Interval,
	})
}

func (m *Module) AuthoringOAuthToken(w http.ResponseWriter, r *http.Request) {
	service, ok := m.authoringOAuthService(w)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeAuthoringOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid token request")
		return
	}
	if r.Form.Get("client_id") != access.AuthoringCLIClientID {
		writeAuthoringOAuthError(w, http.StatusUnauthorized, "invalid_client", "unknown authoring client")
		return
	}
	if r.Form.Get("grant_type") != authoringDeviceGrantType {
		writeAuthoringOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported authoring grant type")
		return
	}
	tokens, err := service.ExchangeDeviceCode(r.Context(), r.Form.Get("device_code"))
	if err != nil {
		writeAuthoringOAuthServiceError(w, err)
		return
	}
	privileges := make([]string, len(tokens.Session.Scope.Privileges))
	for index, privilege := range tokens.Session.Scope.Privileges {
		privileges[index] = string(privilege)
	}
	setAuthoringOAuthNoStore(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"token_type":    tokens.TokenType,
		"expires_in":    tokens.ExpiresIn,
		"session_id":    tokens.Session.ID,
		"session_kind":  tokens.Session.Kind,
		"target_id":     tokens.Session.Scope.TargetID,
		"project_id":    tokens.Session.Scope.ProjectID,
		"scope":         strings.Join(privileges, " "),
	})
}

type authoringOAuthAuthentication interface {
	InstanceID() string
	BeginDeviceAuthorization(context.Context, access.AuthoringScope) (access.DeviceAuthorizationResponse, error)
	ExchangeDeviceCode(context.Context, string) (access.AuthoringTokenSet, error)
}

func (m *Module) authoringOAuthService(w http.ResponseWriter) (authoringOAuthAuthentication, bool) {
	if m == nil || m.handler.AuthoringAuth == nil {
		writeAuthoringOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "authoring authentication is unavailable")
		return nil, false
	}
	return m.handler.AuthoringAuth, true
}

func authoringOAuthPrivileges(scope string) ([]access.Privilege, error) {
	values := strings.Fields(scope)
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one authoring privilege is required")
	}
	privileges := make([]access.Privilege, 0, len(values))
	for _, value := range values {
		privilege, ok := access.ParsePrivilege(value)
		if !ok {
			return nil, fmt.Errorf("unsupported authoring privilege %q", value)
		}
		privileges = append(privileges, privilege)
	}
	return privileges, nil
}

func writeAuthoringOAuthServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, access.ErrDeviceAuthorizationPending):
		writeAuthoringOAuthError(w, http.StatusBadRequest, "authorization_pending", "device authorization is pending")
	case errors.Is(err, access.ErrDeviceAuthorizationSlowDown):
		writeAuthoringOAuthError(w, http.StatusBadRequest, "slow_down", "device authorization polling is too frequent")
	case errors.Is(err, access.ErrDeviceAuthorizationDenied):
		writeAuthoringOAuthError(w, http.StatusBadRequest, "access_denied", "device authorization was denied")
	case errors.Is(err, access.ErrDeviceAuthorizationExpired):
		writeAuthoringOAuthError(w, http.StatusBadRequest, "expired_token", "device authorization expired")
	case errors.Is(err, access.ErrAuthoringScopeDenied):
		writeAuthoringOAuthError(w, http.StatusBadRequest, "invalid_scope", "authoring scope was denied")
	case errors.Is(err, access.ErrInvalidAuthoringCredential):
		writeAuthoringOAuthError(w, http.StatusBadRequest, "invalid_grant", "device credential is invalid")
	default:
		writeAuthoringOAuthError(w, http.StatusInternalServerError, "server_error", "authoring token exchange failed")
	}
}

func writeAuthoringOAuthError(w http.ResponseWriter, status int, code, description string) {
	setAuthoringOAuthNoStore(w)
	writeJSON(w, status, map[string]string{
		"error":             code,
		"error_description": description,
	})
}

func setAuthoringOAuthNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
