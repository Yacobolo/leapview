package module

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/flidai/leapview/internal/deployment"
	"github.com/flidai/leapview/internal/deployment/apiadapter"
	deploymenthttp "github.com/flidai/leapview/internal/deployment/http"
)

type Module struct {
	handler           *deploymenthttp.Handler
	candidates        *deployment.CandidateService
	candidateRuntimes *deployment.CandidateRuntimeService
	candidateSources  deployment.CandidateSourceSynchronizer
	jobs              JobConfig
	api               APIConfig
}

type Principal struct {
	ID string
}

type CandidateEvent = deployment.CandidateEvent
type CandidateConnectionRequest = deployment.CandidateConnectionRequest
type CandidateConnectionEvidence = deployment.CandidateConnectionEvidence
type CandidateConnectionLeases = deployment.CandidateConnectionLeases
type CandidateRuntimeRequest = deployment.CandidateRuntimeRequest
type CandidateWorkspaceRuntime = deployment.CandidateWorkspaceRuntime
type CandidateConnectionRequirement = deployment.CandidateConnectionRequirement
type CandidateDataMode = deployment.CandidateDataMode

const (
	CandidateDataReuseSnapshot  = deployment.CandidateDataReuseSnapshot
	CandidateDataRefreshSources = deployment.CandidateDataRefreshSources
)

type ServingStatePort interface {
	deployment.ServingStateRepository
}

type Config struct {
	Database                 *sql.DB
	States                   ServingStatePort
	Runtime                  deployment.Runtime
	ManagedData              deployment.ManagedDataResolver
	DeploymentMetadata       apiadapter.Metadata
	ActivationHooks          ActivationHooks
	MaxJSONBodyBytes         int64
	Logger                   *slog.Logger
	InstanceID               string
	CanonicalOrigin          string
	InstanceEnvironment      string
	CandidateLifetime        time.Duration
	MaxCandidatesPerOwner    int
	CandidateAudit           func(context.Context, deployment.CandidateEvent) error
	CandidateConnections     deployment.CandidateConnectionLeaser
	CandidateRuntime         deployment.CandidateRuntimeHost
	CandidateSources         deployment.CandidateSourceSynchronizer
	RuntimeVersion           string
	CurrentPrincipal         func(*http.Request) (Principal, bool)
	Jobs                     JobConfig
	API                      APIConfig
	PublicationAuthorization PublicationAuthorizationConfig
}

func Build(_ context.Context, config Config) (*Module, error) {
	options := deploymenthttp.Options{MaxJSONBodyBytes: config.MaxJSONBodyBytes}
	options.CurrentPrincipal = func(r *http.Request) (deploymenthttp.Principal, bool) {
		if config.CurrentPrincipal == nil {
			return deploymenthttp.Principal{}, false
		}
		principal, ok := config.CurrentPrincipal(r)
		return deploymenthttp.Principal{ID: principal.ID}, ok
	}
	var coordinator deploymenthttp.Coordinator
	var candidates *deployment.CandidateService
	var candidateRuntimes *deployment.CandidateRuntimeService
	if config.Database != nil {
		if config.States == nil || config.Runtime == nil || config.ManagedData == nil || config.DeploymentMetadata == nil {
			return nil, errors.New("deployment states, runtime, managed data, and metadata are required")
		}
		repository, activation, candidateRepository := newPersistence(config.Database, config.ActivationHooks, config.API.Releases, config.API.Workflow)
		service, err := deployment.New(repository, activation, config.States, config.Runtime, config.ManagedData)
		if err != nil {
			return nil, err
		}
		if config.CandidateConnections != nil || config.CandidateRuntime != nil {
			candidateRuntimes, err = deployment.NewCandidateRuntimeService(
				deployment.CandidateRuntimeServiceConfig{
					Connections:    config.CandidateConnections,
					Runtime:        config.CandidateRuntime,
					RuntimeVersion: config.RuntimeVersion,
				},
			)
			if err != nil {
				return nil, err
			}
		}
		coordinator, err = apiadapter.New(service, config.DeploymentMetadata)
		if err != nil {
			return nil, err
		}
		candidates, err = deployment.NewCandidateService(candidateRepository, deployment.CandidateServiceConfig{
			TargetID: config.InstanceID, CanonicalOrigin: config.CanonicalOrigin,
			Environment: config.InstanceEnvironment, Lifetime: config.CandidateLifetime,
			MaxActivePerOwner: config.MaxCandidatesPerOwner, Audit: config.CandidateAudit,
		})
		if err != nil {
			return nil, err
		}
	}
	options.Coordinator = coordinator
	options.Logger = config.Logger
	options.InstanceEnvironment = config.InstanceEnvironment
	jobs := config.Jobs
	if jobs.Coordinator == nil {
		jobs.Coordinator = coordinator
	}
	m := &Module{
		handler: deploymenthttp.NewHandler(options), candidates: candidates,
		candidateRuntimes: candidateRuntimes, candidateSources: config.CandidateSources,
		jobs: jobs, api: config.API,
	}
	if m.jobs.Authorize == nil {
		m.jobs.Authorize = m.publicationAuthorizer(config.PublicationAuthorization)
	}
	return m, nil
}

func (m *Module) HTTP() *deploymenthttp.Handler { return m.handler }

func (m *Module) PrepareCandidateRuntime(
	ctx context.Context,
	request deployment.CandidateRuntimeRequest,
) error {
	if m == nil || m.candidateRuntimes == nil {
		return deployment.ErrCandidateUnavailable
	}
	return m.candidateRuntimes.Prepare(ctx, request)
}
