// Package runtime defines typed analytical contracts shared with
// consumer-owned adapters. Capability modules expose capabilities rather than
// DuckDB or cache implementations.
package runtime

import (
	"context"

	analyticsmaterialize "github.com/Yacobolo/leapview/internal/analytics/materialize"
	"github.com/Yacobolo/leapview/internal/analytics/resource"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
)

type WorkspaceDatabase interface {
	analyticsmaterialize.Database
	resource.SessionProvider
	ValidateSnapshot(context.Context, int64) error
	CommitTransaction(context.Context, string, map[string]string, func(transaction.Transaction) error) (int64, error)
}

type Resources interface {
	WorkspaceDatabase() WorkspaceDatabase
	ResultCache() resultcache.ScopeProvider
}

type resources struct {
	database WorkspaceDatabase
	cache    resultcache.ScopeProvider
}

func NewResources(database WorkspaceDatabase, cache resultcache.ScopeProvider) Resources {
	return resources{database: database, cache: cache}
}

func (r resources) WorkspaceDatabase() WorkspaceDatabase {
	return r.database
}

func (r resources) ResultCache() resultcache.ScopeProvider {
	return r.cache
}
