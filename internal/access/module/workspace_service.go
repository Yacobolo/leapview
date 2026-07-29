package module

import "github.com/flidai/leapview/internal/access"

func (m *Module) WorkspaceAccessService() access.WorkspaceAccessService {
	return m.repositoryValue()
}
