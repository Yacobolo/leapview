// Package plan turns compiled refresh definitions into deterministic execution plans.
package plan

import (
	"fmt"
	"strings"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	refreshartifact "github.com/Yacobolo/leapview/internal/refresh/artifact"
)

type Plan struct {
	TargetType       string
	TargetID         string
	ModelID          string
	Tables           []string
	DependencyTables []string
}

func ForPipeline(definition *refreshartifact.Definition, workspaceID, pipelineID string) (Plan, error) {
	if definition == nil {
		return Plan{}, fmt.Errorf("workspace definition is required")
	}
	pipelineID = strings.TrimSpace(pipelineID)
	pipeline, ok := definition.Pipelines[pipelineID]
	if !ok {
		return Plan{}, fmt.Errorf("unknown refresh pipeline %q", pipelineID)
	}
	model, ok := definition.Models[pipeline.SemanticModel]
	if !ok {
		return Plan{}, fmt.Errorf("refresh pipeline %q references unknown semantic model %q", pipelineID, pipeline.SemanticModel)
	}
	order, err := modelTableOrder(model)
	if err != nil {
		return Plan{}, err
	}
	targetID := strings.TrimSpace(workspaceID) + "." + pipelineID
	if _, err := localWorkspaceAssetName(workspaceID, targetID); err != nil {
		return Plan{}, err
	}
	return Plan{
		TargetType:       "refresh_pipeline",
		TargetID:         targetID,
		ModelID:          pipeline.SemanticModel,
		Tables:           order,
		DependencyTables: append([]string(nil), order...),
	}, nil
}

func modelTableOrder(model *semanticmodel.Model) ([]string, error) {
	if model == nil {
		return nil, fmt.Errorf("semantic model is required")
	}
	temporary := map[string]bool{}
	permanent := map[string]bool{}
	order := make([]string, 0, len(model.Tables))
	var visit func(string) error
	visit = func(name string) error {
		if permanent[name] {
			return nil
		}
		if temporary[name] {
			return fmt.Errorf("model table dependency cycle includes %q", name)
		}
		table, ok := model.Tables[name]
		if !ok {
			return fmt.Errorf("unknown model table %q", name)
		}
		temporary[name] = true
		for _, dependency := range table.ModelDependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(temporary, name)
		permanent[name] = true
		order = append(order, name)
		return nil
	}
	for _, name := range model.TableNames() {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func localWorkspaceAssetName(workspaceID, key string) (string, error) {
	prefix := strings.TrimSpace(workspaceID) + "."
	key = strings.TrimSpace(key)
	if prefix == "." {
		return "", fmt.Errorf("workspace id is required")
	}
	if !strings.HasPrefix(key, prefix) {
		return "", fmt.Errorf("asset key %q is not in workspace %q", key, workspaceID)
	}
	name := strings.TrimSpace(strings.TrimPrefix(key, prefix))
	if name == "" {
		return "", fmt.Errorf("asset key %q is missing a local name", key)
	}
	return name, nil
}
