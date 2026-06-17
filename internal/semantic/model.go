package semantic

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var semanticIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Model struct {
	Name          string                `yaml:"name"`
	Title         string                `yaml:"title"`
	Description   string                `yaml:"description"`
	Connections   map[string]Connection `yaml:"connections"`
	Sources       map[string]Source     `yaml:"sources"`
	Cache         Cache                 `yaml:"cache"`
	Datasets      map[string]Dataset    `yaml:"datasets"`
	Relationships []Relationship        `yaml:"relationships"`
}

type Connection struct {
	Type    string         `yaml:"type"`
	Secret  string         `yaml:"secret"`
	Scope   string         `yaml:"scope"`
	Auth    ConnectionAuth `yaml:"auth"`
	Options map[string]any `yaml:"options"`
}

type ConnectionAuth struct {
	Method  string         `yaml:"method"`
	Profile string         `yaml:"profile"`
	Chain   string         `yaml:"chain"`
	Account string         `yaml:"account"`
	Params  map[string]any `yaml:"params"`
}

type Source struct {
	Type       string         `yaml:"type"`
	Format     string         `yaml:"format"`
	Location   string         `yaml:"location"`
	Connection string         `yaml:"connection"`
	Engine     string         `yaml:"engine"`
	Object     string         `yaml:"object"`
	Query      string         `yaml:"query"`
	Options    map[string]any `yaml:"options"`
}

type Cache struct {
	Tables map[string]CacheTable `yaml:"tables"`
}

type CacheTable struct {
	Description string `yaml:"description"`
	SQL         string `yaml:"sql"`
}

type Dataset struct {
	Source string `yaml:"source"`
}

type MetricView struct {
	ID            string                     `yaml:"id"`
	Title         string                     `yaml:"title"`
	Description   string                     `yaml:"description"`
	SemanticModel string                     `yaml:"semantic_model"`
	Dataset       string                     `yaml:"dataset"`
	Timeseries    string                     `yaml:"timeseries"`
	Dimensions    map[string]MetricDimension `yaml:"dimensions"`
	Measures      map[string]MetricMeasure   `yaml:"measures"`
}

type MetricDimension struct {
	Label     string `yaml:"label"`
	Expr      string `yaml:"expr"`
	Where     string `yaml:"where"`
	OrderExpr string `yaml:"order_expr"`
}

type MetricMeasure struct {
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Expression  string `yaml:"expression"`
	Unit        string `yaml:"unit"`
	Format      string `yaml:"format"`
}

type Relationship struct {
	ID          string `yaml:"id"`
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	Cardinality string `yaml:"cardinality"`
	Active      bool   `yaml:"active"`
}

func Load(path string) (*Model, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var model Model
	if err := yaml.Unmarshal(bytes, &model); err != nil {
		return nil, err
	}
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &model, nil
}

func (m *Model) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("semantic model name is required")
	}
	if len(m.Sources) == 0 {
		return fmt.Errorf("semantic model %q has no sources", m.Name)
	}
	if len(m.Cache.Tables) == 0 {
		return fmt.Errorf("semantic model %q has no cache tables", m.Name)
	}
	for name, connection := range m.Connections {
		if err := connection.Validate(name); err != nil {
			return err
		}
	}
	for name, source := range m.Sources {
		if err := source.Validate(name, m.Connections); err != nil {
			return err
		}
	}
	for name, table := range m.Cache.Tables {
		if table.SQL == "" {
			return fmt.Errorf("cache table %q is missing sql", name)
		}
	}
	if len(m.Datasets) == 0 {
		return fmt.Errorf("semantic model %q has no datasets", m.Name)
	}
	for name, dataset := range m.Datasets {
		if dataset.Source == "" {
			return fmt.Errorf("dataset %q requires source", name)
		}
		if _, ok := m.Cache.Tables[dataset.Source]; !ok {
			return fmt.Errorf("dataset %q references unknown cache table %q", name, dataset.Source)
		}
	}
	seenRelationships := map[string]struct{}{}
	for index, relationship := range m.Relationships {
		if relationship.ID == "" || relationship.From == "" || relationship.To == "" {
			return fmt.Errorf("relationship %d requires id, from, and to", index)
		}
		if _, exists := seenRelationships[relationship.ID]; exists {
			return fmt.Errorf("duplicate relationship id %q", relationship.ID)
		}
		seenRelationships[relationship.ID] = struct{}{}
	}
	return nil
}

func LoadMetricView(path string, model *Model) (*MetricView, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var view MetricView
	if err := yaml.Unmarshal(bytes, &view); err != nil {
		return nil, err
	}
	if err := view.Validate(model); err != nil {
		return nil, err
	}
	return &view, nil
}

