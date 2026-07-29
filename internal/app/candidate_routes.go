package app

import (
	"net/http"
	"strings"

	deploymentmodule "github.com/flidai/leapview/internal/deployment/module"
	webpage "github.com/flidai/leapview/internal/platform/web/page"
	"github.com/go-chi/chi/v5"
)

type candidatePreviewHandler interface {
	ServeCandidatePreview(http.ResponseWriter, *http.Request, string, string, webpage.Provider)
}

func candidatePreview(routes *capabilityRoutes, runtime *runtimeServices, platform *platformServices, policy *httpPolicy, w http.ResponseWriter, r *http.Request) {
	if routes == nil || routes.accessModule == nil || routes.deploymentModule == nil {
		http.Error(w, "Candidate preview is unavailable", http.StatusServiceUnavailable)
		return
	}
	principal, ok := routes.accessModule.CurrentPrincipal(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	serveCandidatePreview(
		routes.deploymentModule, strings.TrimSpace(chi.URLParam(r, "candidate")), principal.ID,
		applicationLayout(routes, platform.assets, r), w, r,
	)
}

func serveCandidatePreview(
	handler candidatePreviewHandler,
	candidateID, principalID string,
	layout webpage.Provider,
	w http.ResponseWriter,
	r *http.Request,
) {
	if handler == nil || candidateID == "" || principalID == "" {
		http.NotFound(w, r)
		return
	}
	handler.ServeCandidatePreview(w, r, candidateID, principalID, layout)
}

var _ candidatePreviewHandler = (*deploymentmodule.Module)(nil)
