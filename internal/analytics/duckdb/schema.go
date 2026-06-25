package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

func DiscoverSchemas(ctx context.Context, db *Database, model *semanticmodel.Model) error {
	if db == nil || db.SQLDB() == nil {
		return fmt.Errorf("schema discovery requires a DuckDB database")
	}
	if model == nil {
		return fmt.Errorf("schema discovery requires a semantic model")
	}
	var databaseName string
	if err := db.SQLDB().QueryRowContext(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		return err
	}
	rows, err := db.SQLDB().QueryContext(ctx, `
SELECT schema_name, table_name, column_name, column_index, data_type, is_nullable, column_default, comment
FROM duckdb_columns()
WHERE database_name = ? AND schema_name IN ('source', 'model')
ORDER BY schema_name, table_name, column_index`, databaseName)
	if err != nil {
		return err
	}
	defer rows.Close()

	sourceColumns := map[string][]semanticmodel.ColumnSchema{}
	tableColumns := map[string][]semanticmodel.ColumnSchema{}
	for rows.Next() {
		var schemaName, tableName, columnName, dataType string
		var ordinal int
		var nullable sql.NullBool
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&schemaName, &tableName, &columnName, &ordinal, &dataType, &nullable, &defaultValue, &comment); err != nil {
			return err
		}
		var nullableValue *bool
		if nullable.Valid {
			value := nullable.Bool
			nullableValue = &value
		}
		column := semanticmodel.ColumnSchema{
			Name:         columnName,
			Ordinal:      ordinal,
			PhysicalType: dataType,
			Nullable:     nullableValue,
			Default:      defaultValue.String,
			Comment:      comment.String,
		}
		switch schemaName {
		case "source":
			sourceColumns[tableName] = append(sourceColumns[tableName], column)
		case "model":
			tableColumns[tableName] = append(tableColumns[tableName], column)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for name, source := range model.Sources {
		columns, err := discoverSourceSchema(ctx, db.SQLDB(), model, source)
		if err != nil {
			return fmt.Errorf("discovering source %s schema: %w", name, err)
		}
		if len(columns) == 0 {
			columns = sortedColumns(sourceColumns[name])
		}
		source.Schema = semanticmodel.TableSchema{Columns: columns}
		model.Sources[name] = source
	}
	for name, table := range model.Tables {
		columns := sortedColumns(tableColumns[name])
		for index := range columns {
			columns[index].PrimaryKey = columns[index].Name == table.PrimaryKey
		}
		table.Schema = semanticmodel.TableSchema{Columns: columns}
		model.Tables[name] = table
	}
	return model.ValidateDiscoveredSchemas()
}

func discoverSourceSchema(ctx context.Context, db *sql.DB, model *semanticmodel.Model, source semanticmodel.Source) ([]semanticmodel.ColumnSchema, error) {
	if source.Kind() != semanticmodel.KindObject {
		return nil, nil
	}
	connection := model.Connections[source.Connection]
	spec, ok := semanticmodel.LookupConnection(connection.Kind)
	if !ok || spec.ObjectRelation != semanticmodel.ObjectRelationQuackQuery {
		return nil, nil
	}
	sqlText, err := quackMetadataColumnsSQL(connection.Path, source.Object, connection.Options)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []semanticmodel.ColumnSchema{}
	for rows.Next() {
		var columnName, dataType string
		var ordinal int
		var nullableText sql.NullString
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&columnName, &ordinal, &dataType, &nullableText, &defaultValue, &comment); err != nil {
			return nil, err
		}
		var nullableValue *bool
		if nullableText.Valid {
			value := strings.EqualFold(nullableText.String, "YES") || strings.EqualFold(nullableText.String, "true")
			nullableValue = &value
		}
		columns = append(columns, semanticmodel.ColumnSchema{
			Name:         columnName,
			Ordinal:      ordinal,
			PhysicalType: dataType,
			Nullable:     nullableValue,
			Default:      defaultValue.String,
			Comment:      comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sortedColumns(columns), nil
}

func quackMetadataColumnsSQL(uri, object string, options map[string]any) (string, error) {
	parts := strings.Split(object, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("quack object %q must include at least schema and table", object)
	}
	tableName := parts[len(parts)-1]
	schemaName := parts[len(parts)-2]
	catalogPredicate := ""
	if len(parts) >= 3 {
		catalogPredicate = " AND table_catalog = '" + sqlString(parts[len(parts)-3]) + "'"
	}
	remoteSQL := "SELECT column_name, ordinal_position, data_type, is_nullable, column_default, NULL AS comment FROM information_schema.columns WHERE table_schema = '" +
		sqlString(schemaName) + "' AND table_name = '" + sqlString(tableName) + "'" + catalogPredicate + " ORDER BY ordinal_position"
	call, err := quackQueryCall(uri, remoteSQL, options)
	if err != nil {
		return "", err
	}
	return "SELECT column_name, ordinal_position, data_type, is_nullable, column_default, comment FROM " + call, nil
}

func (db *Database) DiscoverSchemas(ctx context.Context, model *semanticmodel.Model) error {
	return DiscoverSchemas(ctx, db, model)
}

func sortedColumns(columns []semanticmodel.ColumnSchema) []semanticmodel.ColumnSchema {
	out := append([]semanticmodel.ColumnSchema{}, columns...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ordinal != out[j].Ordinal {
			return out[i].Ordinal < out[j].Ordinal
		}
		return out[i].Name < out[j].Name
	})
	return out
}
