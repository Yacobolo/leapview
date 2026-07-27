package api

import "context"

type agentVisualProjectionContextKey struct{}

// WithAgentVisualProjection requests the compact, bounded visual query
// projection used by agent transports.
func WithAgentVisualProjection(ctx context.Context) context.Context {
	return context.WithValue(ctx, agentVisualProjectionContextKey{}, true)
}

// RequestsAgentVisualProjection reports whether the caller requested the
// compact agent projection.
func RequestsAgentVisualProjection(ctx context.Context) bool {
	requested, _ := ctx.Value(agentVisualProjectionContextKey{}).(bool)
	return requested
}
