package analyticsduckdb

import (
	"fmt"
	"sort"

	manageddataruntimebinding "github.com/Yacobolo/leapview/internal/manageddata/runtimebinding"
	refreshartifact "github.com/Yacobolo/leapview/internal/refresh/artifact"
)

type workspaceBindingTarget struct {
	definition *refreshartifact.Definition
}

func bindManagedDataRoots(definition *refreshartifact.Definition, roots map[string]string) error {
	if definition == nil {
		return fmt.Errorf("workspace definition is required")
	}
	return manageddataruntimebinding.BindRoots(workspaceBindingTarget{definition: definition}, roots)
}

func (t workspaceBindingTarget) ManagedConnections() []manageddataruntimebinding.Connection {
	var connections []manageddataruntimebinding.Connection
	for modelID, model := range t.definition.Models {
		if model == nil {
			continue
		}
		for name, connection := range model.Connections {
			if connection.Kind == "managed" {
				connections = append(connections, manageddataruntimebinding.Connection{ModelID: modelID, Name: name})
			}
		}
	}
	sort.Slice(connections, func(i, j int) bool {
		if connections[i].ModelID == connections[j].ModelID {
			return connections[i].Name < connections[j].Name
		}
		return connections[i].ModelID < connections[j].ModelID
	})
	return connections
}

func (t workspaceBindingTarget) BindManagedRoot(ref manageddataruntimebinding.Connection, root string) error {
	model := t.definition.Models[ref.ModelID]
	if model == nil {
		return fmt.Errorf("semantic model %q is unavailable while binding managed data", ref.ModelID)
	}
	connection, ok := model.Connections[ref.Name]
	if !ok || connection.Kind != "managed" {
		return fmt.Errorf("semantic model %q managed connection %q is unavailable", ref.ModelID, ref.Name)
	}
	connection.Root = root
	connection.Scope = ""
	model.Connections[ref.Name] = connection
	return nil
}
