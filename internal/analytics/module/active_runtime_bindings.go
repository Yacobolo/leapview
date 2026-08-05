package module

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
	analyticsruntime "github.com/flidai/leapview/internal/analytics/runtime"
	platformdigest "github.com/flidai/leapview/internal/platform/digest"
)

// ActiveRuntimeBindingEvidence is the non-secret, immutable connection proof
// retained with a ready release.
type ActiveRuntimeBindingEvidence struct {
	BindingID          string
	LogicalConnection  string
	ConnectorKind      string
	Revision           int64
	ValidatedVersion   string
	EndpointConfigHash string
}

type ActiveRuntimeBindingEvidenceSource interface {
	BindingEvidence(context.Context, string, string) ([]ActiveRuntimeBindingEvidence, error)
}

func (m *Module) ConfigureActiveRuntimeBindings(source ActiveRuntimeBindingEvidenceSource) error {
	if m == nil || source == nil || m.connectionBindings == nil || m.connectionFactory == nil {
		return connectionbinding.ErrProviderUnavailable
	}
	m.activeRuntimeBindingEvidence = source
	return nil
}

type activeRuntimeConnectionResolver struct {
	module         *Module
	servingStateID string
	workspaceID    string
	environment    string

	once     sync.Once
	evidence map[string]ActiveRuntimeBindingEvidence
	err      error
}

func (r *activeRuntimeConnectionResolver) Resolve(
	ctx context.Context,
	name string,
	logical semanticmodel.Connection,
) (semanticmodel.Connection, error) {
	if r == nil || r.module == nil {
		return semanticmodel.Connection{}, connectionbinding.ErrProviderUnavailable
	}
	evidence, err := r.evidenceFor(ctx, name)
	if err != nil {
		return semanticmodel.Connection{}, err
	}
	logicalID, err := connectionbinding.ParseLogicalConnectionID(strings.TrimSpace(name))
	if err != nil {
		return semanticmodel.Connection{}, connectionbinding.ErrBindingNotFound
	}
	binding, err := r.module.connectionBindings.Binding(ctx, connectionbinding.BindingScope{
		WorkspaceID: r.workspaceID, Environment: r.environment,
	}, r.module.targetID, logicalID)
	if err != nil {
		return semanticmodel.Connection{}, err
	}
	actual := binding.Evidence()
	if !binding.Enabled || binding.ID != evidence.BindingID ||
		binding.LogicalConnectionID.String() != evidence.LogicalConnection ||
		binding.ConnectorKind != evidence.ConnectorKind || binding.Revision < evidence.Revision ||
		actual.EndpointConfigHash != evidence.EndpointConfigHash ||
		strings.TrimSpace(evidence.ValidatedVersion) == "" {
		return semanticmodel.Connection{}, connectionbinding.ErrIncompatibleBinding
	}
	resolver, err := connectionbinding.SelectResolver(connectionbinding.ResolverSelection{
		TargetID: binding.TargetID, Environment: binding.Scope.Environment,
		TargetClass: r.module.targetClass, Kind: r.module.connectionResolverKind(),
	}, r.module.targetResolvers)
	if err != nil {
		return semanticmodel.Connection{}, err
	}
	versioned, ok := resolver.(connectionbinding.VersionedCredentialResolver)
	if !ok {
		return semanticmodel.Connection{}, connectionbinding.ErrProviderUnavailable
	}
	snapshot, err := versioned.ResolveVersion(ctx, binding.CredentialReference, evidence.ValidatedVersion)
	if err != nil {
		return semanticmodel.Connection{}, err
	}
	defer snapshot.Destroy()
	pool, err := r.module.connectionFactory.Prepare(ctx, binding, snapshot)
	if err != nil || pool == nil {
		return semanticmodel.Connection{}, connectionbinding.ErrProviderUnavailable
	}
	// The prepared pool is an activation probe, not shared mutable binding state.
	// Resolve returns an isolated connection copy before the probe is destroyed.
	defer pool.Close()
	if err := pool.HealthCheck(ctx); err != nil {
		return semanticmodel.Connection{}, connectionbinding.ErrProviderUnavailable
	}
	target, ok := pool.(analyticsruntime.ConnectionResolver)
	if !ok {
		return semanticmodel.Connection{}, connectionbinding.ErrProviderUnavailable
	}
	return target.Resolve(ctx, name, logical)
}

func (r *activeRuntimeConnectionResolver) evidenceFor(
	ctx context.Context,
	name string,
) (ActiveRuntimeBindingEvidence, error) {
	r.once.Do(func() {
		values, err := r.module.activeRuntimeBindingEvidence.BindingEvidence(
			ctx, r.servingStateID, r.workspaceID,
		)
		if err != nil {
			r.err = err
			return
		}
		r.evidence = make(map[string]ActiveRuntimeBindingEvidence, len(values))
		for _, value := range values {
			value.BindingID = strings.TrimSpace(value.BindingID)
			value.LogicalConnection = strings.TrimSpace(value.LogicalConnection)
			value.ConnectorKind = strings.TrimSpace(value.ConnectorKind)
			value.ValidatedVersion = strings.TrimSpace(value.ValidatedVersion)
			value.EndpointConfigHash = strings.TrimSpace(value.EndpointConfigHash)
			if value.LogicalConnection == "" || value.BindingID == "" || value.ConnectorKind == "" ||
				value.Revision < 1 || value.ValidatedVersion == "" ||
				platformdigest.ValidateSHA256Identity(value.EndpointConfigHash) != nil {
				r.err = fmt.Errorf("%w: active binding evidence is invalid", connectionbinding.ErrIncompatibleBinding)
				return
			}
			if _, exists := r.evidence[value.LogicalConnection]; exists {
				r.err = fmt.Errorf("%w: duplicate active binding evidence", connectionbinding.ErrIncompatibleBinding)
				return
			}
			r.evidence[value.LogicalConnection] = value
		}
	})
	if r.err != nil {
		return ActiveRuntimeBindingEvidence{}, r.err
	}
	evidence, ok := r.evidence[strings.TrimSpace(name)]
	if !ok {
		return ActiveRuntimeBindingEvidence{}, connectionbinding.ErrBindingNotFound
	}
	return evidence, nil
}
