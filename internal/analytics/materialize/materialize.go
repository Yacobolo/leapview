package materialize

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

type Executor interface {
	Exec(ctx context.Context, statement string) error
}

type SourceRegistrar interface {
	RegisterSourceReads(ctx context.Context, model *semanticmodel.Model, reads []SourceReadPlan) error
}

type SourceReadPlan struct {
	Source string
	Fields []string
}

type SourcePathResolver interface {
	ResolveSourcePath(model *semanticmodel.Model, source semanticmodel.Source, dataDir string) (string, error)
}

type MissingDataError struct {
	DataDir string
	Missing []string
}

func (e *MissingDataError) Error() string {
	return fmt.Sprintf("local source files are missing in %s: %s. Run the workspace bootstrap script or set LIBREDASH_DATA_DIR.", e.DataDir, strings.Join(e.Missing, ", "))
}

func (e *MissingDataError) SetupRequired() bool {
	return true
}

func Refresh(ctx context.Context, executor Executor, sources SourceRegistrar, model *semanticmodel.Model) (time.Time, error) {
	if executor == nil {
		return time.Time{}, fmt.Errorf("materialization executor is required")
	}
	if sources == nil {
		return time.Time{}, fmt.Errorf("source registrar is required")
	}
	if err := ModelTables(ctx, executor, sources, model); err != nil {
		return time.Time{}, err
	}
	return time.Now(), nil
}

func ValidateFiles(model *semanticmodel.Model, dataDir string) error {
	return ValidateFilesWithResolver(model, dataDir, defaultSourcePathResolver{})
}

func ValidateFilesWithResolver(model *semanticmodel.Model, dataDir string, resolver SourcePathResolver) error {
	if resolver == nil {
		return fmt.Errorf("source path resolver is required")
	}
	var missing []string
	for name, source := range model.Sources {
		if source.Path == "" {
			continue
		}
		connection := model.Connections[source.Connection]
		if connection.Kind != "local" {
			continue
		}
		file, err := resolver.ResolveSourcePath(model, source, dataDir)
		if err != nil {
			return fmt.Errorf("resolving local source %s: %w", name, err)
		}
		if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, file)
		} else if err != nil {
			return err
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return &MissingDataError{DataDir: dataDir, Missing: missing}
	}
	return nil
}

func ResolveSourcePath(model *semanticmodel.Model, source semanticmodel.Source, dataDir string) (string, error) {
	return defaultSourcePathResolver{}.ResolveSourcePath(model, source, dataDir)
}

type defaultSourcePathResolver struct{}

func (defaultSourcePathResolver) ResolveSourcePath(model *semanticmodel.Model, source semanticmodel.Source, dataDir string) (string, error) {
	connection := model.Connections[source.Connection]
	switch connection.Kind {
	case "local":
		if filepath.IsAbs(source.Path) {
			return source.Path, nil
		}
		root := connection.Root
		if root == "" {
			root = dataDir
		} else if !filepath.IsAbs(root) {
			root = filepath.Join(dataDir, root)
		}
		return filepath.Join(root, source.Path), nil
	default:
		if connection.Scope == "" {
			return source.Path, nil
		}
		if semanticmodel.IsLocalPath(source.Path) {
			return semanticmodel.JoinScope(connection.Scope, source.Path), nil
		}
		if !semanticmodel.WithinScope(connection.Scope, source.Path) {
			return "", fmt.Errorf("path %q is outside connection %q scope %q", source.Path, source.Connection, connection.Scope)
		}
		return source.Path, nil
	}
}

func ModelTables(ctx context.Context, executor Executor, sources SourceRegistrar, model *semanticmodel.Model) error {
	if executor == nil {
		return fmt.Errorf("materialization executor is required")
	}
	if sources == nil {
		return fmt.Errorf("source registrar is required")
	}
	order, err := materializationOrder(model)
	if err != nil {
		return err
	}
	for _, name := range order {
		if err := validateIdentifier(name); err != nil {
			return err
		}
		table := model.Tables[name]
		if err := sources.RegisterSourceReads(ctx, model, sourceReadPlans(model, name, table)); err != nil {
			return err
		}
		sourceSQL := table.Transform.SQL
		if table.Source != "" {
			if err := validateIdentifier(table.Source); err != nil {
				return err
			}
			if sourceSQL == "" {
				sourceSQL = "SELECT * FROM source." + table.Source
			}
		}
		stmt := fmt.Sprintf("CREATE OR REPLACE TABLE model.%s AS %s", name, sourceSQL)
		if err := executor.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("materializing model.%s: %w", name, err)
		}
	}
	return nil
}

func materializationOrder(model *semanticmodel.Model) ([]string, error) {
	if model == nil {
		return nil, fmt.Errorf("semantic model is required")
	}
	temporary := map[string]bool{}
	permanent := map[string]bool{}
	order := []string{}
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
		temporary[name] = false
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

func sourceReadPlans(model *semanticmodel.Model, tableName string, table semanticmodel.Table) []SourceReadPlan {
	plans := []SourceReadPlan{}
	if table.Source != "" && table.Transform.SQL == "" {
		plans = append(plans, SourceReadPlan{Source: table.Source, Fields: modelTableReadFields(model, tableName, table)})
		return plans
	}
	for _, source := range table.SourceDependencies {
		plans = append(plans, SourceReadPlan{Source: source, Fields: sourceProjectionHint(model, source)})
	}
	return plans
}

func sourceProjectionHint(model *semanticmodel.Model, sourceName string) []string {
	if model == nil {
		return nil
	}
	source, ok := model.Sources[sourceName]
	if !ok || len(source.Fields) == 0 {
		return nil
	}
	fields := make([]string, 0, len(source.Fields))
	for field := range source.Fields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func modelTableReadFields(model *semanticmodel.Model, tableName string, table semanticmodel.Table) []string {
	fields := map[string]struct{}{}
	add := func(field string) {
		if field == "" {
			return
		}
		fields[field] = struct{}{}
	}
	add(table.PrimaryKey)
	for field := range table.Dimensions {
		add(field)
	}
	for _, measure := range table.Measures {
		for _, ref := range semanticmodel.ExpressionFieldRefs(measure.SQLExpression()) {
			refTable, refField, ok := strings.Cut(ref, ".")
			if ok && refTable == tableName {
				add(refField)
			}
		}
	}
	if model != nil {
		for _, measure := range model.Measures {
			if measure.Table != tableName {
				continue
			}
			for _, ref := range semanticmodel.ExpressionFieldRefs(measure.SQLExpression()) {
				refTable, refField, ok := strings.Cut(ref, ".")
				if ok && refTable == tableName {
					add(refField)
				}
			}
		}
	}
	return sortedStringSet(fields)
}

func sortedStringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func validateIdentifier(value string) error {
	for i, r := range value {
		if i == 0 {
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '_' {
				return fmt.Errorf("invalid identifier %q", value)
			}
			continue
		}
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("invalid identifier %q", value)
		}
	}
	if value == "" {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}
