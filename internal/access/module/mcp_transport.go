package module

import (
	"net/http"

	"github.com/Yacobolo/leapview/internal/access"
)

type MCPIdentity struct {
	PrincipalID string
	DevBypass   bool
	Credential  access.APICredential
	Restricted  bool
}

func (m *Module) MCPIdentity(r *http.Request) (MCPIdentity, bool) {
	if m == nil || m.auth == nil {
		return MCPIdentity{}, false
	}
	principal, ok := m.auth.Principal(r)
	if !ok {
		return MCPIdentity{}, false
	}
	identity := MCPIdentity{PrincipalID: principal.ID, DevBypass: principal.DevBypass}
	if credential, ok := m.auth.APICredential(r); ok {
		identity.Credential = credential
		identity.Restricted = credential.Token.Privileges != nil
	}
	return identity, true
}

func (m *Module) ProtectMCP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || m.auth == nil || m.oauthResource == nil {
			if m != nil && m.oauthResource != nil {
				m.oauthResource.Challenge(w)
			} else {
				WriteBearerChallenge(w, r)
			}
			return
		}
		r = r.WithContext(access.WithAuthorizationCache(r.Context()))
		var principal Principal
		if m.auth.DevBypass() && m.auth.AcceptsPublicBearer(r) {
			principal = LocalDeveloperPrincipal()
		} else {
			credential, err := m.oauthResource.Authenticate(r.Context(), BearerToken(r))
			if err != nil {
				m.oauthResource.Challenge(w)
				return
			}
			principal = Principal{
				ID: credential.Principal.ID, Email: credential.Principal.Email,
				DisplayName: credential.Principal.DisplayName,
			}
		}
		r = r.WithContext(WithPrincipal(r.Context(), principal))
		if !principal.DevBypass {
			allowed, err := m.authorizeAnyWorkspace(r.Context(), principal.ID, nil, access.PrivilegeUseAgent)
			if err != nil {
				WriteAuthError(w, r, err, http.StatusInternalServerError)
				return
			}
			if !allowed {
				WriteAuthError(w, r, ErrForbidden, http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
