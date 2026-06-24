package runtime

import (
	"context"
	"fmt"
)

func (m *Service) RefreshMaterializations(ctx context.Context, modelID string) error {
	runtime, ok := m.runtimes[modelID]
	if !ok {
		return fmt.Errorf("unknown semantic model %q", modelID)
	}
	if runtime.missing != nil {
		return runtime.missing
	}
	if runtime.data == nil {
		return fmt.Errorf("dashboard data runtime is not initialized")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return runtime.data.Refresh(ctx)
}
