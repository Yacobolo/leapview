package compiler

import (
	"strings"

	"github.com/Yacobolo/libredash/internal/semantic"
	"github.com/Yacobolo/libredash/internal/workspace"
)

type Options struct {
	WorkspaceID  workspace.WorkspaceID
	DeploymentID workspace.DeploymentID
}

type CompiledWorkspace struct {
	Workspace  workspace.Workspace
	Definition *workspace.Definition
}

func Compile(catalogPath string, opts Options) (CompiledWorkspace, error) {
	definition, err := semantic.LoadWorkspace(catalogPath)
	if err != nil {
		return CompiledWorkspace{}, err
	}
	workspaceID := opts.WorkspaceID
	if workspaceID == "" {
		workspaceID = workspace.WorkspaceID(workspaceIDOrDefault(definition.Catalog.Workspace.ID))
	}
	graph, err := ExtractLineage(workspaceID, opts.DeploymentID, definition)
	if err != nil {
		return CompiledWorkspace{}, err
	}
	return CompiledWorkspace{
		Workspace: workspace.Workspace{
			ID:          workspaceID,
			Title:       workspaceTitle(definition.Catalog.Workspace.Title),
			Description: definition.Catalog.Workspace.Description,
			BaseDir:     definition.BaseDir,
			Graph:       graph,
		},
		Definition: definition,
	}, nil
}

func workspaceIDOrDefault(value string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return "libredash"
}

func workspaceTitle(value string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return "LibreDash Workspace"
}
