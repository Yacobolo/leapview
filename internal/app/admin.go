package app

import "net/http"

func (s *Server) currentAdminRoleLabel(r *http.Request) string {
	if s.auth == nil {
		return "Local platform"
	}
	principal, ok := s.auth.Principal(r)
	if ok && principal.DevBypass {
		return "Platform admin"
	}
	return "Platform access"
}
