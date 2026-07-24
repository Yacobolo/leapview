package module

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	visualizationmapasset "github.com/Yacobolo/leapview/internal/dashboard/visualization/mapasset"
	mapassethttp "github.com/Yacobolo/leapview/internal/dashboard/visualization/mapasset/http"
)

// Assets is the dashboard-owned delivery and readiness surface for immutable
// visualization map assets.
type Assets interface {
	Handler() http.Handler
	Verify(context.Context) error
}

type mapAssets struct {
	root     string
	verifier *visualizationmapasset.Verifier
	handler  http.Handler
}

// BuildAssets verifies the configured package before the application opens
// persistent state, then returns the narrow surface retained by composition.
func BuildAssets(ctx context.Context, root string) (Assets, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	verifier := visualizationmapasset.NewVerifier(root)
	if err := verifier.Verify(ctx); err != nil {
		return nil, fmt.Errorf("verify map assets: %w", err)
	}
	return &mapAssets{
		root:     root,
		verifier: verifier,
		handler:  mapassethttp.CacheHandler(http.StripPrefix("/map-assets/", http.FileServer(http.Dir(root)))),
	}, nil
}

func (a *mapAssets) Handler() http.Handler {
	if a == nil {
		return http.NotFoundHandler()
	}
	return a.handler
}

func (a *mapAssets) Verify(ctx context.Context) error {
	if a == nil || a.verifier == nil {
		return nil
	}
	return a.verifier.Verify(ctx)
}
