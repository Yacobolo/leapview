package observability

import (
	"context"
	"net/http"
	"time"

	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
)

type HealthConfig struct {
	Platform         func(context.Context) error
	Analytics        func() error
	ActiveWorkspaces func(context.Context) ([]string, error)
	RuntimeReady     func(context.Context, string) error
	Checks           map[string]func(context.Context) error
}

type Health struct {
	config HealthConfig
}

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func NewHealth(config HealthConfig) *Health {
	return &Health{config: config}
}

func (h *Health) Healthz(w http.ResponseWriter, _ *http.Request) {
	apitransport.WriteJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (h *Health) Readyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	if h == nil || h.config.Platform == nil {
		checks["platformStore"] = "missing"
		apitransport.WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Checks: checks})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.config.Platform(ctx); err != nil {
		checks["platformStore"] = err.Error()
		apitransport.WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Checks: checks})
		return
	}
	checks["platformStore"] = "ok"
	if h.config.Analytics != nil {
		if err := h.config.Analytics(); err != nil {
			checks["analytics"] = err.Error()
			apitransport.WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Checks: checks})
			return
		}
		checks["analytics"] = "ok"
	}
	for name, check := range h.config.Checks {
		if check == nil {
			continue
		}
		if err := check(ctx); err != nil {
			checks[name] = err.Error()
			apitransport.WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Checks: checks})
			return
		}
		checks[name] = "ok"
	}
	if !h.runtimeReady(ctx, checks) {
		apitransport.WriteJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready", Checks: checks})
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, healthResponse{Status: "ready", Checks: checks})
}

func (h *Health) runtimeReady(ctx context.Context, checks map[string]string) bool {
	if h.config.ActiveWorkspaces == nil || h.config.RuntimeReady == nil {
		checks["runtime"] = "missing"
		return false
	}
	workspaces, err := h.config.ActiveWorkspaces(ctx)
	if err != nil {
		checks["runtime"] = err.Error()
		return false
	}
	if len(workspaces) == 0 {
		checks["runtime"] = "no_active_deployments"
		return true
	}
	ready := true
	for _, workspaceID := range workspaces {
		checkName := "workspaceRuntime:" + workspaceID
		if err := h.config.RuntimeReady(ctx, workspaceID); err != nil {
			checks[checkName] = err.Error()
			ready = false
			continue
		}
		checks[checkName] = "ok"
	}
	return ready
}
