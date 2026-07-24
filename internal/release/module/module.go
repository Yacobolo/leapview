package module

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Yacobolo/leapview/internal/platform/jobs"
	"github.com/Yacobolo/leapview/internal/release"
	releasefilesystem "github.com/Yacobolo/leapview/internal/release/filesystem"
	releasesqlite "github.com/Yacobolo/leapview/internal/release/sqlite"
	"github.com/Yacobolo/leapview/internal/servingstate"
	"github.com/Yacobolo/leapview/internal/servingstate/validate"
)

type Module struct {
	service     *release.Service
	catalog     release.CatalogRepository
	deployments release.DeploymentLinkage
	environment string
	api         APIConfig
}

type Config struct {
	Database          *sql.DB
	States            ServingStateRepository
	Workspaces        WorkspaceProvisioner
	ManagedDataPins   release.PinValidator
	ManagedDataHook   validate.Hook
	ArtifactDirectory string
	Environment       servingstate.Environment
	API               APIConfig
}

type ServingStateRepository interface {
	release.ServingStateRepository
	validate.Repository
}

type WorkspaceProvisioner interface {
	release.WorkspaceRepository
}

func Build(_ context.Context, config Config) (*Module, error) {
	releases, finalization, catalog, deployments, err := releaseStores(config.Database, config.API.Workflow)
	if err != nil {
		return nil, err
	}
	store := releasefilesystem.NewArtifactStore(config.ArtifactDirectory)
	hooks := []validate.Hook{}
	if config.ManagedDataHook != nil {
		hooks = append(hooks, config.ManagedDataHook)
	}
	validator := validate.NewService(config.States, store, releasefilesystem.Validator{}, hooks...)
	service, err := release.NewService(release.ServiceOptions{
		Releases: releases, Finalization: finalization, States: config.States, Workspaces: config.Workspaces,
		Artifacts: store, Validator: validator, Pins: config.ManagedDataPins, Environment: config.Environment,
	})
	if err != nil {
		return nil, err
	}
	return &Module{
		service: service, catalog: catalog, deployments: deployments,
		environment: string(config.Environment), api: config.API,
	}, nil
}

func releaseStores(database *sql.DB, workflow ...jobs.WorkflowRecorder) (release.Repository, release.FinalizationUnitOfWork, release.CatalogRepository, release.DeploymentLinkage, error) {
	if database == nil {
		return nil, nil, nil, nil, errors.New("release database is required")
	}
	var recorder jobs.WorkflowRecorder
	if len(workflow) > 0 {
		recorder = workflow[0]
	}
	owned := releasesqlite.NewRepositoryWithWorkflow(database, recorder)
	return owned, owned, owned, owned, nil
}

func (m *Module) DeploymentLinkage() release.DeploymentLinkage {
	if m == nil {
		return nil
	}
	return m.deployments
}
