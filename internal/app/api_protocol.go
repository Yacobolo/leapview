package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	apiprotocol "github.com/Yacobolo/leapview/internal/api/protocol"
)

func (s *applicationAssembly) configureAPIProtocol(ctx context.Context, database *sql.DB) error {
	if ctx == nil {
		ctx = context.Background()
	}
	protocol, err := apiprotocol.Build(ctx, apiprotocol.Config{
		Database:    database,
		BearerToken: accessmodule.BearerToken,
		AcceptsBearer: func(r *http.Request) bool {
			return s.platform.auth == nil || s.platform.auth.AcceptsPublicBearer(r)
		},
		PrincipalID: func(r *http.Request) (string, bool) {
			if s.platform.auth == nil {
				return "", false
			}
			principal, _, ok := s.platform.auth.Authenticate(r)
			return principal.ID, ok
		},
		CursorSnapshot: s.cursorSnapshot,
	})
	if err != nil {
		return err
	}
	s.platform.apiProtocol = protocol
	return nil
}

func (s *applicationAssembly) publicProtocolMiddleware(next http.Handler) http.Handler {
	return s.platform.apiProtocol.Middleware(next)
}

func (s *applicationAssembly) openAPIDescription(w http.ResponseWriter, r *http.Request) {
	s.platform.apiProtocol.OpenAPIDescription(w, r)
}

func (s *applicationAssembly) publicDocs(w http.ResponseWriter, r *http.Request) {
	s.platform.apiProtocol.PublicDocs(w, r)
}

func (s *applicationAssembly) cursorSnapshot(r *http.Request) string {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for index, segment := range segments {
		if index+1 >= len(segments) {
			continue
		}
		switch segment {
		case "workspaces":
			if s.routes.workspaceModule != nil {
				snapshot, err := s.routes.workspaceModule.ActiveServingStateID(r.Context(), s.workspaceID(segments[index+1]))
				if err == nil && snapshot != "" {
					return snapshot
				}
			}
		case "projects":
			if snapshot := s.routes.releaseModule.ProjectCursorSnapshot(r, segments[index+1]); snapshot != "" {
				return snapshot
			}
		}
	}
	return ""
}
