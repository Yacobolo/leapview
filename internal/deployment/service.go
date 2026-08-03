package deployment

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/flidai/leapview/internal/platform/digest"
	"github.com/flidai/leapview/internal/runtimehost"
	servingstate "github.com/flidai/leapview/internal/servingstate"
)

type Repository interface {
	CreateDeployment(context.Context, CreateInput) (Deployment, error)
	DeploymentByID(context.Context, string) (Deployment, error)
	CancelDeployment(context.Context, string) (Deployment, error)
	FailDeployment(context.Context, string, error) error
}

// ActivationUnitOfWork atomically validates the prepared deployment, advances
// managed-data and serving-state pointers, installs access/publication
// projections, and marks the deployment active.
type ActivationUnitOfWork interface {
	ActivateDeployment(context.Context, ActivationInput) (Deployment, error)
}

func (s *Service) Cancel(ctx context.Context, scope Scope) (Deployment, error) {
	row, err := s.Get(ctx, scope)
	if err != nil {
		return Deployment{}, err
	}
	if row.Status == StatusCancelled {
		return row, nil
	}
	if row.Status != StatusPending {
		return Deployment{}, fmt.Errorf("%w: deployment is %s", ErrConflict, row.Status)
	}
	return s.repository.CancelDeployment(ctx, row.ID)
}

type ServingStateRepository interface {
	RecordDuckLakeSnapshot(context.Context, servingstate.ID, int64) error
}

type ManagedDataResolver interface {
	ResolveManagedData(context.Context, servingstate.ID) (runtimehost.ManagedDataResolution, error)
}

type Prepared interface {
	Snapshots() []runtimehost.PreparedSnapshot
	Close() error
}

type Runtime interface {
	Prepare(context.Context, []runtimehost.ServingStateCandidate) (Prepared, error)
	Verify(context.Context, Prepared) (Verification, error)
	Activate(Prepared, func() error) error
}

type runtimeRegistry interface {
	PrepareServingStateCandidates(context.Context, []runtimehost.ServingStateCandidate) (*runtimehost.PreparedSet, error)
	VerifyPreparedSet(context.Context, *runtimehost.PreparedSet) (runtimehost.PreparedVerification, error)
	ActivatePreparedSet(*runtimehost.PreparedSet, func() error) error
}

type registryRuntime struct{ registry runtimeRegistry }
type registryPrepared struct{ set *runtimehost.PreparedSet }

func NewRegistryRuntime(registry runtimeRegistry) (Runtime, error) {
	if registry == nil {
		return nil, fmt.Errorf("runtime registry is required")
	}
	return registryRuntime{registry: registry}, nil
}

func (r registryRuntime) Prepare(ctx context.Context, candidates []runtimehost.ServingStateCandidate) (Prepared, error) {
	set, err := r.registry.PrepareServingStateCandidates(ctx, candidates)
	if err != nil {
		return nil, err
	}
	return registryPrepared{set: set}, nil
}

func (r registryRuntime) Activate(prepared Prepared, activate func() error) error {
	value, ok := prepared.(registryPrepared)
	if !ok || value.set == nil {
		return fmt.Errorf("prepared runtimes belong to a different deployment coordinator")
	}
	return r.registry.ActivatePreparedSet(value.set, activate)
}

func (r registryRuntime) Verify(
	ctx context.Context,
	prepared Prepared,
) (Verification, error) {
	value, ok := prepared.(registryPrepared)
	if !ok || value.set == nil {
		return Verification{}, fmt.Errorf(
			"prepared runtimes belong to a different deployment coordinator",
		)
	}
	verification, err := r.registry.VerifyPreparedSet(ctx, value.set)
	if err != nil {
		return Verification{}, err
	}
	return Verification{Digest: verification.Digest}, nil
}

func (p registryPrepared) Snapshots() []runtimehost.PreparedSnapshot { return p.set.Snapshots() }
func (p registryPrepared) Close() error                              { return p.set.Close() }

type Service struct {
	repository Repository
	activation ActivationUnitOfWork
	states     ServingStateRepository
	runtime    Runtime
	resolver   ManagedDataResolver
}

