package module

import (
	"context"

	manageddataresolver "github.com/Yacobolo/leapview/internal/manageddata/resolver"
	"github.com/Yacobolo/leapview/internal/runtimehost"
	"github.com/Yacobolo/leapview/internal/servingstate"
)

type managedDataResolver struct {
	resolver ManagedDataSource
}

type ManagedDataSource interface {
	ResolveManagedData(context.Context, servingstate.ID) (manageddataresolver.Resolution, error)
}

func NewManagedDataResolver(resolver ManagedDataSource) runtimehost.ManagedDataResolver {
	if resolver == nil {
		return nil
	}
	return managedDataResolver{resolver: resolver}
}

func (r managedDataResolver) ResolveManagedData(ctx context.Context, id servingstate.ID) (runtimehost.ManagedDataResolution, error) {
	resolved, err := r.resolver.ResolveManagedData(ctx, id)
	if err != nil {
		return runtimehost.ManagedDataResolution{}, err
	}
	return runtimehost.ManagedDataResolution{
		RevisionID: resolved.RevisionID,
		Roots:      resolved.Roots,
		Lifetime:   resolved.Lifetime,
	}, nil
}
