package module

import (
	"fmt"
	"net/http"

	"github.com/Yacobolo/leapview/internal/access/scimprov"
)

func (m *Module) SCIMHandler(bearerToken string) (http.Handler, error) {
	if m == nil || m.repository == nil {
		return nil, fmt.Errorf("access repository is unavailable")
	}
	repository, err := m.repository()
	if err != nil {
		return nil, err
	}
	if repository == nil {
		return nil, fmt.Errorf("access repository is unavailable")
	}
	return scimprov.NewHandler(scimprov.Options{Repository: repository, BearerToken: bearerToken})
}
