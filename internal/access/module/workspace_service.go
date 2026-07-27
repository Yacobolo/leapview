package module

import "github.com/Yacobolo/leapview/internal/access"

func (m *Module) WorkspaceAccessService() access.WorkspaceAccessService {
	return m.repositoryValue()
}
