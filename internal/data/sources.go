package data

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Yacobolo/libredash/internal/semantic"
)

func (m *DuckDBMetrics) prepareSourceRuntime(ctx context.Context, runtime *modelRuntime) error {
	for _, extension := range requiredExtensions(runtime.model) {
		if err := validateIdentifier(extension); err != nil {
			return fmt.Errorf("invalid extension %q: %w", extension, err)
		}
		if _, err := runtime.db.ExecContext(ctx, "INSTALL "+extension); err != nil {
			return fmt.Errorf("installing DuckDB extension %s: %w", extension, err)
		}
		if _, err := runtime.db.ExecContext(ctx, "LOAD "+extension); err != nil {
			return fmt.Errorf("loading DuckDB extension %s: %w", extension, err)
		}
	}
	for _, name := range sortedKeys(runtime.model.Connections) {
		stmt, ok, err := compileConnectionSecret(name, runtime.model.Connections[name])
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if _, err := runtime.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating DuckDB secret for connection %s: %w", name, err)
		}
	}
	if runtime.attachedConnections == nil {
		runtime.attachedConnections = map[string]struct{}{}
	}
	for _, sourceName := range sortedKeys(runtime.model.Sources) {
		source := runtime.model.Sources[sourceName]
		if source.Type != "database" {
			continue
		}
		if _, ok := runtime.attachedConnections[source.Connection]; ok {
			continue
		}
		connection := runtime.model.Connections[source.Connection]
		stmt, err := compileDatabaseAttach(source.Connection, source.Engine, connection)
		if err != nil {
			return err
		}
		if _, err := runtime.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("attaching database connection %s: %w", source.Connection, err)
		}
		runtime.attachedConnections[source.Connection] = struct{}{}
	}
	return nil
}

func (m *DuckDBMetrics) sourceRelation(source semantic.Source) (string, error) {
	location := source.Location
	if source.Type == "file" && source.Location != "" && isLocalSourceLocation(source.Location) {
		location = filepath.Join(m.dataDir, source.Location)
	}
	return compileSourceRelation(location, source)
}

func compileSourceRelation(location string, source semantic.Source) (string, error) {
	switch source.Type {
	case "file":
		switch source.Format {
		case "csv":
			return scanRelation("read_csv", location, source.Options)
		case "json":
			return scanRelation("read_json", location, source.Options)
		case "parquet":
			return scanRelation("read_parquet", location, source.Options)
		case "excel":
			return scanRelation("read_xlsx", location, source.Options)
		default:
			return "", fmt.Errorf("unsupported file format %q", source.Format)
		}
	case "lakehouse":
		switch source.Format {
		case "delta":
			return scanRelation("delta_scan", location, source.Options)
		case "iceberg":
			return scanRelation("iceberg_scan", location, source.Options)
		default:
			return "", fmt.Errorf("unsupported lakehouse format %q", source.Format)
		}
	case "database":
		object, err := qualifiedSQLName(source.Object)
		if err != nil {
			return "", err
		}
		alias, err := databaseAlias(source.Connection)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("SELECT * FROM %s.%s", alias, object), nil
	case "query":
		return source.Query, nil
	default:
		return "", fmt.Errorf("unsupported source type %q", source.Type)
	}
}

func scanRelation(function, location string, options map[string]any) (string, error) {
	optionSQL, err := sqlOptions(options)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SELECT * FROM %s('%s'%s)", function, sqlString(location), optionSQL), nil
}

func sqlOptions(options map[string]any) (string, error) {
	if len(options) == 0 {
		return "", nil
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		if err := validateIdentifier(key); err != nil {
			return "", fmt.Errorf("invalid source option %q: %w", key, err)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(", ")
		builder.WriteString(key)
		builder.WriteString(" = ")
		builder.WriteString(sqlLiteral(options[key]))
	}
	return builder.String(), nil
}

func sqlLiteral(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + sqlString(v) + "'"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, sqlLiteral(item))
		}
		return "[" + strings.Join(values, ", ") + "]"
	case []string:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, sqlLiteral(item))
		}
		return "[" + strings.Join(values, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fields := make([]string, 0, len(keys))
		for _, key := range keys {
			fields = append(fields, "'"+sqlString(key)+"': "+sqlLiteral(v[key]))
		}
		return "{" + strings.Join(fields, ", ") + "}"
	default:
		return "'" + sqlString(fmt.Sprint(v)) + "'"
	}
}

func requiredExtensions(model *semantic.Model) []string {
	extensions := map[string]struct{}{}
	addConnection := func(connectionType string) {
		switch connectionType {
		case "s3", "r2", "gcs", "http":
			extensions["httpfs"] = struct{}{}
		case "azure":
			extensions["azure"] = struct{}{}
		case "postgres", "mysql", "sqlite":
			extensions[connectionType] = struct{}{}
		}
	}
	addLocation := func(location string) {
		switch {
		case strings.HasPrefix(location, "s3://"), strings.HasPrefix(location, "r2://"), strings.HasPrefix(location, "gcs://"), strings.HasPrefix(location, "gs://"), strings.HasPrefix(location, "http://"), strings.HasPrefix(location, "https://"):
			extensions["httpfs"] = struct{}{}
		case strings.HasPrefix(location, "az://"), strings.HasPrefix(location, "azure://"), strings.HasPrefix(location, "abfss://"):
			extensions["azure"] = struct{}{}
		}
	}
	for _, name := range sortedKeys(model.Connections) {
		addConnection(model.Connections[name].Type)
	}
	for _, name := range sortedKeys(model.Sources) {
		source := model.Sources[name]
		addLocation(source.Location)
		switch source.Type {
		case "file":
			if source.Format == "excel" {
				extensions["excel"] = struct{}{}
			}
		case "lakehouse":
			extensions[source.Format] = struct{}{}
		case "database":
			extensions[source.Engine] = struct{}{}
		}
	}
	return sortedKeys(extensions)
}

