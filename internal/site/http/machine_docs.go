package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"

	content "github.com/Yacobolo/libredash/docs"
)

type machineDocumentation struct {
	cliManifest   []byte
	apiOperations []byte
	cliByID       map[string]json.RawMessage
	apiByID       map[string]json.RawMessage
	apiSchemas    map[string]json.RawMessage
	cli           []machineCLICommand
	api           []machineAPIOperation
}

type machineCLICommand struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Description  string   `json:"description"`
	Usage        string   `json:"usage"`
	Effect       string   `json:"effect"`
	Confirmation string   `json:"confirmation"`
	Arguments    []string `json:"arguments"`
}

type machineAPIOperation struct {
	OperationID   string                 `json:"operationId"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	Summary       string                 `json:"summary"`
	Description   string                 `json:"description"`
	Tags          []string               `json:"tags"`
	Effect        string                 `json:"effect"`
	Confirmation  string                 `json:"confirmation"`
	Authorization map[string]any         `json:"authorization"`
	Parameters    []machineAPIParameter  `json:"parameters"`
	RequestBody   *machineAPIRequestBody `json:"requestBody"`
	Responses     []machineAPIResponse   `json:"responses"`
}

type machineAPIParameter struct {
	Name        string         `json:"name"`
	In          string         `json:"in"`
	Required    bool           `json:"required"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type machineAPIRequestBody struct {
	Required    bool                `json:"required"`
	Description string              `json:"description"`
	Content     []machineAPIContent `json:"content"`
}

type machineAPIResponse struct {
	Status      string              `json:"status"`
	Description string              `json:"description"`
	Content     []machineAPIContent `json:"content"`
}

type machineAPIContent struct {
	ContentType string         `json:"contentType"`
	Schema      map[string]any `json:"schema"`
}

var machineDocs = loadMachineDocumentation()

func loadMachineDocumentation() machineDocumentation {
	cliManifest := mustReadDocumentationArtifact("reference/cli/manifest.json")
	apiOperations := mustReadDocumentationArtifact("api/operations.json")
	var cliManifestDecoded struct {
		Commands []json.RawMessage `json:"commands"`
	}
	if err := json.Unmarshal(cliManifest, &cliManifestDecoded); err != nil {
		panic(fmt.Sprintf("decode CLI machine manifest: %v", err))
	}
	var apiManifestDecoded struct {
		Operations []json.RawMessage          `json:"operations"`
		Schemas    map[string]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal(apiOperations, &apiManifestDecoded); err != nil {
		panic(fmt.Sprintf("decode API operation manifest: %v", err))
	}
	loaded := machineDocumentation{
		cliManifest: cliManifest, apiOperations: apiOperations,
		cliByID:    make(map[string]json.RawMessage, len(cliManifestDecoded.Commands)),
		apiByID:    make(map[string]json.RawMessage, len(apiManifestDecoded.Operations)),
		apiSchemas: apiManifestDecoded.Schemas,
	}
	for _, raw := range cliManifestDecoded.Commands {
		var command machineCLICommand
		if err := json.Unmarshal(raw, &command); err != nil {
			panic(fmt.Sprintf("decode CLI machine command: %v", err))
		}
		if command.ID == "" || loaded.cliByID[command.ID] != nil {
			panic(fmt.Sprintf("invalid or duplicate CLI machine command %q", command.ID))
		}
		loaded.cliByID[command.ID] = raw
		loaded.cli = append(loaded.cli, command)
	}
	for _, raw := range apiManifestDecoded.Operations {
		var operation machineAPIOperation
		if err := json.Unmarshal(raw, &operation); err != nil {
			panic(fmt.Sprintf("decode API machine operation: %v", err))
		}
		if operation.OperationID == "" || loaded.apiByID[operation.OperationID] != nil {
			panic(fmt.Sprintf("invalid or duplicate API machine operation %q", operation.OperationID))
		}
		loaded.apiByID[operation.OperationID] = raw
		loaded.api = append(loaded.api, operation)
	}
	return loaded
}

func mustReadDocumentationArtifact(name string) []byte {
	contents, err := content.Files.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("read generated documentation artifact %q: %v", name, err))
	}
	return contents
}

func docsCLIManifest(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	writeMachineArtifact(w, "application/json; charset=utf-8", machineDocs.cliManifest)
}

func docsAPIOperations(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	writeMachineArtifact(w, "application/json; charset=utf-8", machineDocs.apiOperations)
}

func docsCLICommand(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, format, ok := machineItemPath(r.PathValue("command"))
	if !ok {
		stdhttp.NotFound(w, r)
		return
	}
	raw, exists := machineDocs.cliByID[id]
	if !exists {
		stdhttp.NotFound(w, r)
		return
	}
	if format == "json" {
		writeMachineArtifact(w, "application/json; charset=utf-8", prettyJSON(raw))
		return
	}
	document, exists := siteDocumentBySlug("cli/" + id)
	if !exists {
		stdhttp.Error(w, "generated CLI article is missing", stdhttp.StatusInternalServerError)
		return
	}
	writeMachineArtifact(w, "text/markdown; charset=utf-8", []byte(document.markdown))
}

