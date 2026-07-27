package runtimefactory

import (
	projectbundle "github.com/Yacobolo/leapview/internal/project/bundle"
	refreshrun "github.com/Yacobolo/leapview/internal/refresh/run"
)

func NewRefreshArtifactLoader() refreshrun.ArtifactLoader {
	return projectbundle.RefreshArtifactLoader{}
}
