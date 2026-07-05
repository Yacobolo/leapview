package refresh

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/workspace"
)

const WorkspaceRefreshModelID = "workspace"

type Plan struct {
	TargetType       string
	TargetID         string
	ModelID          string
	Tables           []string
	DependencyTables []string
	ChildTrigger     string
}

func PlanForAsset(definition *workspace.Definition, workspaceID string, asset workspace.AssetView) (Plan, error) {
	if definition == nil {
		return Plan{}, fmt.Errorf("workspace definition is required")
	}
	targetID := AssetRefreshTargetID(asset)
	switch asset.Type {
	case string(workspace.AssetTypeSemanticModel):
		modelID, err := localWorkspaceAssetName(workspaceID, asset.Key)
		if err != nil {
			return Plan{}, err
		}
		model, ok := definition.Models[modelID]
		if !ok {
			return Plan{}, fmt.Errorf("unknown semantic model %q", modelID)
		}
		order, err := materialize.ModelTableOrder(model)
		if err != nil {
			return Plan{}, err
		}
		return Plan{
			TargetType:       materialize.TargetSemanticModel,
			TargetID:         targetID,
			ModelID:          modelID,
			Tables:           order,
			DependencyTables: order,
			ChildTrigger:     materialize.TriggerSemanticModel,
		}, nil
	case string(workspace.AssetTypeModelTable):
		tableName, err := localWorkspaceAssetName(workspaceID, asset.Key)
		if err != nil {
			return Plan{}, err
		}
		order, err := workspaceModelTableDependencyOrder(definition.Models, tableName)
		if err != nil {
			return Plan{}, err
		}
		dependencies := append([]string(nil), order...)
		if len(dependencies) > 0 && dependencies[len(dependencies)-1] == tableName {
			dependencies = dependencies[:len(dependencies)-1]
		}
		return Plan{
			TargetType:       materialize.TargetModelTable,
			TargetID:         targetID,
			ModelID:          WorkspaceRefreshModelID,
			Tables:           order,
			DependencyTables: dependencies,
			ChildTrigger:     materialize.TriggerDependency,
		}, nil
	default:
		return Plan{}, fmt.Errorf("asset type %q cannot be refreshed", asset.Type)
	}
}

func workspaceModelTableDependencyOrder(models map[string]*semanticmodel.Model, selectedTable string) ([]string, error) {
	model, err := physicalWorkspaceModel(models)
	if err != nil {
		return nil, err
	}
	return materialize.ModelTableDependencyOrder(model, selectedTable)
}

func physicalWorkspaceModel(models map[string]*semanticmodel.Model) (*semanticmodel.Model, error) {
	workspaceModel := &semanticmodel.Model{
		Name:              WorkspaceRefreshModelID,
		DefaultConnection: "",
		Connections:       map[string]semanticmodel.Connection{},
		Sources:           map[string]semanticmodel.Source{},
		Tables:            map[string]semanticmodel.Table{},
		Measures:          map[string]semanticmodel.MetricMeasure{},
	}
	for modelID, model := range models {
		if model == nil {
			return nil, fmt.Errorf("semantic model %q is required", modelID)
		}
		if workspaceModel.DefaultConnection == "" {
			workspaceModel.DefaultConnection = model.DefaultConnection
		}
		for name, connection := range model.Connections {
			existing, ok := workspaceModel.Connections[name]
			if ok && !reflect.DeepEqual(existing, connection) {
				return nil, fmt.Errorf("semantic model %q connection %q conflicts with another workspace model", modelID, name)
			}
			workspaceModel.Connections[name] = connection
		}
		for name, source := range model.Sources {
			existing, ok := workspaceModel.Sources[name]
			if ok && !reflect.DeepEqual(sourcePhysicalSignature(existing), sourcePhysicalSignature(source)) {
				return nil, fmt.Errorf("semantic model %q source %q conflicts with another workspace model", modelID, name)
			}
			workspaceModel.Sources[name] = source
		}
		for name, table := range model.Tables {
			existing, ok := workspaceModel.Tables[name]
			if ok && !reflect.DeepEqual(tablePhysicalSignature(existing), tablePhysicalSignature(table)) {
				return nil, fmt.Errorf("semantic model %q model table %q conflicts with another workspace model", modelID, name)
			}
			workspaceModel.Tables[name] = table
		}
	}
	return workspaceModel, nil
}

func sourcePhysicalSignature(source semanticmodel.Source) semanticmodel.Source {
	source.Description = ""
	source.Fields = nil
	source.Schema = semanticmodel.TableSchema{}
	return source
}

type tablePhysicalSignatureValue struct {
	Kind               string
	Source             string
	Sources            []string
	SQL                string
	Transform          semanticmodel.Transform
	Columns            map[string]semanticmodel.ModelColumn
	PrimaryKey         string
	Grain              string
	SourceDependencies []string
	ModelDependencies  []string
}

func tablePhysicalSignature(table semanticmodel.Table) tablePhysicalSignatureValue {
	return tablePhysicalSignatureValue{
		Kind:               table.Kind,
		Source:             table.Source,
		Sources:            append([]string{}, table.Sources...),
		SQL:                table.SQL,
		Transform:          table.Transform,
		Columns:            table.Columns,
		PrimaryKey:         table.PrimaryKey,
		Grain:              table.Grain,
		SourceDependencies: append([]string{}, table.SourceDependencies...),
		ModelDependencies:  append([]string{}, table.ModelDependencies...),
	}
}

func AssetRefreshTargetID(asset workspace.AssetView) string {
	return asset.Key
}

func AssetTypeForRefreshTarget(targetType string) string {
	switch targetType {
	case materialize.TargetModelTable:
		return string(workspace.AssetTypeModelTable)
	case materialize.TargetSemanticModel:
		return string(workspace.AssetTypeSemanticModel)
	default:
		return targetType
	}
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
