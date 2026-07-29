package runtimefactory

import (
	projectbundle "github.com/flidai/leapview/internal/project/bundle"
	refreshrun "github.com/flidai/leapview/internal/refresh/run"
)

func NewRefreshArtifactLoader() refreshrun.ArtifactLoader {
	return projectbundle.RefreshArtifactLoader{}
}