func New(repository Repository, activation ActivationUnitOfWork, states ServingStateRepository, runtime Runtime, resolver ManagedDataResolver) (*Service, error) {
	if repository == nil || activation == nil || states == nil || runtime == nil || resolver == nil {
		return nil, fmt.Errorf("deployment repository, activation unit of work, serving-state repository, runtime, and managed-data resolver are required")
	}
	return &Service{repository: repository, activation: activation, states: states, runtime: runtime, resolver: resolver}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Deployment, error) {
	if err := validateCreate(input); err != nil {
		return Deployment{}, err
	}
	input.ID = strings.TrimSpace(input.ID)
	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.Environment = strings.TrimSpace(input.Environment)
	input.RequestDigest = strings.TrimSpace(input.RequestDigest)
	input.CreatedBy = strings.TrimSpace(input.CreatedBy)
	input.Targets = append([]TargetInput(nil), input.Targets...)
	sort.Slice(input.Targets, func(i, j int) bool { return input.Targets[i].WorkspaceID < input.Targets[j].WorkspaceID })
	return s.repository.CreateDeployment(ctx, input)
}

func (s *Service) Get(ctx context.Context, scope Scope) (Deployment, error) {
	projectID := strings.TrimSpace(scope.ProjectID)
	deploymentID := strings.TrimSpace(scope.DeploymentID)
	if projectID == "" || deploymentID == "" {
		return Deployment{}, fmt.Errorf("project and deployment id are required")
	}
	row, err := s.repository.DeploymentByID(ctx, deploymentID)
	if err != nil {
		return Deployment{}, err
	}
	if row.ID != deploymentID || row.ProjectID != projectID {
		return Deployment{}, ErrNotFound
	}
	return row, nil
}

func (s *Service) Activate(
	ctx context.Context,
	request ActivationRequest,
) (Deployment, error) {
	request.ActorID = strings.TrimSpace(request.ActorID)
	if request.ActorID == "" {
		return Deployment{}, fmt.Errorf("activation principal is required")
	}
	row, err := s.Get(ctx, request.Scope)
	if err != nil {
		return Deployment{}, err
	}
	if row.Status == StatusActive {
		return row, nil
	}
	if row.Status != StatusPending {
		return Deployment{}, fmt.Errorf("%w: deployment is %s", ErrConflict, row.Status)
	}

	targets := append([]Target(nil), row.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].WorkspaceID < targets[j].WorkspaceID })
	candidates := make([]runtimehost.ServingStateCandidate, 0, len(targets))
	for _, target := range targets {
		resolution, resolveErr := s.resolver.ResolveManagedData(ctx, servingstate.ID(target.ServingStateID))
		if resolveErr != nil {
			_ = s.repository.FailDeployment(ctx, row.ID, resolveErr)
			return Deployment{}, resolveErr
		}
		candidates = append(candidates, runtimehost.ServingStateCandidate{ServingStateID: target.ServingStateID, ManagedData: resolution})
	}

	prepared, err := s.runtime.Prepare(ctx, candidates)
	if err != nil {
		_ = s.repository.FailDeployment(ctx, row.ID, err)
		return Deployment{}, err
	}
	defer prepared.Close()
	for _, snapshot := range prepared.Snapshots() {
		if snapshot.DuckLakeSnapshotID <= 0 {
			continue
		}
		if err := s.states.RecordDuckLakeSnapshot(ctx, snapshot.ServingStateID, snapshot.DuckLakeSnapshotID); err != nil {
			_ = s.repository.FailDeployment(ctx, row.ID, err)
			return Deployment{}, err
		}
	}
	verification, err := s.runtime.Verify(ctx, prepared)
	if err != nil {
		_ = s.repository.FailDeployment(ctx, row.ID, err)
		return Deployment{}, err
	}
	if err := digest.ValidateSHA256Identity(verification.Digest); err != nil {
		invalid := fmt.Errorf("runtime verification returned invalid evidence: %w", err)
		_ = s.repository.FailDeployment(ctx, row.ID, invalid)
		return Deployment{}, invalid
	}

	var activated Deployment
	err = s.runtime.Activate(prepared, func() error {
		var activateErr error
		activated, activateErr = s.activation.ActivateDeployment(ctx, ActivationInput{
			DeploymentID: row.ID, ActivationPrincipal: request.ActorID,
			VerificationDigest: verification.Digest,
		})
		return activateErr
	})
	if err != nil {
		return Deployment{}, err
	}
	return activated, nil
}
