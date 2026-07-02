package materialize

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	semanticquery "github.com/Yacobolo/libredash/internal/analytics/query"
)

type RuntimeConfig struct {
	ModelID string
	Model   *semanticmodel.Model
	DataDir string
	DBDir   string

	Database Database
	Sources  SourceRegistrar
	Resolver SourcePathResolver
}

type ModelTableQuery struct {
	Table   string
	Columns []string
	Sort    []semanticquery.Sort
	Limit   int
	Offset  int
}

type Runtime struct {
	model       *semanticmodel.Model
	db          Database
	sources     SourceRegistrar
	queries     *semanticquery.Service
	lastRefresh time.Time
}

type Database interface {
	Executor
	semanticquery.Executor
	Close() error
	Path() string
}

type schemaDiscoverer interface {
	DiscoverSchemas(context.Context, *semanticmodel.Model) error
}

func OpenRuntime(ctx context.Context, config RuntimeConfig) (*Runtime, error) {
	if config.Model == nil {
		return nil, fmt.Errorf("semantic model is required")
	}
	if config.Database == nil {
		return nil, fmt.Errorf("materialization database is required")
	}
	if config.Sources == nil {
		return nil, fmt.Errorf("source registrar is required")
	}
	resolver := config.Resolver
	if resolver == nil {
		resolver = defaultSourcePathResolver{}
	}
	if err := ValidateFilesWithResolver(config.Model, config.DataDir, resolver); err != nil {
		return nil, err
	}
	runtime := &Runtime{
		model:   config.Model,
		db:      config.Database,
		sources: config.Sources,
		queries: semanticquery.NewService(semanticquery.NewPlanner(config.Model), config.Database),
	}
	if err := runtime.Refresh(ctx); err != nil {
		config.Database.Close()
		return nil, err
	}
	return runtime, nil
}

func DatabasePath(dbDir, modelID string) string {
	if path := os.Getenv("LIBREDASH_DUCKDB_PATH"); path != "" {
		return path
	}
	return filepath.Join(dbDir, "libredash-"+modelID+".duckdb")
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Runtime) Refresh(ctx context.Context) error {
	lastRefresh, err := Refresh(ctx, r.db, r.sources, r.model)
	if err != nil {
		return err
	}
	if discoverer, ok := r.db.(schemaDiscoverer); ok {
		if err := discoverer.DiscoverSchemas(ctx, r.model); err != nil {
			return err
		}
	}
	r.lastRefresh = lastRefresh
	return nil
}

func (r *Runtime) RefreshModelTables(ctx context.Context, tableNames []string) error {
	lastRefresh, err := RefreshModelTables(ctx, r.db, r.sources, r.model, tableNames)
	if err != nil {
		return err
	}
	if discoverer, ok := r.db.(schemaDiscoverer); ok {
		if err := discoverer.DiscoverSchemas(ctx, r.model); err != nil {
			return err
		}
	}
	r.lastRefresh = lastRefresh
	return nil
}

func (r *Runtime) Queries() *semanticquery.Service {
	if r == nil {
		return nil
	}
	return r.queries
}

func (r *Runtime) CountModelTable(ctx context.Context, tableName string) (int, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("materialization runtime is not initialized")
	}
	if _, err := r.modelTable(tableName); err != nil {
		return 0, err
	}
	quotedTable, err := quotedModelTableName(tableName)
	if err != nil {
		return 0, err
	}
	return r.db.Count(ctx, semanticquery.Plan{
		SQL:     "SELECT count(*) FROM model." + quotedTable,
		Columns: []string{"count"},
	})
}

