package runtime

import (
	"context"
	"fmt"
	"sync"
)

type MaterializationService struct {
	mu       *sync.RWMutex
	runtimes map[string]*modelRuntime
}

func (m *Service) RefreshMaterializations(ctx context.Context, modelID string) error {
	return m.materializations.RefreshMaterializations(ctx, modelID)
}

func (s *MaterializationService) RefreshMaterializations(ctx context.Context, modelID string) error {
	runtime, ok := s.runtimes[modelID]
	if !ok {
		return fmt.Errorf("unknown semantic model %q", modelID)
	}
	if runtime.missing != nil {
		return runtime.missing
	}
	if runtime.data == nil {
		return fmt.Errorf("dashboard data runtime is not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return runtime.data.Refresh(ctx)
}
