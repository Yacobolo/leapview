package module

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/workspace"
	workspacehttp "github.com/Yacobolo/leapview/internal/workspace/http"
	"github.com/go-chi/chi/v5"
)

// AssetObjectRefs resolves the authorization chain for workspace asset API
// operations without exposing the workspace HTTP adapter to composition.
func AssetObjectRefs(r *http.Request, workspaceID string) []access.ObjectRef {
	return workspacehttp.AssetObjectRefs(r, workspaceID)
}

// AssetObjectRefs makes a refresh pipeline inherit the authorization boundary
// of the semantic model it refreshes. Pipelines are configuration and are not
// independently grantable.
func (m *Module) AssetObjectRefs(r *http.Request, workspaceID string) []access.ObjectRef {
	if m == nil {
		return []access.ObjectRef{access.WorkspaceObject(workspaceID)}
	}
	rawAssetID := strings.TrimSpace(chi.URLParam(r, "asset"))
	if !strings.HasPrefix(rawAssetID, string(workspace.AssetTypeRefreshPipeline)+":") {
		return workspacehttp.AssetObjectRefs(r, workspaceID)
	}
	environment := ""
	if m.handler.Environment != nil {
		environment = m.handler.Environment(r)
	}
	assets, edges, err := m.handler.ReadModel.WorkspaceAssetsAndEdgesForData(r.Context(), workspaceID, environment)
	if err != nil {
		return []access.ObjectRef{access.WorkspaceObject(workspaceID)}
	}
	for _, edge := range edges {
		if edge.FromAssetID != rawAssetID || edge.Type != string(workspace.AssetEdgeRefreshesSemanticModel) {
			continue
		}
		for _, asset := range assets {
			if asset.ID != edge.ToAssetID || asset.Type != string(workspace.AssetTypeSemanticModel) {
				continue
			}
			modelID := strings.TrimPrefix(asset.Key, workspaceID+".")
			model := access.ItemObjectWithParent(access.SecurableSemanticModel, workspaceID, modelID, access.WorkspaceObject(workspaceID))
			return []access.ObjectRef{model, access.WorkspaceObject(workspaceID)}
		}
	}
	return []access.ObjectRef{access.WorkspaceObject(workspaceID)}
}
