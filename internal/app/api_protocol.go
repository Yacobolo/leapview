package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	apiprotocol "github.com/Yacobolo/leapview/internal/api/protocol"
)

func (s *runtimeRouter) configureAPIProtocol(database *sql.DB) {
	protocol, err := apiprotocol.Build(context.Background(), apiprotocol.Config{
		Database:    database,
		BearerToken: accessmodule.BearerToken,
		AcceptsBearer: func(r *http.Request) bool {
			return s.auth == nil || s.auth.AcceptsPublicBearer(r)
		},
		PrincipalID: func(r *http.Request) (string, bool) {
			if s.auth == nil {
				return "", false
			}
			principal, _, ok := s.auth.Authenticate(r)
			return principal.ID, ok
		},
		CursorSnapshot: s.cursorSnapshot,
	})
	if err != nil {
		s.logger.ErrorContext(context.Background(), "configure API protocol failed", "error", err)
		return
	}
	s.apiProtocol = protocol
}

func (s *runtimeRouter) publicProtocolMiddleware(next http.Handler) http.Handler {
	return s.apiProtocol.Middleware(next)
}

func (s *runtimeRouter) openAPIDescription(w http.ResponseWriter, r *http.Request) {
	s.apiProtocol.OpenAPIDescription(w, r)
}

func (s *runtimeRouter) publicDocs(w http.ResponseWriter, r *http.Request) {
	s.apiProtocol.PublicDocs(w, r)
}

func (s *runtimeRouter) cursorSnapshot(r *http.Request) string {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for index, segment := range segments {
		if index+1 >= len(segments) {
			continue
		}
		switch segment {
		case "workspaces":
			if s.workspaceModule != nil {
				snapshot, err := s.workspaceModule.ActiveServingStateID(r.Context(), s.workspaceID(segments[index+1]))
				if err == nil && snapshot != "" {
					return snapshot
				}
			}
		case "projects":
			if snapshot := s.releaseModule.ProjectCursorSnapshot(r, segments[index+1]); snapshot != "" {
				return snapshot
			}
		}
	}
	return ""
}