func focusedAPIOperationJSON(raw json.RawMessage) []byte {
	references := map[string]struct{}{}
	collectSchemaReferences(raw, references)
	schemas := map[string]json.RawMessage{}
	queue := make([]string, 0, len(references))
	for name := range references {
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if _, exists := schemas[name]; exists {
			continue
		}
		schema, exists := machineDocs.apiSchemas[name]
		if !exists {
			continue
		}
		schemas[name] = schema
		nested := map[string]struct{}{}
		collectSchemaReferences(schema, nested)
		for nestedName := range nested {
			if _, exists := schemas[nestedName]; !exists {
				queue = append(queue, nestedName)
			}
		}
	}
	contents, err := json.MarshalIndent(struct {
		SchemaVersion int                        `json:"schemaVersion"`
		Operation     json.RawMessage            `json:"operation"`
		Schemas       map[string]json.RawMessage `json:"schemas"`
	}{SchemaVersion: 1, Operation: raw, Schemas: schemas}, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("encode focused API operation: %v", err))
	}
	return append(contents, '\n')
}

func collectSchemaReferences(raw json.RawMessage, references map[string]struct{}) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		panic(fmt.Sprintf("decode generated schema references: %v", err))
	}
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if reference, ok := value["$ref"].(string); ok {
				const prefix = "#/components/schemas/"
				if strings.HasPrefix(reference, prefix) {
					references[strings.TrimPrefix(reference, prefix)] = struct{}{}
				}
			}
			for _, nested := range value {
				visit(nested)
			}
		case []any:
			for _, nested := range value {
				visit(nested)
			}
		}
	}
	visit(value)
}

func docsAPIOperation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	id, format, ok := machineItemPath(r.PathValue("operation"))
	if !ok {
		stdhttp.NotFound(w, r)
		return
	}
	raw, exists := machineDocs.apiByID[id]
	if !exists {
		stdhttp.NotFound(w, r)
		return
	}
	if format == "json" {
		writeMachineArtifact(w, "application/json; charset=utf-8", focusedAPIOperationJSON(raw))
		return
	}
	var operation machineAPIOperation
	if err := json.Unmarshal(raw, &operation); err != nil {
		stdhttp.Error(w, "decode generated API operation", stdhttp.StatusInternalServerError)
		return
	}
	writeMachineArtifact(w, "text/markdown; charset=utf-8", []byte(renderMachineAPIOperation(operation)))
}

func machineItemPath(value string) (id, format string, ok bool) {
	id, extension, found := strings.Cut(value, ".")
	if !found || id == "" || strings.Contains(id, "/") {
		return "", "", false
	}
	if extension != "json" && extension != "md" {
		return "", "", false
	}
	return id, extension, true
}

func writeMachineArtifact(w stdhttp.ResponseWriter, contentType string, contents []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Robots-Tag", "noindex")
	_, _ = w.Write(contents)
}

func prettyJSON(raw json.RawMessage) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		panic(fmt.Sprintf("indent generated JSON: %v", err))
	}
	out.WriteByte('\n')
	return out.Bytes()
}

func renderMachineAPIOperation(operation machineAPIOperation) string {
	var out strings.Builder
	title := operation.Summary
	if title == "" {
		title = operation.OperationID
	}
	out.WriteString("# " + title + "\n\n")
	out.WriteString("`" + operation.Method + " " + operation.Path + "`\n\n")
	out.WriteString("Operation ID: `" + operation.OperationID + "`  \n")
	out.WriteString("Effect: `" + operation.Effect + "`  \n")
	out.WriteString("Confirmation: `" + operation.Confirmation + "`\n\n")
	if operation.Description != "" {
		out.WriteString(strings.TrimSpace(operation.Description) + "\n\n")
	}
	if len(operation.Authorization) > 0 {
		out.WriteString("## Authorization\n\n")
		keys := make([]string, 0, len(operation.Authorization))
		for key := range operation.Authorization {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out.WriteString("- " + key + ": `" + fmt.Sprint(operation.Authorization[key]) + "`\n")
		}
		out.WriteByte('\n')
	}
	if len(operation.Parameters) > 0 {
		out.WriteString("## Parameters\n\n| Name | In | Required | Schema | Description |\n| --- | --- | --- | --- | --- |\n")
		for _, parameter := range operation.Parameters {
			schema, _ := json.Marshal(parameter.Schema)
			out.WriteString("| `" + parameter.Name + "` | " + parameter.In + " | " + strconv.FormatBool(parameter.Required) + " | `" + strings.ReplaceAll(string(schema), "|", "\\|") + "` | " + strings.ReplaceAll(parameter.Description, "|", "\\|") + " |\n")
		}
		out.WriteByte('\n')
	}
	if operation.RequestBody != nil {
		out.WriteString("## Request body\n\n")
		for _, item := range operation.RequestBody.Content {
			out.WriteString("- `" + item.ContentType + "`: `" + schemaLabel(item.Schema) + "`\n")
		}
		out.WriteByte('\n')
	}
	if len(operation.Responses) > 0 {
		out.WriteString("## Responses\n\n| Status | Content | Description |\n| --- | --- | --- |\n")
		for _, response := range operation.Responses {
			contentTypes := make([]string, 0, len(response.Content))
			for _, item := range response.Content {
				contentTypes = append(contentTypes, "`"+item.ContentType+"` ("+schemaLabel(item.Schema)+")")
			}
			out.WriteString("| `" + response.Status + "` | " + strings.Join(contentTypes, ", ") + " | " + strings.ReplaceAll(response.Description, "|", "\\|") + " |\n")
		}
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

func schemaLabel(schema map[string]any) string {
	if reference, ok := schema["$ref"].(string); ok {
		return strings.TrimPrefix(reference, "#/components/schemas/")
	}
	if kind, ok := schema["type"].(string); ok {
		return kind
	}
	return "schema"
}

func docsLLMs(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	contents, err := content.Files.ReadFile("llms.txt")
	if err != nil {
		stdhttp.Error(w, "generated llms.txt is missing", stdhttp.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(contents)
}
