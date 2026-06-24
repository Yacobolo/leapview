package compiler

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
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
	definition, err := CompileDefinition(catalogPath)
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

func CompileDefinition(catalogPath string) (*workspace.Definition, error) {
	catalog, baseDir, err := workspace.LoadCatalog(catalogPath)
	if err != nil {
		return nil, err
	}
	definition := &workspace.Definition{
		Catalog:    catalog,
		Models:     map[string]*model.Model{},
		Dashboards: map[string]*report.Dashboard{},
		BaseDir:    baseDir,
	}

	for _, entry := range catalog.SemanticModels {
		model, err := semantic.Load(filepath.Join(baseDir, entry.Path))
		if err != nil {
			return nil, fmt.Errorf("loading semantic model %q: %w", entry.ID, err)
		}
		if model.Name != entry.ID {
			return nil, fmt.Errorf("catalog model %q path loads model %q", entry.ID, model.Name)
		}
		definition.Models[entry.ID] = model
	}

	for _, entry := range catalog.Dashboards {
		dashboard, err := semantic.LoadDashboard(filepath.Join(baseDir, entry.Path))
		if err != nil {
			return nil, fmt.Errorf("loading dashboard %q: %w", entry.ID, err)
		}
		if dashboard.ID != entry.ID {
			return nil, fmt.Errorf("catalog dashboard %q path loads dashboard %q", entry.ID, dashboard.ID)
		}
		if err := ValidateDashboard(dashboard, definition.Models); err != nil {
			return nil, fmt.Errorf("loading dashboard %q: %w", entry.ID, err)
		}
		definition.Dashboards[entry.ID] = dashboard
	}

	return definition, nil
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
