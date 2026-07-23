// Package runtime defines the opaque analytical runtime resources shared with
// consumer-owned adapters. Capability modules never expose DuckDB or cache
// implementations through their public surface.
package runtime

type Resources interface {
	analyticsRuntimeResources()
}

type resources struct {
	database any
	cache    any
}

func (resources) analyticsRuntimeResources() {}

func NewResources(database, cache any) Resources {
	return resources{database: database, cache: cache}
}

func Unwrap(input Resources) (database, cache any) {
	value, ok := input.(resources)
	if !ok {
		return nil, nil
	}
	return value.database, value.cache
}