func (v *MetricView) Validate(model *Model) error {
	if v.ID == "" || v.Title == "" || v.SemanticModel == "" || v.Dataset == "" {
		return fmt.Errorf("metrics view requires id, title, semantic_model, and dataset")
	}
	if model == nil {
		return fmt.Errorf("metrics view %q requires semantic model %q", v.ID, v.SemanticModel)
	}
	if v.SemanticModel != model.Name {
		return fmt.Errorf("metrics view %q semantic_model %q does not match model %q", v.ID, v.SemanticModel, model.Name)
	}
	if _, ok := model.Datasets[v.Dataset]; !ok {
		return fmt.Errorf("metrics view %q references unknown dataset %q", v.ID, v.Dataset)
	}
	if v.Timeseries == "" {
		return fmt.Errorf("metrics view %q requires timeseries", v.ID)
	}
	if len(v.Dimensions) == 0 {
		return fmt.Errorf("metrics view %q requires dimensions", v.ID)
	}
	if len(v.Measures) == 0 {
		return fmt.Errorf("metrics view %q requires measures", v.ID)
	}
	if _, ok := v.Dimensions[v.Timeseries]; !ok {
		return fmt.Errorf("metrics view %q timeseries %q is not a dimension", v.ID, v.Timeseries)
	}
	for name, dimension := range v.Dimensions {
		if dimension.Expr == "" {
			return fmt.Errorf("metrics view %q dimension %q requires expr", v.ID, name)
		}
	}
	for name, measure := range v.Measures {
		if measure.Label == "" || measure.Expression == "" {
			return fmt.Errorf("metrics view %q measure %q requires label and expression", v.ID, name)
		}
	}
	return nil
}

func (m *Model) SourceFiles() map[string]string {
	files := make(map[string]string, len(m.Sources))
	for name, source := range m.Sources {
		if source.Type == "file" && isLocalLocation(source.Location) {
			files[name] = source.Location
		}
	}
	return files
}

func (s Source) Validate(name string, connections map[string]Connection) error {
	if err := validateSemanticIdentifier(name); err != nil {
		return fmt.Errorf("source %q has invalid name: %w", name, err)
	}
	if s.Type == "" {
		return fmt.Errorf("source %q requires type", name)
	}
	for key := range s.Options {
		if err := validateSemanticIdentifier(key); err != nil {
			return fmt.Errorf("source %q option %q is invalid: %w", name, key, err)
		}
	}
	switch s.Type {
	case "file":
		if s.Format == "" || s.Location == "" {
			return fmt.Errorf("source %q type file requires format and location", name)
		}
		if !supportsFileFormat(s.Format) {
			return fmt.Errorf("source %q has unsupported file format %q", name, s.Format)
		}
	case "lakehouse":
		if s.Format == "" || s.Location == "" {
			return fmt.Errorf("source %q type lakehouse requires format and location", name)
		}
		if !supportsLakehouseFormat(s.Format) {
			return fmt.Errorf("source %q has unsupported lakehouse format %q", name, s.Format)
		}
	case "database":
		if s.Engine == "" || s.Connection == "" || s.Object == "" {
			return fmt.Errorf("source %q type database requires engine, connection, and object", name)
		}
		if !supportsDatabaseEngine(s.Engine) {
			return fmt.Errorf("source %q has unsupported database engine %q", name, s.Engine)
		}
	case "query":
		if s.Query == "" {
			return fmt.Errorf("source %q type query requires query", name)
		}
	default:
		return fmt.Errorf("source %q has unsupported type %q", name, s.Type)
	}
	if s.Connection != "" {
		connection, ok := connections[s.Connection]
		if !ok {
			return fmt.Errorf("source %q references unknown connection %q", name, s.Connection)
		}
		if s.Type == "database" && connection.Type != s.Engine {
			return fmt.Errorf("source %q database engine %q does not match connection %q type %q", name, s.Engine, s.Connection, connection.Type)
		}
	}
	return nil
}

func (c Connection) Validate(name string) error {
	if err := validateSemanticIdentifier(name); err != nil {
		return fmt.Errorf("connection %q has invalid name: %w", name, err)
	}
	if c.Type == "" {
		return fmt.Errorf("connection %q requires type", name)
	}
	if !supportsConnectionType(c.Type) {
		return fmt.Errorf("connection %q has unsupported type %q", name, c.Type)
	}
	if c.Secret != "" {
		if err := validateSemanticIdentifier(c.Secret); err != nil {
			return fmt.Errorf("connection %q secret %q is invalid: %w", name, c.Secret, err)
		}
	}
	if c.Auth.Method != "" && !supportsAuthMethod(c.Auth.Method) {
		return fmt.Errorf("connection %q has unsupported auth method %q", name, c.Auth.Method)
	}
	for key := range c.Auth.Params {
		if err := validateSemanticIdentifier(key); err != nil {
			return fmt.Errorf("connection %q auth param %q is invalid: %w", name, key, err)
		}
	}
	for key := range c.Options {
		if !supportsConnectionOption(key) {
			return fmt.Errorf("connection %q has unsupported option %q", name, key)
		}
	}
	return nil
}

