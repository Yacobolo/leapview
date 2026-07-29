package app

import (
	"context"

	analyticsmodule "github.com/flidai/leapview/internal/analytics/module"
)

// connectionBindingDependenciesWithoutConsumers is the composition adapter
// for the current serving graph. Target-owned bindings are not yet referenced
// by compiled serving states, so there are no registered dependents to report.
// The adapter keeps that fact explicit at the capability boundary.
type connectionBindingDependenciesWithoutConsumers struct{}

func (connectionBindingDependenciesWithoutConsumers) Dependents(
	context.Context,
	analyticsmodule.ConnectionTargetBinding,
) ([]analyticsmodule.ConnectionBindingDependency, error) {
	return nil, nil
}
