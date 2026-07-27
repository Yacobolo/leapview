package module

import (
	"context"
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/access/httpauth"
)

func (m *Module) Protect(privilege access.Privilege, handler http.HandlerFunc) http.HandlerFunc {
	return m.ProtectHandler(privilege, handler).ServeHTTP
}

func (m *Module) ProtectWithObjects(privilege access.Privilege, resolver func(*http.Request, string) []access.ObjectRef, handler http.HandlerFunc) http.HandlerFunc {
	return m.ProtectHandlerWithObjects(privilege, resolver, handler).ServeHTTP
}

func (m *Module) ProtectAnyWorkspace(privilege access.Privilege, handler http.HandlerFunc) http.HandlerFunc {
	return m.protectAnyWorkspace(privilege, handler).ServeHTTP
}

func (m *Module) ProtectGlobal(privilege access.Privilege, handler http.HandlerFunc) http.HandlerFunc {
	return m.protectAnyWorkspace(privilege, handler).ServeHTTP
}

func (m *Module) ProtectHandler(privilege access.Privilege, next http.Handler) http.Handler {
	return m.ProtectHandlerWithObjects(privilege, nil, next)
}

func (m *Module) ProtectNamed(privilege string, next http.Handler) http.Handler {
	return m.ProtectHandler(access.Privilege(privilege), next)
}

func (m *Module) ProtectGlobalNamed(privilege string, next http.Handler) http.Handler {
	return m.protectAnyWorkspace(access.Privilege(privilege), next)
}

func (m *Module) ProtectAnyWorkspaceNamed(privilege string, next http.Handler) http.Handler {
	return m.protectAnyWorkspace(access.Privilege(privilege), next)
}

func (m *Module) ProtectViewItem(handler http.HandlerFunc) http.HandlerFunc {
	return m.Protect(access.PrivilegeViewItem, handler)
}

func (m *Module) ProtectIngestData(next http.Handler) http.Handler {
	return m.ProtectHandler(access.PrivilegeIngestData, next)
}

func (m *Module) ProtectHandlerWithObjects(privilege access.Privilege, resolver func(*http.Request, string) []access.ObjectRef, next http.Handler) http.Handler {
	if m == nil || m.auth == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), LocalDeveloperPrincipal())))
		})
	}
	return m.auth.MiddlewareWithObjectResolver(privilege, httpauth.ObjectResolver(resolver), next)
}

func (m *Module) CSRFMiddleware(next http.Handler) http.Handler {
	if m == nil || m.auth == nil {
		return next
	}
	return m.auth.CSRFMiddleware(next)
}

func (m *Module) protectAnyWorkspace(privilege access.Privilege, next http.Handler) http.Handler {
	if m == nil || m.auth == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), LocalDeveloperPrincipal())))
		})
	}
	return m.auth.Middleware("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := m.auth.Principal(r)
		if !ok {
			writeAuthError(w, r, errUnauthorized, http.StatusUnauthorized)
			return
		}
		if principal.DevBypass {
			next.ServeHTTP(w, r)
			return
		}
		var credential *access.APICredential
		if resolved, ok := m.auth.APICredential(r); ok {
			credential = &resolved
		}
		allowed, err := m.authorizeAnyWorkspace(r.Context(), principal.ID, credential, privilege)
		if err != nil {
			writeAuthError(w, r, err, http.StatusInternalServerError)
			return
		}
		if !allowed {
			writeAuthError(w, r, errForbidden, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (m *Module) authorizeAnyWorkspace(ctx context.Context, principalID string, credential *access.APICredential, privilege access.Privilege) (bool, error) {
	if m.workspaceIDs == nil || m.repository == nil {
		return false, nil
	}
	workspaceIDs, err := m.workspaceIDs(ctx)
	if err != nil {
		return false, err
	}
	repository, err := m.repository()
	if err != nil {
		return false, err
	}
	if repository == nil || strings.TrimSpace(principalID) == "" {
		return false, nil
	}
	objects := authorizationObjects(workspaceIDs, m.workspaceID, credential, privilege)
	decision, err := repository.AuthorizeAny(ctx, principalID, privilege, objects)
	return decision.Allowed, err
}

func authorizationObjects(workspaceIDs []string, defaultWorkspaceID string, credential *access.APICredential, privilege access.Privilege) []access.ObjectRef {
	objects := make([]access.ObjectRef, 0, len(workspaceIDs)+1)
	if credential == nil || apiTokenAllows(credential.Token, "", privilege) {
		objects = append(objects, access.PlatformObject())
	}
	for _, workspaceID := range workspaceIDs {
		if credential != nil && !apiTokenAllows(credential.Token, workspaceID, privilege) {
			continue
		}
		objects = append(objects, access.WorkspaceObject(workspaceID))
	}
	defaultWorkspaceID = strings.TrimSpace(defaultWorkspaceID)
	if defaultWorkspaceID != "" && !containsWorkspaceObject(objects, defaultWorkspaceID) &&
		(credential == nil || apiTokenAllows(credential.Token, defaultWorkspaceID, privilege)) {
		objects = append(objects, access.WorkspaceObject(defaultWorkspaceID))
	}
	return objects
}

func containsWorkspaceObject(objects []access.ObjectRef, workspaceID string) bool {
	expected := access.WorkspaceObject(workspaceID)
	for _, object := range objects {
		if object == expected {
			return true
		}
	}
	return false
}

func (m *Module) AuthorizeAnyWorkspace(ctx context.Context, principalID string, credential *access.APICredential, privilege access.Privilege) (bool, error) {
	return m.authorizeAnyWorkspace(ctx, principalID, credential, privilege)
}

func (m *Module) AuthorizeObject(ctx context.Context, principalID string, privilege access.Privilege, object access.ObjectRef) (bool, error) {
	if m == nil || m.repository == nil {
		return true, nil
	}
	repository, err := m.repository()
	if err != nil {
		return false, err
	}
	if repository == nil {
		return true, nil
	}
	decision, err := repository.Authorize(ctx, principalID, privilege, object)
	return decision.Allowed, err
}

func (m *Module) AuthorizeAnyObject(ctx context.Context, principalID string, privilege access.Privilege, objects []access.ObjectRef) (bool, error) {
	if m == nil || m.repository == nil {
		return true, nil
	}
	repository, err := m.repository()
	if err != nil {
		return false, err
	}
	if repository == nil {
		return true, nil
	}
	decision, err := repository.AuthorizeAny(ctx, principalID, privilege, objects)
	return decision.Allowed, err
}

func (m *Module) RecordAudit(ctx context.Context, input access.AuditEventInput) error {
	repository := m.repositoryValue()
	if repository == nil {
		return nil
	}
	return access.PersistAuditEvent(ctx, repository, input)
}