func (s Source) Description() string {
	switch s.Type {
	case "file":
		return s.Format + " file: " + s.Location
	case "lakehouse":
		return s.Format + " table: " + s.Location
	case "database":
		return s.Engine + " table: " + s.Object
	case "query":
		return "trusted query source"
	default:
		return s.Type
	}
}

func (s Source) Role() string {
	switch s.Type {
	case "file":
		return s.Format
	case "lakehouse":
		return s.Format
	case "database":
		return s.Engine
	case "query":
		return "query"
	default:
		return s.Type
	}
}

func isLocalLocation(location string) bool {
	for _, prefix := range []string{"s3://", "r2://", "gcs://", "gs://", "az://", "azure://", "abfss://", "http://", "https://", "file://"} {
		if strings.HasPrefix(location, prefix) {
			return false
		}
	}
	return !strings.Contains(location, "://")
}

func supportsConnectionType(connectionType string) bool {
	switch connectionType {
	case "s3", "r2", "gcs", "azure", "http", "postgres", "mysql", "sqlite":
		return true
	default:
		return false
	}
}

func supportsAuthMethod(method string) bool {
	switch method {
	case "credential_chain", "config":
		return true
	default:
		return false
	}
}

func supportsConnectionOption(option string) bool {
	switch option {
	case "connection_string", "uri", "path", "database":
		return true
	default:
		return false
	}
}

func validateSemanticIdentifier(value string) error {
	if !semanticIdentifierPattern.MatchString(value) {
		return fmt.Errorf("must match %s", semanticIdentifierPattern.String())
	}
	return nil
}

func supportsFileFormat(format string) bool {
	switch format {
	case "csv", "json", "parquet", "excel":
		return true
	default:
		return false
	}
}

func supportsLakehouseFormat(format string) bool {
	switch format {
	case "delta", "iceberg":
		return true
	default:
		return false
	}
}

func supportsDatabaseEngine(engine string) bool {
	switch engine {
	case "postgres", "mysql", "sqlite":
		return true
	default:
		return false
	}
}

func (m *Model) CacheTableNames() []string {
	names := make([]string, 0, len(m.Cache.Tables))
	for name := range m.Cache.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func supportsVisualKind(kind string) bool {
	return kind == "chart"
}

func supportsVisualShape(shape string) bool {
	switch shape {
	case "category_value", "category_series_value", "category_multi_measure", "category_delta", "single_value", "matrix", "graph", "geo", "ohlc", "distribution", "binned_measure", "hierarchy":
		return true
	default:
		return false
	}
}

func supportsRenderer(renderer string) bool {
	return renderer == "echarts"
}

func rendererSupportsType(renderer, chartType string) bool {
	if renderer != "echarts" {
		return false
	}
	switch chartType {
	case "line", "area", "bar", "column", "pie", "donut", "scatter", "funnel", "treemap", "gauge", "heatmap", "sankey", "graph", "map", "candlestick", "boxplot", "combo", "waterfall", "histogram", "radar", "tree", "sunburst":
		return true
	default:
		return false
	}
}

func supportsSeries(shape string) bool {
	return shape == "category_series_value"
}

func rendererSupportsShapeType(renderer, shape, chartType string) bool {
	if renderer != "echarts" {
		return false
	}
	switch shape {
	case "category_value":
		switch chartType {
		case "line", "area", "bar", "column", "pie", "donut", "scatter", "funnel", "treemap", "radar":
			return true
		}
	case "category_series_value":
		return rendererTypeSupportsSeries(renderer, chartType)
	case "category_multi_measure":
		return chartType == "combo"
	case "category_delta":
		return chartType == "waterfall"
	case "single_value":
		return chartType == "gauge"
	case "matrix":
		return chartType == "heatmap"
	case "graph":
		return chartType == "sankey" || chartType == "graph"
	case "geo":
		return chartType == "map"
	case "ohlc":
		return chartType == "candlestick"
	case "distribution":
		return chartType == "boxplot"
	case "binned_measure":
		return chartType == "histogram"
	case "hierarchy":
		return chartType == "tree" || chartType == "sunburst"
	}
	return false
}

func rendererTypeSupportsSeries(renderer, chartType string) bool {
	if renderer != "echarts" {
		return false
	}
	switch chartType {
	case "line", "area", "bar", "column", "scatter":
		return true
	default:
		return false
	}
}
