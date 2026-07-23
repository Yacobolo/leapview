package module

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Yacobolo/leapview/internal/access"
	accesshttp "github.com/Yacobolo/leapview/internal/access/http"
	"github.com/Yacobolo/leapview/internal/access/http/mcpoauth"
)

type Module struct {
	handler       accesshttp.Handler
	auth          *Auth
	repository    func() (access.Repository, error)
	workspaceIDs  func(context.Context) ([]string, error)
	workspaceID   string
	oauth         *mcpoauth.Service
	oauthResource mcpoauth.ResourceServer
	logger        *slog.Logger
}

type SurfaceConfig struct {
	Repository         func() (access.Repository, error)
	CurrentPrincipal   func(*http.Request) (Principal, bool)
	CurrentCredential  func(*http.Request) (access.APICredential, bool)
	WorkspaceID        func(string) string
	Auth               *Auth
	WorkspaceIDs       func(context.Context) ([]string, error)
	DefaultWorkspaceID string
	Logger             *slog.Logger
	OAuth              *mcpoauth.Service
	OAuthResource      mcpoauth.ResourceServer
}

func newSurface(config SurfaceConfig) *Module {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	currentPrincipal := func(r *http.Request) (accesshttp.Principal, bool) {
		if config.CurrentPrincipal == nil {
			return accesshttp.Principal{}, false
		}
		principal, ok := config.CurrentPrincipal(r)
		return accesshttp.Principal{ID: principal.ID, Email: principal.Email, DisplayName: principal.DisplayName}, ok
	}
	return &Module{auth: config.Auth, repository: config.Repository, workspaceIDs: config.WorkspaceIDs, workspaceID: config.DefaultWorkspaceID, logger: logger,
		oauth: config.OAuth, oauthResource: config.OAuthResource, handler: accesshttp.Handler{
			Repository: config.Repository, CurrentPrincipal: currentPrincipal,
			CurrentCredential: config.CurrentCredential, WorkspaceID: config.WorkspaceID,
		}}
}

func (m *Module) HTTP() accesshttp.Handler { return m.handler }

func (m *Module) Auth() *Auth {
	if m == nil {
		return nil
	}
	return m.auth
}

func (m *Module) CurrentPrincipal(r *http.Request) (Principal, bool) {
	if m == nil || m.auth == nil {
		return LocalDeveloperPrincipal(), true
	}
	return m.auth.Principal(r)
}
