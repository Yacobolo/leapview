package semantic

import (
	"regexp"

	"github.com/Yacobolo/libredash/internal/analytics/model"
)

var semanticIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type modelFile struct {
	Name              string                       `yaml:"name"`
	Title             string                       `yaml:"title"`
	Description       string                       `yaml:"description"`
	DefaultConnection string                       `yaml:"default_connection"`
	Connections       map[string]model.Connection  `yaml:"connections"`
	Sources           map[string]model.Source      `yaml:"sources"`
	Models            map[string]model.Table       `yaml:"models"`
	SemanticModels    map[string]semanticModelSpec `yaml:"semantic_models"`
}

type semanticModelSpec struct {
	BaseTable     string                `yaml:"base_table"`
	Tables        []string              `yaml:"tables"`
	Relationships []model.Relationship  `yaml:"relationships"`
	Measures      semanticModelMeasures `yaml:"measures"`
}

type semanticModelMeasures struct {
	Defaults model.MeasureDefaults
	Items    map[string]model.MetricMeasure
}
