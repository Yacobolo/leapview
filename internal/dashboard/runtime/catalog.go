package runtime

import (
	"sync"

	"github.com/Yacobolo/leapview/internal/dashboard/catalog"
	dashboarddefinition "github.com/Yacobolo/leapview/internal/dashboard/definition"
)

type CatalogService struct {
	mu        *sync.RWMutex
	workspace *dashboarddefinition.Workspace
	catalog   catalog.Catalog
}

func NewCatalogService(mu *sync.RWMutex, workspace *dashboarddefinition.Workspace) *CatalogService {
	service := &CatalogService{mu: mu, workspace: workspace}
	service.catalog = service.catalogView()
	return service
}

func (m *Service) Catalog() catalog.Catalog {
	return m.catalog.Catalog()
}

func (s *CatalogService) Catalog() catalog.Catalog {
	return s.catalog
}

func (s *CatalogService) catalogView() catalog.Catalog {
	if s.workspace == nil {
		return catalog.Catalog{}
	}
	return s.workspace.Catalog
}
