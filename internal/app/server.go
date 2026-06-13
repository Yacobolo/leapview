package app

import (
	"context"
	"net/http"
	"time"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/ui"
	"github.com/starfederation/datastar-go/datastar"
)

type queryMetrics interface {
	QueryDashboard(ctx context.Context, filters dashboard.Filters) (dashboard.Patch, error)
	DataDir() string
}

type Server struct {
	metrics queryMetrics
}

func New(metrics queryMetrics) *Server {
	return &Server{metrics: metrics}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.home)
	mux.HandleFunc("GET /updates", s.updates)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return mux
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := ui.Page(s.metrics.DataDir()).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) updates(w http.ResponseWriter, r *http.Request) {
	signals := dashboard.Signals{}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	filters := signals.Filters.WithDefaults()

	sse := datastar.NewSSE(w, r)
	_ = sse.MarshalAndPatchSignals(map[string]any{
		"status": map[string]any{
			"loading":       true,
			"error":         "",
			"dataDirectory": s.metrics.DataDir(),
		},
	})

	patch, err := s.metrics.QueryDashboard(r.Context(), filters)
	if err != nil {
		patch = dashboard.EmptyPatch(filters, s.metrics.DataDir(), err)
	}

	if err := sse.MarshalAndPatchSignals(patch); err != nil {
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			patch, err := s.metrics.QueryDashboard(r.Context(), filters)
			if err != nil {
				patch = dashboard.EmptyPatch(filters, s.metrics.DataDir(), err)
			}
			if err := sse.MarshalAndPatchSignals(patch); err != nil {
				return
			}
		}
	}
}
