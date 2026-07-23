package module

import (
	"context"
	"database/sql"
	"errors"

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
	Releases          release.Repository
	Catalog           release.CatalogRepository
	DeploymentLinkage release.DeploymentLinkage
	States            ServingStateRepository
	Workspaces        release.WorkspaceRepository
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

func Build(_ context.Context, config Config) (*Module, error) {
	releases, catalog, deployments, err := releaseStores(config)
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
		Releases: releases, States: config.States, Workspaces: config.Workspaces,
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

func releaseStores(config Config) (release.Repository, release.CatalogRepository, release.DeploymentLinkage, error) {
	releases, catalog, deployments := config.Releases, config.Catalog, config.DeploymentLinkage
	if releases != nil && catalog != nil && deployments != nil {
		return releases, catalog, deployments, nil
	}
	if config.Database == nil {
		return nil, nil, nil, errors.New("release database is required when release stores are not supplied")
	}
	owned := releasesqlite.NewRepository(config.Database)
	if releases == nil {
		releases = owned
	}
	if catalog == nil {
		catalog = owned
	}
	if deployments == nil {
		deployments = owned
	}
	return releases, catalog, deployments, nil
}

func (m *Module) DeploymentLinkage() release.DeploymentLinkage {
	if m == nil {
		return nil
	}
	return m.deployments
}
