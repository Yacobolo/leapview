package agenttools

import (
	"net/http"
	"sort"
	"strings"

	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
)

const ExtensionKey = "x-agent"

const QueryVisualToolName = "query_visual"

type Extension struct {
	Enabled      bool
	Name         string
	Risk         string
	Tags         []string
	DefaultLimit int
}

type APIGenOperation struct {
	Contract  apigenapi.GenOperationContract
	Extension Extension
}

func APIGenOperations() []APIGenOperation {
	spec, err := apigenapi.GetEmbeddedOpenAPISpec()
	if err != nil {
		return nil
	}
	paths, _ := spec["paths"].(map[string]any)
	contracts := apigenapi.GetAPIGenOperationContracts()
	operations := make([]APIGenOperation, 0, len(contracts))
	for _, contract := range contracts {
		extension, ok := ParseExtension(contract.Extensions[ExtensionKey])
		if !ok || !operationAllowed(contract, extension) || !hasOpenAPIOperation(paths, contract) {
			continue
		}
		operations = append(operations, APIGenOperation{Contract: contract, Extension: extension})
	}
	sort.Slice(operations, func(i, j int) bool {
		return operations[i].Extension.Name < operations[j].Extension.Name
	})
	return operations
}

func APIGenToolNames() []string {
	operations := APIGenOperations()
	names := make([]string, 0, len(operations))
	for _, operation := range operations {
		names = append(names, operation.Extension.Name)
	}
	return names
}

func ManualToolNames() []string {
	return []string{QueryVisualToolName}
}

func ToolNames() []string {
	names := append([]string{}, APIGenToolNames()...)
	names = append(names, ManualToolNames()...)
	sort.Strings(names)
	return names
}

func IsKnownTool(name string) bool {
	for _, tool := range ToolNames() {
		if tool == name {
			return true
		}
	}
	return false
}

func ParseExtension(value any) (Extension, bool) {
	raw, ok := value.(map[string]any)
	if !ok {
		return Extension{}, false
	}
	extension := Extension{
		Enabled:      boolFromMap(raw, "enabled"),
		Name:         stringFromMap(raw, "name"),
		Risk:         stringFromMap(raw, "risk"),
		DefaultLimit: intFromMap(raw, "defaultLimit"),
	}
	if tags, ok := raw["tags"].([]any); ok {
		for _, tag := range tags {
			if text, ok := tag.(string); ok && text != "" {
				extension.Tags = append(extension.Tags, text)
			}
		}
	}
	return extension, true
}

func operationAllowed(contract apigenapi.GenOperationContract, extension Extension) bool {
	if !extension.Enabled || extension.Name == "" || extension.Risk != "read" {
		return false
	}
	if contract.Manual {
		return false
	}
	if contract.Method != http.MethodGet && contract.Method != http.MethodPost {
		return false
	}
	switch operationPermission(contract) {
	case "workspace:read", "asset:read", "deployment:read", "materialization:run":
		return true
	default:
		return false
	}
}

func operationPermission(contract apigenapi.GenOperationContract) string {
	raw, _ := contract.Extensions["x-authz"].(map[string]any)
	return stringFromMap(raw, "permission")
}

func hasOpenAPIOperation(paths map[string]any, contract apigenapi.GenOperationContract) bool {
	pathItem, ok := paths[contract.Path].(map[string]any)
	if !ok {
		return false
	}
	_, ok = pathItem[strings.ToLower(contract.Method)].(map[string]any)
	return ok
}

func boolFromMap(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func stringFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func intFromMap(values map[string]any, key string) int {
	switch value := values[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
