package module

import (
	"context"

	"github.com/Yacobolo/leapview/internal/workspace"
)

type activeWorkspaceLister interface {
	ListWithActiveMetadata(context.Context, string) ([]workspace.Summary, error)
}

func (m *Module) ActiveRuntimeWorkspaces(ctx context.Context) ([]string, error) {
	if m == nil {
		return nil, nil
	}
	if lister, ok := m.repository.(activeWorkspaceLister); ok {
		summaries, err := lister.ListWithActiveMetadata(ctx, m.runtimeEnvironment)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(summaries))
		for _, summary := range summaries {
			if summary.ID != "" && summary.ActiveServingStateID != "" {
				out = append(out, string(summary.ID))
			}
		}
		return out, nil
	}
	if m.defaultWorkspaceID != "" {
		return []string{m.defaultWorkspaceID}, nil
	}
	if m.rootMetrics != nil {
		catalog := m.rootMetrics.Catalog()
		if catalog.Workspace.ID != "" && (len(catalog.Dashboards) > 0 || len(catalog.Models) > 0) {
			return []string{catalog.Workspace.ID}, nil
		}
	}
	return nil, nil
}
