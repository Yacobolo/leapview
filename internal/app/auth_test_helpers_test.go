package app

import (
	"context"

	"github.com/Yacobolo/leapview/internal/access"
	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	accesssqlite "github.com/Yacobolo/leapview/internal/access/sqlite"
	"github.com/Yacobolo/leapview/internal/platform"
)

func testAuth(store *platform.Store, workspaceID string, cfg AuthConfig) *accessmodule.Auth {
	repo := accesssqlite.NewRepository(store.SQLDB())
	if cfg.DevBypass {
		_, _ = repo.SetPlatformRole(context.Background(), access.PlatformRoleInput{
			PrincipalID: "dev",
			Email:       "dev@localhost",
			DisplayName: "Local Developer",
			Role:        access.RolePlatformAdmin,
		})
	}
	return NewAuth(repo, workspaceID, cfg)
}
