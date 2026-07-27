package runtimefactory

import (
	"context"

	dashboarddefinition "github.com/Yacobolo/leapview/internal/dashboard/definition"
	dashboardruntime "github.com/Yacobolo/leapview/internal/dashboard/runtime"
)

type Input struct {
	Directory, ServingStateID, WorkspaceID, Environment   string
	SemanticModelDigest, ArtifactDigest, SourceDataDigest string
	SnapshotID                                            int64
	Definition                                            *dashboarddefinition.Workspace
}

type Builder func(context.Context, Input) (*dashboardruntime.Service, error)
