package model

import "regexp"

var (
	semanticIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envReferencePattern       = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
)

type Model struct {
	Name              string                   `yaml:"-"`
	Title             string                   `yaml:"-"`
	Description       string                   `yaml:"-"`
	DefaultConnection string                   `yaml:"-"`
	Connections       map[string]Connection    `yaml:"-"`
	Sources           map[string]Source        `yaml:"-"`
	Tables            map[string]Table         `yaml:"-"`
	BaseTable         string                   `yaml:"-"`
	Relationships     []Relationship           `yaml:"-"`
	Measures          map[string]MetricMeasure `yaml:"-"`
}

type Connection struct {
	Kind     string             `yaml:"kind"`
	Path     string             `yaml:"path"`
	Root     string             `yaml:"root"`
	Scope    string             `yaml:"scope"`
	Auth     ConnectionAuth     `yaml:"auth"`
	Options  map[string]any     `yaml:"options"`
	Defaults ConnectionDefaults `yaml:"defaults"`
}

type ConnectionDefaults struct {
	Options map[string]any `yaml:"options"`
}

type ConnectionAuth map[string]any

type Source struct {
	Format     string         `yaml:"format"`
	Path       string         `yaml:"path"`
	Connection string         `yaml:"connection"`
	Object     string         `yaml:"object"`
	Options    map[string]any `yaml:"options"`
}

type Table struct {
	Kind               string                     `yaml:"kind"`
	Source             string                     `yaml:"source"`
	Sources            []string                   `yaml:"sources"`
	SQL                string                     `yaml:"sql"`
	Transform          Transform                  `yaml:"transform"`
	PrimaryKey         string                     `yaml:"primary_key"`
	Grain              string                     `yaml:"grain"`
	Dimensions         map[string]MetricDimension `yaml:"dimensions"`
	Measures           map[string]MetricMeasure   `yaml:"measures"`
	Description        string                     `yaml:"description"`
	SourceDependencies []string                   `yaml:"-"`
}

type Transform struct {
	SQL string `yaml:"sql"`
}

type MeasureDefaults struct {
	Table  string   `yaml:"table"`
	Grain  string   `yaml:"grain"`
	Time   string   `yaml:"time"`
	Grains []string `yaml:"grains"`
}

type MetricDimension struct {
	Field      string `yaml:"-"`
	Table      string `yaml:"-"`
	Name       string `yaml:"-"`
	Label      string `yaml:"label"`
	Expr       string `yaml:"expr"`
	Expression string `yaml:"expression"`
	Where      string `yaml:"where"`
	OrderExpr  string `yaml:"order_expr"`
	Type       string `yaml:"type"`
}

type MetricMeasure struct {
	Field       string   `yaml:"-"`
	Table       string   `yaml:"table"`
	Name        string   `yaml:"-"`
	Label       string   `yaml:"label"`
	Description string   `yaml:"description"`
	Expr        string   `yaml:"expr"`
	Expression  string   `yaml:"expression"`
	Unit        string   `yaml:"unit"`
	Format      string   `yaml:"format"`
	Grain       string   `yaml:"grain"`
	Time        string   `yaml:"time"`
	Grains      []string `yaml:"grains"`
}

type Relationship struct {
	ID          string `yaml:"id"`
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	Cardinality string `yaml:"cardinality"`
	Active      bool   `yaml:"active"`
}
