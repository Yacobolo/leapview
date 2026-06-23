package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func PrepareSourceRuntime(ctx context.Context, db *sql.DB, model *semanticmodel.Model, dataDir string, attachedConnections map[string]struct{}) error {
	for _, extension := range RequiredExtensions(model) {
		if err := validateIdentifier(extension); err != nil {
			return fmt.Errorf("invalid extension %q: %w", extension, err)
		}
		if _, err := db.ExecContext(ctx, "INSTALL "+extension); err != nil {
			return fmt.Errorf("installing DuckDB extension %s: %w", extension, err)
		}
		if _, err := db.ExecContext(ctx, "LOAD "+extension); err != nil {
			return fmt.Errorf("loading DuckDB extension %s: %w", extension, err)
		}
	}
	for _, name := range sortedKeys(model.Connections) {
		stmt, ok, err := compileConnectionSecret(name, model.Connections[name])
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating DuckDB secret for connection %s: %w", name, err)
		}
	}
	sourceSecrets, err := compileSourceSecretStatements(model)
	if err != nil {
		return err
	}
	for _, stmt := range sourceSecrets {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating DuckDB source secret: %w", err)
		}
	}
	if attachedConnections == nil {
		attachedConnections = map[string]struct{}{}
	}
	for _, sourceName := range sortedKeys(model.Sources) {
		source := model.Sources[sourceName]
		if source.Kind() != semanticmodel.KindObject {
			continue
		}
		if _, ok := attachedConnections[source.Connection]; ok {
			continue
		}
		connection := model.Connections[source.Connection]
		connectionSpec, ok := semanticmodel.LookupConnection(connection.Kind)
		if !ok {
			return fmt.Errorf("unsupported connection kind %q", connection.Kind)
		}
		if !connectionRequiresObjectAttach(connectionSpec) {
			continue
		}
		stmt, err := compileObjectAttach(model, source.Connection, connection, dataDir)
		if err != nil {
			return err
		}
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("attaching source connection %s: %w", source.Connection, err)
		}
		attachedConnections[source.Connection] = struct{}{}
	}
	return nil
}

func RegisterSourceViews(ctx context.Context, db *sql.DB, model *semanticmodel.Model, dataDir string, attachedConnections map[string]struct{}) error {
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS raw"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS source"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS model"); err != nil {
		return err
	}

	if err := PrepareSourceRuntime(ctx, db, model, dataDir, attachedConnections); err != nil {
		return err
	}

	for _, name := range sortedKeys(model.Sources) {
		source := model.Sources[name]
		if err := validateIdentifier(name); err != nil {
			return err
		}
		relation, err := SourceRelation(model, source, dataDir)
		if err != nil {
			return fmt.Errorf("compiling source %s: %w", name, err)
		}
		stmt := fmt.Sprintf("CREATE OR REPLACE VIEW raw.%s AS %s", name, relation)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("registering source %s: %w", name, err)
		}
		stmt = fmt.Sprintf("CREATE OR REPLACE VIEW source.%s AS %s", name, relation)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("registering source %s: %w", name, err)
		}
	}
	return nil
}

type sourcePlan struct {
	kind             string
	format           string
	path             string
	connection       string
	connectionConfig semanticmodel.Connection
	connectionSpec   semanticmodel.ConnectionSpec
	object           string
	options          map[string]any
}

func SourceRelation(model *semanticmodel.Model, source semanticmodel.Source, dataDir string) (string, error) {
	plan, err := ResolveSourcePlan(model, source, dataDir)
	if err != nil {
		return "", err
	}
	return compileSourceRelation(plan)
}

func ResolveSourcePlan(model *semanticmodel.Model, source semanticmodel.Source, dataDir string) (sourcePlan, error) {
	plan := sourcePlan{
		kind:       source.Kind(),
		format:     source.Format,
		connection: source.Connection,
		object:     source.Object,
		options:    source.Options,
	}
	if connection, ok := model.Connections[source.Connection]; ok {
		plan.connectionConfig = connection
		if spec, ok := semanticmodel.LookupConnection(connection.Kind); ok {
			plan.connectionSpec = spec
		}
	}
	if source.Path == "" {
		return plan, nil
	}
	path, err := ResolveSourcePath(model, source, dataDir)
	if err != nil {
		return plan, err
	}
	plan.path = path
	return plan, nil
}

