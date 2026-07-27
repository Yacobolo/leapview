package compiler

import (
	"fmt"
	"strings"

	"github.com/Yacobolo/leapview/internal/brand"
	projectartifact "github.com/Yacobolo/leapview/internal/project/artifact"
	"github.com/Yacobolo/leapview/internal/workspace"
)

type Options struct {
	WorkspaceID    workspace.WorkspaceID
	ServingStateID workspace.ServingStateID
}

func Compile(projectPath string, opts Options) (projectartifact.Workspace, error) {
	compiled, err := CompileProject(projectPath, opts)
	if err != nil {
		return projectartifact.Workspace{}, err
	}
	workspaceID := opts.WorkspaceID
	if workspaceID == "" {
		return projectartifact.Workspace{}, fmt.Errorf("workspace id is required")
	}
	selected, ok := compiled.Workspace(string(workspaceID))
	if !ok {
		return projectartifact.Workspace{}, fmt.Errorf("project %q has no workspace %q", projectPath, workspaceID)
	}
	return selected, nil
}

func workspaceTitle(value, workspaceID string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	if strings.TrimSpace(workspaceID) != "" {
		return workspaceID
	}
	return brand.Name
}
