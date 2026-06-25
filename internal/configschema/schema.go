package configschema

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	cuejsonschema "cuelang.org/go/encoding/jsonschema"
	cueyaml "cuelang.org/go/encoding/yaml"
)

//go:embed contracts/contracts.cue
var contractsCUE string

type Kind string

const (
	KindCatalog       Kind = "catalog"
	KindSemanticModel Kind = "semantic-model"
	KindDashboard     Kind = "dashboard"
)

type Severity string

const (
	SeverityError Severity = "error"
)

type Diagnostic struct {
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
}

type Error struct {
	Diagnostics []Diagnostic
}

func (e *Error) Error() string {
	if len(e.Diagnostics) == 0 {
		return "configuration schema validation failed"
	}
	return e.Diagnostics[0].String()
}

func (d Diagnostic) String() string {
	location := d.File
	if d.Line > 0 {
		location += fmt.Sprintf(":%d", d.Line)
		if d.Column > 0 {
			location += fmt.Sprintf(":%d", d.Column)
		}
	}
	if location == "" {
		return fmt.Sprintf("%s %s: %s", d.Severity, d.Code, d.Message)
	}
	return fmt.Sprintf("%s: %s %s: %s", location, d.Severity, d.Code, d.Message)
}

func ValidateFile(kind Kind, path string) error {
	content, err := readFile(path)
	if err != nil {
		return err
	}
	return ValidateBytes(kind, path, content)
}

func ValidateBytes(kind Kind, filename string, content []byte) error {
	definition, err := definitionName(kind)
	if err != nil {
		return err
	}
	ctx := cuecontext.New()
	contracts := ctx.CompileString(contractsCUE, cue.Filename("contracts.cue"))
	if err := contracts.Err(); err != nil {
		return err
	}
	file, err := cueyaml.Extract(filename, content)
	if err != nil {
		return &Error{Diagnostics: []Diagnostic{{
			File:     filename,
			Severity: SeverityError,
			Code:     "schema.yaml",
			Message:  err.Error(),
		}}}
	}
	data := ctx.BuildFile(file)
	value := contracts.LookupPath(cue.MakePath(cue.Def(definition))).Unify(data)
	if err := value.Validate(cue.Final()); err != nil {
		return &Error{Diagnostics: diagnosticsForCUEError(filename, definition, err)}
	}
	return nil
}

