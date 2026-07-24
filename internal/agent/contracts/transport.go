package contracts

import "context"

type dashboardVisualProjectionContextKey struct{}

func WithDashboardVisualProjection(ctx context.Context) context.Context {
	return context.WithValue(ctx, dashboardVisualProjectionContextKey{}, true)
}

func RequestsDashboardVisualProjection(ctx context.Context) bool {
	requested, _ := ctx.Value(dashboardVisualProjectionContextKey{}).(bool)
	return requested
}
