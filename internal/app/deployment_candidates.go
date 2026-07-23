package app

import (
	"context"
	"fmt"
	"net/http"

	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
	servingstatemodule "github.com/Yacobolo/leapview/internal/servingstate/module"
)

type runtimeReloader interface {
	PrepareServingState(ctx context.Context, servingStateID string) (servingstatemodule.PreparedRuntime, error)
	ActivatePrepared(prepared servingstatemodule.PreparedRuntime, activate func() error) error
}

type servingStateRepository interface {
	refreshmodule.ServingStateRepository
	ListActiveScopes(context.Context) ([]servingstatemodule.ActiveScope, error)
}

func (s *runtimeRouter) servingStateRepository() (servingStateRepository, error) {
	if s.construction.servingStateRepo != nil {
		return s.construction.servingStateRepo, nil
	}
	return nil, fmt.Errorf("serving state repository is not configured")
}

func (s *runtimeRouter) workspaceID(value string) string {
	return value
}

func (s *runtimeRouter) defaultServingEnvironment() servingstatemodule.Environment {
	return servingstatemodule.NormalizeEnvironment(servingstatemodule.Environment(s.defaultEnvironment))
}

func (s *runtimeRouter) requestServingEnvironment(r *http.Request) servingstatemodule.Environment {
	return s.defaultServingEnvironment()
}