func JSONSchema(kind Kind) ([]byte, error) {
	definition, err := definitionName(kind)
	if err != nil {
		return nil, err
	}
	ctx := cuecontext.New()
	contracts := ctx.CompileString(contractsCUE, cue.Filename("contracts.cue"))
	if err := contracts.Err(); err != nil {
		return nil, err
	}
	value := contracts.LookupPath(cue.MakePath(cue.Def(definition)))
	expr, err := cuejsonschema.Generate(value, &cuejsonschema.GenerateConfig{Version: cuejsonschema.VersionDraft2020_12})
	if err != nil {
		return nil, err
	}
	raw, err := ctx.BuildExpr(expr).MarshalJSON()
	if err != nil {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(pretty, '\n'), nil
}

func JSONSchemaFiles() (map[string][]byte, error) {
	kinds := []Kind{KindCatalog, KindSemanticModel, KindDashboard}
	files := map[string][]byte{}
	for _, kind := range kinds {
		content, err := JSONSchema(kind)
		if err != nil {
			return nil, err
		}
		files[JSONSchemaFilename(kind)] = content
	}
	return files, nil
}

func JSONSchemaFilename(kind Kind) string {
	switch kind {
	case KindCatalog:
		return "catalog.schema.json"
	case KindSemanticModel:
		return "semantic-model.schema.json"
	case KindDashboard:
		return "dashboard.schema.json"
	default:
		return string(kind) + ".schema.json"
	}
}

func Diagnostics(err error) []Diagnostic {
	if err == nil {
		return nil
	}
	var schemaErr *Error
	if errors.As(err, &schemaErr) {
		return append([]Diagnostic(nil), schemaErr.Diagnostics...)
	}
	return []Diagnostic{DiagnosticForError(err)}
}

func DiagnosticForError(err error) Diagnostic {
	return Diagnostic{
		Severity: SeverityError,
		Code:     compilerCode(err),
		Message:  err.Error(),
	}
}

func definitionName(kind Kind) (string, error) {
	switch kind {
	case KindCatalog:
		return "Catalog", nil
	case KindSemanticModel:
		return "SemanticModel", nil
	case KindDashboard:
		return "Dashboard", nil
	default:
		return "", fmt.Errorf("unknown schema kind %q", kind)
	}
}

func diagnosticsForCUEError(filename, definition string, err error) []Diagnostic {
	items := cueerrors.Errors(err)
	if len(items) == 0 {
		return []Diagnostic{{
			File:     filename,
			Severity: SeverityError,
			Code:     schemaCode(err.Error()),
			Message:  cleanMessage(definition, err.Error()),
		}}
	}
	diagnostics := make([]Diagnostic, 0, len(items))
	for _, item := range items {
		message := cueerrors.String(item)
		if len(items) > 1 && strings.Contains(message, "empty disjunction") {
			continue
		}
		pos := positionFor(filename, item)
		diagnostics = append(diagnostics, Diagnostic{
			File:     pos.file,
			Line:     pos.line,
			Column:   pos.column,
			Severity: SeverityError,
			Code:     schemaCode(message),
			Message:  cleanMessage(definition, message),
		})
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].File != diagnostics[j].File {
			return diagnostics[i].File < diagnostics[j].File
		}
		if diagnostics[i].Line == 0 || diagnostics[j].Line == 0 {
			return diagnostics[j].Line == 0 && diagnostics[i].Line != 0
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Column < diagnostics[j].Column
	})
	if len(diagnostics) == 0 {
		return []Diagnostic{{
			File:     filename,
			Severity: SeverityError,
			Code:     schemaCode(err.Error()),
			Message:  cleanMessage(definition, err.Error()),
		}}
	}
	return diagnostics
}

type diagnosticPosition struct {
	file   string
	line   int
	column int
}

func positionFor(filename string, err cueerrors.Error) diagnosticPosition {
	positions := cueerrors.Positions(err)
	for _, pos := range positions {
		if filepath.Clean(pos.Filename()) == filepath.Clean(filename) {
			return diagnosticPosition{file: filename, line: pos.Line(), column: pos.Column()}
		}
	}
	for _, pos := range positions {
		if pos.Filename() != "" && pos.Filename() != "contracts.cue" {
			return diagnosticPosition{file: pos.Filename(), line: pos.Line(), column: pos.Column()}
		}
	}
	return diagnosticPosition{file: filename}
}

func schemaCode(message string) string {
	switch {
	case strings.Contains(message, "field not allowed"):
		return "schema.unknown_field"
	case strings.Contains(message, "mismatched types"), strings.Contains(message, "cannot use"):
		return "schema.type"
	case strings.Contains(message, "=~"):
		return "schema.contract"
	case strings.Contains(message, "empty disjunction"), strings.Contains(message, "conflicting values"),
		strings.Contains(message, "invalid value"), strings.Contains(message, "out of bound"), strings.Contains(message, "not allowed"):
		return "schema.enum"
	default:
		return "schema.contract"
	}
}

func compilerCode(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "unknown dimension"),
		strings.Contains(message, "unknown measure"),
		strings.Contains(message, "unknown semantic model"),
		strings.Contains(message, "unknown table"),
		strings.Contains(message, "references unknown"):
		return "compiler.reference"
	default:
		return "compiler.contract"
	}
}

func cleanMessage(definition, message string) string {
	prefixes := []string{"#" + definition + ".", "#" + definition + ":"}
	for _, prefix := range prefixes {
		message = strings.ReplaceAll(message, prefix, "")
	}
	message = strings.TrimSpace(message)
	return message
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