func compileConnectionSecret(name string, connection semantic.Connection) (string, bool, error) {
	if connection.Secret != "" {
		return "", false, nil
	}
	if connection.Auth.Method == "" && len(connection.Auth.Params) == 0 && connection.Auth.Profile == "" && connection.Auth.Chain == "" && connection.Auth.Account == "" && connection.Scope == "" {
		return "", false, nil
	}
	secret, err := connectionSecretName(name, connection)
	if err != nil {
		return "", false, err
	}
	parts := []string{"TYPE " + connection.Type}
	if connection.Auth.Method != "" {
		if err := validateIdentifier(connection.Auth.Method); err != nil {
			return "", false, fmt.Errorf("invalid auth method %q: %w", connection.Auth.Method, err)
		}
		parts = append(parts, "PROVIDER "+connection.Auth.Method)
	}
	if connection.Auth.Profile != "" {
		parts = append(parts, "PROFILE '"+sqlString(connection.Auth.Profile)+"'")
	}
	if connection.Auth.Chain != "" {
		parts = append(parts, "CHAIN '"+sqlString(connection.Auth.Chain)+"'")
	}
	if connection.Auth.Account != "" {
		parts = append(parts, "ACCOUNT_NAME '"+sqlString(connection.Auth.Account)+"'")
	}
	for _, key := range sortedKeys(connection.Auth.Params) {
		if err := validateIdentifier(key); err != nil {
			return "", false, fmt.Errorf("invalid auth param %q: %w", key, err)
		}
		parts = append(parts, strings.ToUpper(key)+" "+sqlLiteral(connection.Auth.Params[key]))
	}
	if connection.Scope != "" {
		parts = append(parts, "SCOPE '"+sqlString(connection.Scope)+"'")
	}
	return fmt.Sprintf("CREATE OR REPLACE SECRET %s (%s)", secret, strings.Join(parts, ", ")), true, nil
}

func compileDatabaseAttach(connectionName, engine string, connection semantic.Connection) (string, error) {
	alias, err := databaseAlias(connectionName)
	if err != nil {
		return "", err
	}
	connectionString, err := connectionStringOption(connection)
	if err != nil {
		return "", err
	}
	parts := []string{"TYPE " + engine, "READ_ONLY"}
	if secret, ok, err := databaseAttachSecret(connectionName, connection); err != nil {
		return "", err
	} else if ok {
		parts = append(parts, "SECRET "+secret)
	}
	return fmt.Sprintf("ATTACH '%s' AS %s (%s)", sqlString(connectionString), alias, strings.Join(parts, ", ")), nil
}

func databaseAttachSecret(connectionName string, connection semantic.Connection) (string, bool, error) {
	if connection.Secret != "" {
		if err := validateIdentifier(connection.Secret); err != nil {
			return "", false, fmt.Errorf("invalid connection secret %q: %w", connection.Secret, err)
		}
		return connection.Secret, true, nil
	}
	if connection.Auth.Method == "" && len(connection.Auth.Params) == 0 && connection.Auth.Profile == "" && connection.Auth.Chain == "" && connection.Auth.Account == "" {
		return "", false, nil
	}
	secret, err := connectionSecretName(connectionName, connection)
	if err != nil {
		return "", false, err
	}
	return secret, true, nil
}

func connectionSecretName(name string, connection semantic.Connection) (string, error) {
	if connection.Secret != "" {
		if err := validateIdentifier(connection.Secret); err != nil {
			return "", fmt.Errorf("invalid connection secret %q: %w", connection.Secret, err)
		}
		return connection.Secret, nil
	}
	if err := validateIdentifier(name); err != nil {
		return "", fmt.Errorf("invalid connection %q: %w", name, err)
	}
	return "libredash_" + name, nil
}

func connectionStringOption(connection semantic.Connection) (string, error) {
	for key := range connection.Options {
		if !supportsDatabaseConnectionOption(key) {
			return "", fmt.Errorf("unsupported database connection option %q", key)
		}
	}
	for _, key := range []string{"connection_string", "uri", "path", "database"} {
		if value, ok := connection.Options[key]; ok {
			return fmt.Sprint(value), nil
		}
	}
	return "", nil
}

func supportsDatabaseConnectionOption(option string) bool {
	switch option {
	case "connection_string", "uri", "path", "database":
		return true
	default:
		return false
	}
}

func databaseAlias(connection string) (string, error) {
	if err := validateIdentifier(connection); err != nil {
		return "", fmt.Errorf("invalid database connection %q: %w", connection, err)
	}
	return "conn_" + connection, nil
}

func qualifiedSQLName(name string) (string, error) {
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		if err := validateIdentifier(part); err != nil {
			return "", fmt.Errorf("invalid database object %q: %w", name, err)
		}
		quoted = append(quoted, part)
	}
	return strings.Join(quoted, "."), nil
}

func isLocalSourceLocation(location string) bool {
	for _, prefix := range []string{"s3://", "r2://", "gcs://", "gs://", "az://", "azure://", "abfss://", "http://", "https://", "file://"} {
		if strings.HasPrefix(location, prefix) {
			return false
		}
	}
	return !strings.Contains(location, "://")
}