func ResolveSourcePath(model *semanticmodel.Model, source semanticmodel.Source, dataDir string) (string, error) {
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

func compileSourceRelation(plan sourcePlan) (string, error) {
	switch plan.kind {
	case semanticmodel.KindPath:
		format, ok := semanticmodel.LookupFormat(plan.format)
		if !ok {
			return "", fmt.Errorf("unsupported source format %q", plan.format)
		}
		if !format.AllowsOptions && len(plan.options) > 0 {
			return "", fmt.Errorf("%s source cannot set options", plan.format)
		}
		switch format.ScanKind {
		case semanticmodel.ScanTableFunction:
			return scanRelation(format.ScanFunction, plan.path, plan.options)
		case semanticmodel.ScanReplacement:
			return replacementScanRelation(plan.path), nil
		default:
			return "", fmt.Errorf("unsupported source scan kind %q", format.ScanKind)
		}
	case semanticmodel.KindObject:
		object, err := qualifiedSQLName(plan.object)
		if err != nil {
			return "", err
		}
		switch plan.connectionSpec.ObjectRelation {
		case semanticmodel.ObjectRelationAttach:
			alias, err := databaseAlias(plan.connection)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("SELECT * FROM %s.%s", alias, object), nil
		case semanticmodel.ObjectRelationQuackQuery:
			return quackQueryRelation(plan.connectionConfig.Path, object, plan.connectionConfig.Options)
		default:
			return "", fmt.Errorf("unsupported object relation mode %q", plan.connectionSpec.ObjectRelation)
		}
	default:
		return "", fmt.Errorf("unsupported source kind %q", plan.kind)
	}
}

func connectionRequiresObjectAttach(connection semanticmodel.ConnectionSpec) bool {
	return connection.ObjectRelation == semanticmodel.ObjectRelationAttach
}

func quackQueryRelation(uri, object string, options map[string]any) (string, error) {
	args := []string{
		"'" + sqlString(uri) + "'",
		"'" + sqlString("SELECT * FROM "+object) + "'",
	}
	if value, ok := options["disable_ssl"]; ok {
		disableSSL, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("quack disable_ssl option must be a boolean")
		}
		args = append(args, fmt.Sprintf("disable_ssl => %t", disableSSL))
	}
	return fmt.Sprintf("SELECT * FROM quack_query(%s)", strings.Join(args, ", ")), nil
}

