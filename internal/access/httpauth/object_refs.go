package httpauth

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/go-chi/chi/v5"
)

func ObjectsForRequest(privilege access.Privilege, r *http.Request, workspaceID string) []access.ObjectRef {
	if privilege == access.PrivilegeManagePlatform {
		return []access.ObjectRef{access.PlatformObject()}
	}
	objects := RouteObjectRefs(r, workspaceID)
	if len(objects) == 0 {
		objects = append(objects, ObjectForWorkspace(workspaceID))
	}
	return objects
}

func RouteCanDeferDataAuth(privilege access.Privilege, r *http.Request) bool {
	if privilege != access.PrivilegeQueryData && privilege != access.PrivilegePreviewData {
		return false
	}
	if strings.TrimSpace(chi.URLParam(r, "dashboard")) != "" {
		return true
	}
	return strings.TrimSpace(chi.URLParam(r, "model")) != "" && strings.TrimSpace(chi.URLParam(r, "dataset")) != ""
}

func RouteCanDeferGrantManagement(privilege access.Privilege, r *http.Request) bool {
	return privilege == access.PrivilegeManageGrants && (strings.Contains(r.URL.Path, "/grants") || strings.Contains(r.URL.Path, "/data-policies"))
}

func RouteObjectRefs(r *http.Request, workspaceID string) []access.ObjectRef {
	workspaceID = strings.TrimSpace(workspaceID)
	objects := []access.ObjectRef{}
	if dashboardID := strings.TrimSpace(chi.URLParam(r, "dashboard")); dashboardID != "" {
		objects = append(objects, access.ItemObject(access.SecurableDashboard, workspaceID, dashboardID))
	}
	modelID := strings.TrimSpace(chi.URLParam(r, "model"))
	if modelID != "" {
		model := access.ItemObject(access.SecurableSemanticModel, workspaceID, modelID)
		if datasetID := strings.TrimSpace(chi.URLParam(r, "dataset")); datasetID != "" {
			objects = append(objects, access.ItemObjectWithParent(access.SecurableDataset, workspaceID, modelID+"/"+datasetID, model))
		}
		objects = append(objects, model)
	}
	if conversationID := strings.TrimSpace(chi.URLParam(r, "conversation")); conversationID != "" {
		objects = append(objects, access.ItemObject(access.SecurableAgentPolicy, workspaceID, "conversation/"+conversationID))
	}
	if assetID := strings.TrimSpace(chi.URLParam(r, "asset")); assetID != "" {
		if object, ok := AssetObjectRef(workspaceID, assetID); ok {
			objects = append(objects, object)
		}
	}
	if workspaceID != "" {
		objects = append(objects, access.WorkspaceObject(workspaceID))
	}
	return objects
}

func ObjectForWorkspace(workspaceID string) access.ObjectRef {
	if strings.TrimSpace(workspaceID) == "" {
		return access.PlatformObject()
	}
	return access.WorkspaceObject(workspaceID)
}

func AssetObjectRef(workspaceID, assetID string) (access.ObjectRef, bool) {
	typ, objectID, ok := assetSecurableParts(assetID)
	if !ok {
		return access.ObjectRef{}, false
	}
	return ObjectWithInferredParent(typ, workspaceID, objectID), true
}

func ObjectWithInferredParent(typ access.SecurableType, workspaceID, objectID string) access.ObjectRef {
	parts := strings.Split(objectID, "/")
	switch typ {
	case access.SecurableDataset, access.SecurableTable:
		if len(parts) >= 2 && strings.TrimSpace(parts[0]) != "" {
			return access.ItemObjectWithParent(typ, workspaceID, objectID, access.ItemObject(access.SecurableSemanticModel, workspaceID, parts[0]))
		}
	case access.SecurableColumn:
		if len(parts) >= 3 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			parent := access.ItemObjectWithParent(access.SecurableDataset, workspaceID, parts[0]+"/"+parts[1], access.ItemObject(access.SecurableSemanticModel, workspaceID, parts[0]))
			return access.ItemObjectWithParent(typ, workspaceID, objectID, parent)
		}
	}
	return access.ItemObject(typ, workspaceID, objectID)
}

func assetSecurableParts(assetID string) (access.SecurableType, string, bool) {
	prefix, objectID, ok := strings.Cut(strings.TrimSpace(assetID), ":")
	if !ok || strings.TrimSpace(objectID) == "" {
		return "", "", false
	}
	switch prefix {
	case string(access.SecurableDashboard):
		return access.SecurableDashboard, objectID, true
	case string(access.SecurableSemanticModel):
		return access.SecurableSemanticModel, objectID, true
	case string(access.SecurableSource):
		return access.SecurableSource, objectID, true
	case string(access.SecurableModelTable):
		return access.SecurableModelTable, objectID, true
	case string(access.SecurableDataset):
		return access.SecurableDataset, objectID, true
	case string(access.SecurableTable):
		return access.SecurableTable, objectID, true
	case string(access.SecurableColumn):
		return access.SecurableColumn, objectID, true
	case string(access.SecurableAgentPolicy):
		return access.SecurableAgentPolicy, objectID, true
	default:
		return "", "", false
	}
}