func (r *Runtime) ModelTableRows(ctx context.Context, request ModelTableQuery) (semanticquery.Rows, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("materialization runtime is not initialized")
	}
	table, err := r.modelTable(request.Table)
	if err != nil {
		return nil, err
	}
	columns, err := modelTableQueryColumns(table, request.Columns)
	if err != nil {
		return nil, err
	}
	quotedTable, err := quotedModelTableName(request.Table)
	if err != nil {
		return nil, err
	}
	var sql strings.Builder
	sql.WriteString("SELECT ")
	for index, column := range columns {
		if index > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(quoteMaterializedIdentifier(column))
	}
	sql.WriteString("\nFROM model.")
	sql.WriteString(quotedTable)
	if len(request.Sort) > 0 {
		orderParts := []string{}
		columnSet := modelTableColumnSet(table)
		for _, sortSpec := range request.Sort {
			if !columnSet[sortSpec.Field] {
				return nil, fmt.Errorf("model table %q does not expose sort column %q", request.Table, sortSpec.Field)
			}
			direction := strings.ToUpper(strings.TrimSpace(sortSpec.Direction))
			if direction != "ASC" && direction != "DESC" {
				return nil, fmt.Errorf("unsupported sort direction %q", sortSpec.Direction)
			}
			orderParts = append(orderParts, quoteMaterializedIdentifier(sortSpec.Field)+" "+direction)
		}
		if len(orderParts) > 0 {
			sql.WriteString("\nORDER BY ")
			sql.WriteString(strings.Join(orderParts, ", "))
		}
	}
	if request.Limit > 0 {
		sql.WriteString(fmt.Sprintf("\nLIMIT %d", request.Limit))
	}
	if request.Offset > 0 {
		if request.Limit <= 0 {
			return nil, fmt.Errorf("offset requires limit")
		}
		sql.WriteString(fmt.Sprintf("\nOFFSET %d", request.Offset))
	}
	return r.db.Query(ctx, semanticquery.Plan{SQL: sql.String(), Columns: columns})
}

func (r *Runtime) modelTable(tableName string) (semanticmodel.Table, error) {
	if r == nil || r.model == nil {
		return semanticmodel.Table{}, fmt.Errorf("semantic model is required")
	}
	tableName = strings.TrimSpace(tableName)
	table, ok := r.model.Tables[tableName]
	if !ok {
		return semanticmodel.Table{}, fmt.Errorf("model table %q is not available in semantic model %q", tableName, r.model.Name)
	}
	return table, nil
}

func modelTableQueryColumns(table semanticmodel.Table, requested []string) ([]string, error) {
	columnSet := modelTableColumnSet(table)
	if len(requested) > 0 {
		columns := []string{}
		for _, column := range requested {
			column = strings.TrimSpace(column)
			if column == "" {
				continue
			}
			if !columnSet[column] {
				return nil, fmt.Errorf("model table does not expose column %q", column)
			}
			columns = append(columns, column)
		}
		if len(columns) > 0 {
			return columns, nil
		}
	}
	if len(table.Schema.Columns) > 0 {
		schemaColumns := append([]semanticmodel.ColumnSchema{}, table.Schema.Columns...)
		sort.SliceStable(schemaColumns, func(i, j int) bool {
			if schemaColumns[i].Ordinal != schemaColumns[j].Ordinal {
				return schemaColumns[i].Ordinal < schemaColumns[j].Ordinal
			}
			return schemaColumns[i].Name < schemaColumns[j].Name
		})
		columns := make([]string, 0, len(schemaColumns))
		for _, column := range schemaColumns {
			if column.Name != "" {
				columns = append(columns, column.Name)
			}
		}
		if len(columns) > 0 {
			return columns, nil
		}
	}
	columns := make([]string, 0, len(table.Columns))
	for name := range table.Columns {
		columns = append(columns, name)
	}
	sort.Strings(columns)
	if len(columns) == 0 {
		return nil, fmt.Errorf("model table has no columns")
	}
	return columns, nil
}

func modelTableColumnSet(table semanticmodel.Table) map[string]bool {
	columns := map[string]bool{}
	for name := range table.Columns {
		columns[name] = true
	}
	for _, column := range table.Schema.Columns {
		if column.Name != "" {
			columns[column.Name] = true
		}
	}
	return columns
}

func quotedModelTableName(tableName string) (string, error) {
	if err := validateIdentifier(tableName); err != nil {
		return "", err
	}
	return quoteMaterializedIdentifier(tableName), nil
}

func quoteMaterializedIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (r *Runtime) LastRefresh() time.Time {
	if r == nil {
		return time.Time{}
	}
	return r.lastRefresh
}

func (r *Runtime) DBPath() string {
	if r == nil {
		return ""
	}
	return r.db.Path()
}
