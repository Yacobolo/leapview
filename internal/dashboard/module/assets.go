package module

import (
	"context"
	"fmt"
	"net/http"

	visualizationmapasset "github.com/flidai/leapview/internal/dashboard/visualization/mapasset"
	mapassethttp "github.com/flidai/leapview/internal/dashboard/visualization/mapasset/http"
)

// Assets is the dashboard-owned delivery and readiness surface for immutable
// visualization map assets.
type Assets interface {
	Handler() http.Handler
	Verify(context.Context) error
}

type mapAssets struct {
	handler http.Handler
}

// BuildAssets verifies the package embedded in the released binary before the
// application opens persistent state, then returns its delivery surface.
func BuildAssets(ctx context.Context) (Assets, error) {
	if err := visualizationmapasset.VerifyEmbedded(ctx); err != nil {
		return nil, fmt.Errorf("verify map assets: %w", err)
	}
	return &mapAssets{
		handler: mapassethttp.CacheHandler(http.StripPrefix("/map-assets/", http.FileServer(http.FS(visualizationmapasset.EmbeddedFS())))),
	}, nil
}

func (a *mapAssets) Handler() http.Handler {
	if a == nil {
		return http.NotFoundHandler()
	}
	return a.handler
}

func (a *mapAssets) Verify(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("embedded map assets are unavailable")
	}
	return visualizationmapasset.VerifyEmbedded(ctx)
}
