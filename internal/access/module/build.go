package module

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/access/http/mcpoauth"
	accesssqlite "github.com/Yacobolo/leapview/internal/access/sqlite"
)

type Config struct {
	Database     *sql.DB
	WorkspaceID  string
	Auth         AuthConfig
	ExistingAuth *Auth
	WorkspaceIDs func(context.Context) ([]string, error)
	PublicURL    string
	MCPIssuerURL string
	Surface      *SurfaceConfig
}

func newRepository(database *sql.DB) access.Repository { return accesssqlite.NewRepository(database) }

func Build(ctx context.Context, config Config) (*Module, error) {
	if config.Database == nil {
		if config.Surface == nil {
			return newSurface(SurfaceConfig{}), nil
		}
		return newSurface(*config.Surface), nil
	}
	if err := accesssqlite.Initialize(ctx, config.Database); err != nil {
		return nil, err
	}
	repository := newRepository(config.Database)
	auth := config.ExistingAuth
	if auth == nil {
		auth = NewAuth(repository, config.WorkspaceID, config.Auth)
	}
	module := newSurface(SurfaceConfig{
		Repository: func() (access.Repository, error) { return repository, nil },
		Auth:       auth, WorkspaceIDs: config.WorkspaceIDs,
		DefaultWorkspaceID: config.WorkspaceID,
		WorkspaceID: func(value string) string {
			if value != "" {
				return value
			}
			return config.WorkspaceID
		},
		CurrentPrincipal: func(r *http.Request) (Principal, bool) {
			principal, ok := auth.Principal(r)
			return principal, ok
		},
		CurrentCredential: auth.APICredential,
	})
	publicURL := strings.TrimSuffix(strings.TrimSpace(config.PublicURL), "/")
	if publicURL == "" {
		publicURL = "http://localhost:8080"
	}
	var err error
	if issuer := strings.TrimSpace(config.MCPIssuerURL); issuer != "" {
		module.oauthResource, err = mcpoauth.NewExternal(repository, mcpoauth.ExternalConfig{IssuerURL: issuer, ResourceURL: publicURL + "/mcp"})
	} else {
		module.oauth, err = mcpoauth.New(config.Database, repository, mcpoauth.Config{
			IssuerURL: publicURL, ResourceURL: publicURL + "/mcp", Secret: auth.MCPOAuthSecret(),
		})
		module.oauthResource = module.oauth
	}
	if err != nil {
		return nil, err
	}
	return module, nil
}

func (m *Module) OAuthResource() mcpoauth.ResourceServer {
	if m == nil {
		return nil
	}
	return m.oauthResource
}

func (m *Module) OAuthService() *mcpoauth.Service {
	if m == nil {
		return nil
	}
	return m.oauth
}

func (m *Module) repositoryValue() access.Repository {
	if m == nil || m.repository == nil {
		return nil
	}
	repository, _ := m.repository()
	return repository
}

func (m *Module) SeedLocalDeveloperPlatformAdmin(ctx context.Context) error {
	if m == nil {
		return nil
	}
	repository := m.repositoryValue()
	if repository == nil {
		return nil
	}
	principal := LocalDeveloperPrincipal()
	_, err := repository.SetPlatformRole(ctx, access.PlatformRoleInput{
		PrincipalID: principal.ID, Email: principal.Email, DisplayName: principal.DisplayName, Role: access.RolePlatformAdmin,
	})
	return err
}
