package app

import (
	accesssqlite "github.com/Yacobolo/libredash/internal/access/sqlite"
	"github.com/Yacobolo/libredash/internal/platform"
)

func testAuth(store *platform.Store, workspaceID string, cfg AuthConfig) *Auth {
	return NewAuth(accesssqlite.NewRepository(store.SQLDB()), workspaceID, cfg)
}
