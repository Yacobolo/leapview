package compiler

import (
	"fmt"
	"sort"
	"strings"

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

func Compile(projectPath string, opts Options) (CompiledWorkspace, error) {
	compiled, err := CompileProject(projectPath, opts)
	if err != nil {
		return CompiledWorkspace{}, err
	}
	workspaceID := opts.WorkspaceID
	if workspaceID == "" {
		return firstCompiledWorkspace(projectPath, compiled)
	}
	selected, ok := compiled.Workspaces[string(workspaceID)]
	if !ok {
		return CompiledWorkspace{}, fmt.Errorf("project %q has no workspace %q", projectPath, workspaceID)
	}
	return selected, nil
}

func firstCompiledWorkspace(projectPath string, compiled CompiledProject) (CompiledWorkspace, error) {
	ids := make([]string, 0, len(compiled.Workspaces))
	for id := range compiled.Workspaces {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return CompiledWorkspace{}, fmt.Errorf("project %q has no workspaces", projectPath)
	}
	return compiled.Workspaces[ids[0]], nil
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