func replacementScanRelation(path string) string {
	return fmt.Sprintf("SELECT * FROM '%s'", sqlString(path))
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

func RequiredExtensions(model *semanticmodel.Model) []string {
	extensions := map[string]struct{}{}
	addConnection := func(kind string) {
		if connection, ok := semanticmodel.LookupConnection(kind); ok && connection.RequiredExtension != "" {
			extensions[connection.RequiredExtension] = struct{}{}
		}
	}
	addPath := func(path string) {
		if extension, ok := semanticmodel.StorageExtension(path); ok {
			extensions[extension] = struct{}{}
		}
	}
	for _, name := range sortedKeys(model.Connections) {
		connection := model.Connections[name]
		addConnection(connection.Kind)
		addPath(connection.Path)
		if dataPath, ok := connection.Options["data_path"]; ok {
			addPath(fmt.Sprint(dataPath))
		}
	}
	for _, name := range sortedKeys(model.Sources) {
		source := model.Sources[name]
		addPath(source.Path)
		switch source.Kind() {
		case semanticmodel.KindPath:
			if format, ok := semanticmodel.LookupFormat(source.Format); ok && format.RequiredExtension != "" {
				extensions[format.RequiredExtension] = struct{}{}
			}
		case semanticmodel.KindObject:
			connection := model.Connections[source.Connection]
			addConnection(connection.Kind)
		}
	}
	return sortedKeys(extensions)
}

func duckDBConnectionType(kind string) string {
	if connection, ok := semanticmodel.LookupConnection(kind); ok && connection.SecretType != "" {
		return connection.SecretType
	}
	return kind
}

func compileSourceSecretStatements(model *semanticmodel.Model) ([]string, error) {
	statements := map[string]string{}
	for _, sourceName := range sortedKeys(model.Sources) {
		source := model.Sources[sourceName]
		if source.Kind() != semanticmodel.KindPath {
			continue
		}
		format, ok := semanticmodel.LookupFormat(source.Format)
		if !ok || format.SourceSecretType == "" {
			continue
		}
		connection := model.Connections[source.Connection]
		if len(connection.Auth) == 0 {
			continue
		}
		stmt, ok, err := compileTypedConnectionSecret(source.Connection+"_"+format.SourceSecretType, connection, format.SourceSecretType)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		statements[source.Connection] = stmt
	}
	result := make([]string, 0, len(statements))
	for _, name := range sortedKeys(statements) {
		result = append(result, statements[name])
	}
	return result, nil
}

func compileConnectionSecret(name string, connection semanticmodel.Connection) (string, bool, error) {
	connectionSpec, ok := semanticmodel.LookupConnection(connection.Kind)
	if !ok || connectionSpec.SecretType == "" {
		return "", false, nil
	}
	if connectionSpec.AttachKind == semanticmodel.AttachDatabase {
		return "", false, nil
	}
	return compileTypedConnectionSecret(name, connection, connectionSpec.SecretType)
}

func compileTypedConnectionSecret(name string, connection semanticmodel.Connection, secretType string) (string, bool, error) {
	if len(connection.Auth) == 0 {
		return "", false, nil
	}
	secret, err := connectionSecretName(name)
	if err != nil {
		return "", false, err
	}
	parts := []string{"TYPE " + secretType}
	if secretType != "quack" {
		parts = append(parts, "PROVIDER "+duckDBSecretProvider(secretType, connection.Auth))
	}
	for _, key := range sortedKeys(connection.Auth) {
		if err := validateIdentifier(key); err != nil {
			return "", false, fmt.Errorf("invalid auth param %q: %w", key, err)
		}
		parts = append(parts, duckDBAuthParameter(key)+" "+sqlLiteral(connection.Auth[key]))
	}
	if scope := duckDBSecretScope(secretType, connection); scope != "" {
		parts = append(parts, "SCOPE '"+sqlString(scope)+"'")
	}
	return fmt.Sprintf("CREATE OR REPLACE SECRET %s (%s)", secret, strings.Join(parts, ", ")), true, nil
}

func duckDBSecretScope(secretType string, connection semanticmodel.Connection) string {
	if connection.Scope != "" {
		return connection.Scope
	}
	if secretType == "quack" {
		return connection.Path
	}
	return ""
}

func duckDBSecretProvider(secretType string, auth semanticmodel.ConnectionAuth) string {
	if secretType == "azure" {
		if _, hasTenant := auth["tenant_id"]; hasTenant {
			if _, hasClient := auth["client_id"]; hasClient {
				if _, hasSecret := auth["client_secret"]; hasSecret {
					return "service_principal"
				}
			}
		}
	}
	return "config"
}

func duckDBAuthParameter(key string) string {
	switch key {
	case "access_key_id":
		return "KEY_ID"
	case "secret_access_key":
		return "SECRET"
	case "account_name":
		return "ACCOUNT_NAME"
	case "account_id":
		return "ACCOUNT_ID"
	case "client_id":
		return "CLIENT_ID"
	case "client_secret":
		return "CLIENT_SECRET"
	case "connection_string":
		return "CONNECTION_STRING"
	case "session_token":
		return "SESSION_TOKEN"
	case "tenant_id":
		return "TENANT_ID"
	case "token":
		return "TOKEN"
	case "url_style":
		return "URL_STYLE"
	case "use_ssl":
		return "USE_SSL"
	default:
		return strings.ToUpper(key)
	}
}

func compileObjectAttach(model *semanticmodel.Model, connectionName string, connection semanticmodel.Connection, dataDir string) (string, error) {
	connectionSpec, ok := semanticmodel.LookupConnection(connection.Kind)
	if !ok {
		return "", fmt.Errorf("unsupported connection kind %q", connection.Kind)
	}
	if connectionSpec.AttachKind == semanticmodel.AttachDuckLake {
		return compileDuckLakeAttach(model, connectionName, connection, dataDir)
	}
	if connectionSpec.AttachKind == "" {
		return "", nil
	}
	return compileDatabaseAttach(connectionName, connection)
}

func compileDatabaseAttach(connectionName string, connection semanticmodel.Connection) (string, error) {
	alias, err := databaseAlias(connectionName)
	if err != nil {
		return "", err
	}
	connectionString, err := connectionStringOption(connection)
	if err != nil {
		return "", err
	}
	parts := []string{"TYPE " + duckDBConnectionType(connection.Kind), "READ_ONLY"}
	if secret, ok, err := databaseAttachSecret(connectionName, connection); err != nil {
		return "", err
	} else if ok {
		parts = append(parts, "SECRET "+secret)
	}
	return fmt.Sprintf("ATTACH '%s' AS %s (%s)", sqlString(connectionString), alias, strings.Join(parts, ", ")), nil
}

func compileDuckLakeAttach(model *semanticmodel.Model, connectionName string, connection semanticmodel.Connection, dataDir string) (string, error) {
	alias, err := databaseAlias(connectionName)
	if err != nil {
		return "", err
	}
	path, err := resolveConnectionPath(model, connection, dataDir)
	if err != nil {
		return "", err
	}
	attachPath := path
	if !strings.HasPrefix(attachPath, "ducklake:") {
		attachPath = "ducklake:" + attachPath
	}
	parts := []string{}
	if dataPath, ok := connection.Options["data_path"]; ok {
		resolved, err := resolvePathInConnectionScope(model, connection, fmt.Sprint(dataPath), dataDir)
		if err != nil {
			return "", err
		}
		parts = append(parts, "DATA_PATH '"+sqlString(resolved)+"'")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("ATTACH '%s' AS %s", sqlString(attachPath), alias), nil
	}
	return fmt.Sprintf("ATTACH '%s' AS %s (%s)", sqlString(attachPath), alias, strings.Join(parts, ", ")), nil
}

func resolveConnectionPath(model *semanticmodel.Model, connection semanticmodel.Connection, dataDir string) (string, error) {
	return resolvePathInConnectionScope(model, connection, connection.Path, dataDir)
}

func resolvePathInConnectionScope(_ *semanticmodel.Model, connection semanticmodel.Connection, path string, dataDir string) (string, error) {
	if connection.Scope != "" {
		if semanticmodel.IsLocalPath(path) {
			return semanticmodel.JoinScope(connection.Scope, path), nil
		}
		if !semanticmodel.WithinScope(connection.Scope, path) {
			return "", fmt.Errorf("path %q is outside connection scope %q", path, connection.Scope)
		}
		return path, nil
	}
	if filepath.IsAbs(path) || !semanticmodel.IsLocalPath(path) {
		return path, nil
	}
	return filepath.Join(dataDir, path), nil
}

func databaseAttachSecret(connectionName string, connection semanticmodel.Connection) (string, bool, error) {
	if len(connection.Auth) == 0 {
		return "", false, nil
	}
	if _, ok := connection.Auth["connection_string"]; ok {
		return "", false, nil
	}
	if _, ok := connection.Auth["path"]; ok {
		return "", false, nil
	}
	secret, err := connectionSecretName(connectionName)
	if err != nil {
		return "", false, err
	}
	return secret, true, nil
}

func connectionSecretName(name string) (string, error) {
	if err := validateIdentifier(name); err != nil {
		return "", fmt.Errorf("invalid connection %q: %w", name, err)
	}
	return "libredash_" + name, nil
}

func connectionStringOption(connection semanticmodel.Connection) (string, error) {
	for key := range connection.Options {
		connectionSpec, _ := semanticmodel.LookupConnection(connection.Kind)
		if !connectionAllowsOption(connectionSpec, key) {
			return "", fmt.Errorf("unsupported database connection option %q", key)
		}
	}
	if len(connection.Auth) > 0 {
		if value, ok := connection.Auth["connection_string"]; ok {
			return fmt.Sprint(value), nil
		}
		if connection.Kind == "sqlite" {
			if value, ok := connection.Auth["path"]; ok {
				return fmt.Sprint(value), nil
			}
		}
		return "", nil
	}
	if connection.Kind == "sqlite" {
		if value, ok := connection.Options["path"]; ok {
			return fmt.Sprint(value), nil
		}
	}
	return "", nil
}

func connectionAllowsOption(connection semanticmodel.ConnectionSpec, option string) bool {
	for _, allowed := range connection.AllowedOptions {
		if option == allowed {
			return true
		}
	}
	return false
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

func validateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func SQLString(path string) string {
	return sqlString(path)
}

func sqlString(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), "'", "''")
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
