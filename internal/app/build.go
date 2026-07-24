package app

import (
	"context"

	"github.com/Yacobolo/leapview/internal/config"
)

// Build is the sole process assembly entrypoint. Capability construction is
// exposed through module surfaces; Application retains only the final HTTP
// handler and lifecycle contracts.
func Build(ctx context.Context, cfg config.Config) (*Application, error) {
	handler, lifecycle, cleanup, err := assemble(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return newApplication(handler, []Lifecycle{lifecycle}, cleanup), nil
}
