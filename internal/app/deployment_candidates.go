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

func (s *applicationAssembly) servingStateRepository(inputs moduleAssemblyInputs) (servingStateRepository, error) {
	if inputs.persistence.servingStateRepo != nil {
		return inputs.persistence.servingStateRepo, nil
	}
	return nil, fmt.Errorf("serving state repository is not configured")
}

func (s *applicationAssembly) workspaceID(value string) string {
	return value
}

func (s *applicationAssembly) defaultServingEnvironment() servingstatemodule.Environment {
	return servingstatemodule.NormalizeEnvironment(servingstatemodule.Environment(s.policy.defaultEnvironment))
}

func (s *applicationAssembly) requestServingEnvironment(r *http.Request) servingstatemodule.Environment {
	return s.defaultServingEnvironment()
}
