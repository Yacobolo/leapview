package datastar

import "github.com/Yacobolo/libredash/internal/ui"

func WorkspaceAccessSignals(access ui.WorkspaceAccessResponse, csrfToken string) map[string]any {
	return map[string]any{
		"workspaceAccess": ui.WorkspaceAccessSignals(access, csrfToken),
	}
}

func WorkspaceAssetRefreshSignals(patch map[string]any) map[string]any {
	return patch
}
