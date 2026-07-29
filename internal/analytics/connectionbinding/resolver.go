package connectionbinding

import (
	"context"
	"fmt"
	"strings"
)

type ResolverKind string

const (
	ResolverInfisical   ResolverKind = "infisical"
	ResolverEnvironment ResolverKind = "environment"
)

type TargetClass string

const (
	TargetProduction  TargetClass = "production"
	TargetDevelopment TargetClass = "development"
)

type ResolverSelection struct {
	TargetID    string
	Environment string
	TargetClass TargetClass
	Kind        ResolverKind
}

type ResolverSelectionInput ResolverSelection

type CredentialResolver interface {
	Resolve(context.Context, CredentialReference) (CredentialSnapshot, error)
}

type Repository interface {
	Create(context.Context, TargetBinding) error
	Binding(context.Context, BindingScope, string, LogicalConnectionID) (TargetBinding, error)
	Save(context.Context, TargetBinding, int64) (TargetBinding, error)
}

func NewResolverSelection(input ResolverSelectionInput) (ResolverSelection, error) {
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Environment = strings.TrimSpace(input.Environment)
	if !identifierPattern.MatchString(input.TargetID) || !identifierPattern.MatchString(input.Environment) {
		return ResolverSelection{}, fmt.Errorf("%w: resolver target and environment are required", ErrInvalidBinding)
	}
	if input.TargetClass != TargetProduction && input.TargetClass != TargetDevelopment {
		return ResolverSelection{}, fmt.Errorf("%w: resolver target class must be explicit", ErrInvalidBinding)
	}
	switch input.Kind {
	case ResolverInfisical:
	case ResolverEnvironment:
		if input.TargetClass == TargetProduction {
			return ResolverSelection{}, fmt.Errorf("%w: environment resolver cannot be selected for a production target", ErrInvalidBinding)
		}
	default:
		return ResolverSelection{}, fmt.Errorf("%w: exactly one authoritative resolver must be selected", ErrInvalidBinding)
	}
	return ResolverSelection(input), nil
}
