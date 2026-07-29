package module

import "github.com/flidai/leapview/internal/access"

func (m *Module) DataAuthorizationService() access.DataAuthorizationService {
	return m.repositoryValue()
}
