package module

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Yacobolo/leapview/internal/deployment"
	"github.com/Yacobolo/leapview/internal/deployment/apiadapter"
	deploymenthttp "github.com/Yacobolo/leapview/internal/deployment/http"
)

type Module struct {
	handler *deploymenthttp.Handler
	jobs    JobConfig
	api     APIConfig
}

type Principal struct {
	ID string
}

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
	InstanceEnvironment      string
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
	if config.Database != nil {
		if config.States == nil || config.Runtime == nil || config.ManagedData == nil || config.DeploymentMetadata == nil {
			return nil, errors.New("deployment states, runtime, managed data, and metadata are required")
		}
		service, err := deployment.New(newRepository(config.Database, config.ActivationHooks), config.States, config.Runtime, config.ManagedData)
		if err != nil {
			return nil, err
		}
		coordinator, err = apiadapter.New(service, config.DeploymentMetadata)
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
	m := &Module{handler: deploymenthttp.NewHandler(options), jobs: jobs, api: config.API}
	if m.jobs.Authorize == nil {
		m.jobs.Authorize = m.publicationAuthorizer(config.PublicationAuthorization)
	}
	return m, nil
}

func (m *Module) HTTP() *deploymenthttp.Handler { return m.handler }
