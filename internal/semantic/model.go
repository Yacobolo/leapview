package semantic

import (
	"fmt"
	"os"
	"sort"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"gopkg.in/yaml.v3"
)

type Model struct {
	Name    string                 `yaml:"name"`
	Title   string                 `yaml:"title"`
	Sources map[string]Source      `yaml:"sources"`
	Cache   Cache                  `yaml:"cache"`
	Metrics map[string]Metric      `yaml:"metrics"`
	Visuals map[string]Visual      `yaml:"visuals"`
	Tables  map[string]TableVisual `yaml:"tables"`
}

type Source struct {
	File string `yaml:"file"`
}

type Cache struct {
	Tables map[string]CacheTable `yaml:"tables"`
}

type CacheTable struct {
	Description string `yaml:"description"`
	SQL         string `yaml:"sql"`
}

type Metric struct {
	Title      string `yaml:"title"`
	Source     string `yaml:"source"`
	Aggregate  string `yaml:"aggregate"`
	Column     string `yaml:"column"`
	Expression string `yaml:"expression"`
	Note       string `yaml:"note"`
	Tone       string `yaml:"tone"`
	Format     string `yaml:"format"`
}

type Visual struct {
	Title     string `yaml:"title"`
	Unit      string `yaml:"unit"`
	Source    string `yaml:"source"`
	Label     string `yaml:"label"`
	LabelExpr string `yaml:"label_expr"`
	Aggregate string `yaml:"aggregate"`
	Value     string `yaml:"value"`
	ValueExpr string `yaml:"value_expr"`
	Where     string `yaml:"where"`
	OrderBy   string `yaml:"order_by"`
	Limit     int    `yaml:"limit"`
}

type TableVisual struct {
	Title       string                  `yaml:"title"`
	Source      string                  `yaml:"source"`
	DefaultSort dashboard.TableSort     `yaml:"default_sort"`
	Columns     []dashboard.TableColumn `yaml:"columns"`
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
	for name, source := range m.Sources {
		if source.File == "" {
			return fmt.Errorf("source %q is missing file", name)
		}
	}
	for name, table := range m.Cache.Tables {
		if table.SQL == "" {
			return fmt.Errorf("cache table %q is missing sql", name)
		}
	}
	for name, metric := range m.Metrics {
		if metric.Title == "" || metric.Source == "" || metric.Aggregate == "" {
			return fmt.Errorf("metric %q requires title, source, and aggregate", name)
		}
	}
	for name, visual := range m.Visuals {
		if visual.Title == "" || visual.Source == "" || visual.Aggregate == "" {
			return fmt.Errorf("visual %q requires title, source, and aggregate", name)
		}
		if visual.Label == "" && visual.LabelExpr == "" {
			return fmt.Errorf("visual %q requires label or label_expr", name)
		}
	}
	for name, table := range m.Tables {
		if table.Title == "" || table.Source == "" || len(table.Columns) == 0 {
			return fmt.Errorf("table %q requires title, source, and columns", name)
		}
	}
	return nil
}

func (m *Model) SourceFiles() map[string]string {
	files := make(map[string]string, len(m.Sources))
	for name, source := range m.Sources {
		files[name] = source.File
	}
	return files
}

func (m *Model) CacheTableNames() []string {
	names := make([]string, 0, len(m.Cache.Tables))
	for name := range m.Cache.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
