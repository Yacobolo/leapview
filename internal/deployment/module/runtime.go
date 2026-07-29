package module

import (
	"context"

	"github.com/flidai/leapview/internal/deployment"
	"github.com/flidai/leapview/internal/runtimehost"
)

type RuntimeRegistry interface {
	PrepareServingStateCandidates(context.Context, []runtimehost.ServingStateCandidate) (*runtimehost.PreparedSet, error)
	ActivatePreparedSet(*runtimehost.PreparedSet, func() error) error
}

func NewRuntime(registry RuntimeRegistry) (deployment.Runtime, error) {
	return deployment.NewRegistryRuntime(registry)
}
