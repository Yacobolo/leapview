package module

import "github.com/Yacobolo/leapview/internal/access"

func (m *Module) DataAuthorizationService() access.DataAuthorizationService {
	return m.repositoryValue()
}
