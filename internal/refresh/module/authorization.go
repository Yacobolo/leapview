package module

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/access"
)

func authorizePipeline(r *http.Request, workspaceID, pipelineID string, privilege access.Privilege, config AuthorizationConfig) (bool, error) {
	if config.CurrentPrincipal == nil {
		return false, nil
	}
	principal, ok := config.CurrentPrincipal(r)
	if !ok {
		return false, nil
	}
	if principal.DevBypass {
		return true, nil
	}
	if config.CurrentCredential != nil {
		if credential, ok := config.CurrentCredential(r); ok && !access.TokenAllows(credential.Token, workspaceID, privilege) {
			return false, nil
		}
	}
	if config.ResolvePipelineModel == nil {
		return false, nil
	}
	modelID, found, err := config.ResolvePipelineModel(r.Context(), workspaceID, strings.TrimSpace(pipelineID))
	if err != nil || !found {
		return false, err
	}
	if config.AuthorizeObject == nil {
		return true, nil
	}
	object := access.ItemObjectWithParent(access.SecurableSemanticModel, workspaceID, modelID, access.WorkspaceObject(workspaceID))
	return config.AuthorizeObject(r.Context(), principal.ID, privilege, object)
}
