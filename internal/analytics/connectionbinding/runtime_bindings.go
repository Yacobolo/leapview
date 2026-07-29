package connectionbinding

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type RuntimeBindingAuthorizer func(context.Context, string, TargetBinding) error

type RuntimeBindingLeaserConfig struct {
	Bindings  BindingCatalog
	Pools     ValidatedPoolDirectory
	Authorize RuntimeBindingAuthorizer
}

type RuntimeBindingLeaser struct {
	bindings  BindingCatalog
	pools     ValidatedPoolDirectory
	authorize RuntimeBindingAuthorizer
}

type RuntimeBindingRequest struct {
	Actor        string
	Scope        BindingScope
	TargetID     string
	Requirements []Requirement
}

// RuntimeBindingLeases holds target-owned pool generations for the lifetime of
// one candidate runtime. It contains only non-secret validation evidence.
type RuntimeBindingLeases struct {
	once     sync.Once
	leases   []ValidatedPoolLease
	evidence []BindingEvidence
}

func NewRuntimeBindingLeaser(config RuntimeBindingLeaserConfig) (*RuntimeBindingLeaser, error) {
	if config.Bindings == nil || config.Pools == nil || config.Authorize == nil {
		return nil, fmt.Errorf(
			"%w: binding catalog, validated pool directory, and authorizer are required",
			ErrInvalidBinding,
		)
	}
	return &RuntimeBindingLeaser{
		bindings: config.Bindings, pools: config.Pools, authorize: config.Authorize,
	}, nil
}

func (leaser *RuntimeBindingLeaser) Acquire(
	ctx context.Context,
	request RuntimeBindingRequest,
) (_ *RuntimeBindingLeases, resultErr error) {
	if leaser == nil {
		return nil, ErrProviderUnavailable
	}
	request.Actor = strings.TrimSpace(request.Actor)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Scope.WorkspaceID = strings.TrimSpace(request.Scope.WorkspaceID)
	request.Scope.Environment = strings.TrimSpace(request.Scope.Environment)
	if request.Actor == "" || request.TargetID == "" ||
		request.Scope.WorkspaceID == "" || request.Scope.Environment == "" {
		return nil, fmt.Errorf(
			"%w: actor, target, workspace, and environment are required",
			ErrInvalidBinding,
		)
	}
	requirements, err := normalizeRuntimeRequirements(request.Requirements)
	if err != nil {
		return nil, err
	}
	result := &RuntimeBindingLeases{
		leases:   make([]ValidatedPoolLease, 0, len(requirements)),
		evidence: make([]BindingEvidence, 0, len(requirements)),
	}
	defer func() {
		if resultErr != nil {
			result.Release()
		}
	}()
	for _, requirement := range requirements {
		binding, err := leaser.bindings.Binding(
			ctx,
			request.Scope,
			request.TargetID,
			requirement.LogicalConnectionID,
		)
		if err != nil {
			return nil, err
		}
		if binding.TargetID != request.TargetID || binding.Scope != request.Scope {
			return nil, ErrBindingNotFound
		}
		if err := leaser.authorize(ctx, request.Actor, binding); err != nil {
			return nil, ErrUnauthorizedBinding
		}
		if _, err := binding.CompatibleEvidence(requirement, true); err != nil {
			return nil, err
		}
		lease, err := leaser.pools.AcquireValidated(ctx, binding, request.Actor)
		if err != nil {
			return nil, err
		}
		evidence := lease.Evidence()
		if err := validateRuntimeBindingEvidence(binding, requirement, evidence); err != nil {
			lease.Release()
			return nil, err
		}
		result.leases = append(result.leases, lease)
		result.evidence = append(result.evidence, evidence)
	}
	return result, nil
}

func (leases *RuntimeBindingLeases) Evidence() []BindingEvidence {
	if leases == nil {
		return nil
	}
	return append([]BindingEvidence(nil), leases.evidence...)
}

func (leases *RuntimeBindingLeases) Release() {
	if leases == nil {
		return
	}
	leases.once.Do(func() {
		for index := len(leases.leases) - 1; index >= 0; index-- {
			leases.leases[index].Release()
		}
		leases.leases = nil
	})
}

func normalizeRuntimeRequirements(requirements []Requirement) ([]Requirement, error) {
	normalized := append([]Requirement(nil), requirements...)
	for index := range normalized {
		logical, err := ParseLogicalConnectionID(
			strings.TrimSpace(normalized[index].LogicalConnectionID.String()),
		)
		if err != nil {
			return nil, err
		}
		normalized[index].LogicalConnectionID = logical
		normalized[index].ConnectorKind = strings.TrimSpace(normalized[index].ConnectorKind)
		normalized[index].ValidatedVersion = strings.TrimSpace(normalized[index].ValidatedVersion)
		if normalized[index].ConnectorKind == "" || normalized[index].BindingRevision < 0 {
			return nil, fmt.Errorf(
				"%w: connector kind and non-negative binding revision are required",
				ErrInvalidBinding,
			)
		}
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].LogicalConnectionID < normalized[j].LogicalConnectionID
	})
	for index := 1; index < len(normalized); index++ {
		if normalized[index-1].LogicalConnectionID == normalized[index].LogicalConnectionID {
			return nil, fmt.Errorf(
				"%w: duplicate runtime requirement %q",
				ErrInvalidBinding,
				normalized[index].LogicalConnectionID,
			)
		}
	}
	return normalized, nil
}

func validateRuntimeBindingEvidence(
	binding TargetBinding,
	requirement Requirement,
	evidence BindingEvidence,
) error {
	if evidence.BindingID != binding.ID ||
		evidence.TargetID != binding.TargetID ||
		evidence.LogicalConnection != binding.LogicalConnectionID ||
		evidence.ConnectorKind != binding.ConnectorKind ||
		evidence.Scope != binding.Scope ||
		evidence.BindingRevision < 1 ||
		strings.TrimSpace(evidence.ValidatedVersion) == "" ||
		evidence.Health == HealthDisabled {
		return ErrIncompatibleBinding
	}
	if requirement.BindingRevision > 0 &&
		requirement.BindingRevision != evidence.BindingRevision {
		return ErrIncompatibleBinding
	}
	if requirement.ValidatedVersion != "" &&
		requirement.ValidatedVersion != evidence.ValidatedVersion {
		return ErrIncompatibleBinding
	}
	return nil
}
