package semantic

import (
	"fmt"
	"os"
	"sort"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"gopkg.in/yaml.v3"
)

type Model struct {
	Name          string                 `yaml:"name"`
	Title         string                 `yaml:"title"`
	Sources       map[string]Source      `yaml:"sources"`
	Cache         Cache                  `yaml:"cache"`
	Datasets      map[string]Dataset     `yaml:"datasets"`
	KPIs          map[string]KPI         `yaml:"kpis"`
	Visuals       map[string]Visual      `yaml:"visuals"`
	Tables        map[string]TableVisual `yaml:"tables"`
	Relationships []Relationship         `yaml:"relationships"`
	Pages         []dashboard.Page       `yaml:"pages"`
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

type Dataset struct {
	Source     string               `yaml:"source"`
	Dimensions map[string]Dimension `yaml:"dimensions"`
	Measures   map[string]Measure   `yaml:"measures"`
}

type Dimension struct {
	Label     string `yaml:"label"`
	Expr      string `yaml:"expr"`
	Where     string `yaml:"where"`
	OrderExpr string `yaml:"order_expr"`
}

type Measure struct {
	Label      string `yaml:"label"`
	Aggregate  string `yaml:"aggregate"`
	Column     string `yaml:"column"`
	Expression string `yaml:"expression"`
	Unit       string `yaml:"unit"`
	Format     string `yaml:"format"`
}

type KPI struct {
	Title   string `yaml:"title"`
	Dataset string `yaml:"dataset"`
	Measure string `yaml:"measure"`
	Note    string `yaml:"note"`
	Tone    string `yaml:"tone"`
}

type Visual struct {
	Title       string      `yaml:"title"`
	Type        string      `yaml:"type"`
	Stacked     bool        `yaml:"stacked"`
	Dataset     string      `yaml:"dataset"`
	Query       VisualQuery `yaml:"query"`
	Interaction Interaction `yaml:"interaction"`
}

type VisualQuery struct {
	Dimensions []string `yaml:"dimensions"`
	Series     string   `yaml:"series"`
	Measures   []string `yaml:"measures"`
	Sort       []Sort   `yaml:"sort"`
	Limit      int      `yaml:"limit"`
}

type Sort struct {
	Field     string `yaml:"field"`
	Direction string `yaml:"direction"`
	Expr      string `yaml:"expr"`
}

type Interaction struct {
	Field   string             `yaml:"field"`
	Targets InteractionTargets `yaml:"targets"`
}

type InteractionTargets struct {
	Visuals []string `yaml:"visuals"`
	Tables  []string `yaml:"tables"`
}

type TableVisual struct {
	Title       string                  `yaml:"title"`
	Dataset     string                  `yaml:"dataset"`
	DefaultSort dashboard.TableSort     `yaml:"default_sort"`
	Columns     []dashboard.TableColumn `yaml:"columns"`
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
		if len(dataset.Dimensions) == 0 {
			return fmt.Errorf("dataset %q requires dimensions", name)
		}
		if len(dataset.Measures) == 0 {
			return fmt.Errorf("dataset %q requires measures", name)
		}
		for dimensionName, dimension := range dataset.Dimensions {
			if dimension.Expr == "" {
				return fmt.Errorf("dataset %q dimension %q requires expr", name, dimensionName)
			}
		}
		for measureName, measure := range dataset.Measures {
			if measure.Aggregate == "" {
				return fmt.Errorf("dataset %q measure %q requires aggregate", name, measureName)
			}
			if measure.Aggregate != "count" && measure.Aggregate != "expression" && measure.Column == "" {
				return fmt.Errorf("dataset %q measure %q requires column", name, measureName)
			}
			if measure.Aggregate == "expression" && measure.Expression == "" {
				return fmt.Errorf("dataset %q measure %q requires expression", name, measureName)
			}
		}
	}
	for name, visual := range m.Visuals {
		if visual.Title == "" || visual.Dataset == "" || visual.Type == "" {
			return fmt.Errorf("visual %q requires title, dataset, and type", name)
		}
		dataset, ok := m.Datasets[visual.Dataset]
		if !ok {
			return fmt.Errorf("visual %q references unknown dataset %q", name, visual.Dataset)
		}
		if len(visual.Query.Dimensions) != 1 {
			return fmt.Errorf("visual %q requires exactly one query dimension", name)
		}
		if len(visual.Query.Measures) != 1 {
			return fmt.Errorf("visual %q requires exactly one query measure", name)
		}
		for _, dimension := range visual.Query.Dimensions {
			if _, ok := dataset.Dimensions[dimension]; !ok {
				return fmt.Errorf("visual %q references unknown dimension %q", name, dimension)
			}
		}
		if visual.Query.Series != "" {
			if _, ok := dataset.Dimensions[visual.Query.Series]; !ok {
				return fmt.Errorf("visual %q references unknown series dimension %q", name, visual.Query.Series)
			}
			if !supportsSeries(visual.Type) {
				return fmt.Errorf("visual %q type %q does not support series", name, visual.Type)
			}
		}
		if !supportsChartType(visual.Type) {
			return fmt.Errorf("visual %q has unsupported type %q", name, visual.Type)
		}
		for _, measure := range visual.Query.Measures {
			if _, ok := dataset.Measures[measure]; !ok {
				return fmt.Errorf("visual %q references unknown measure %q", name, measure)
			}
		}
		for _, sort := range visual.Query.Sort {
			if sort.Field == "" && sort.Expr == "" {
				return fmt.Errorf("visual %q has sort missing field or expr", name)
			}
			if sort.Field != "" && sort.Field != "value" && sort.Field != visual.Query.Series {
				if _, ok := dataset.Dimensions[sort.Field]; !ok {
					if _, ok := dataset.Measures[sort.Field]; !ok {
						return fmt.Errorf("visual %q sort references unknown field %q", name, sort.Field)
					}
				}
			}
		}
		if visual.Interaction.Field != "" {
			if _, ok := dataset.Dimensions[visual.Interaction.Field]; !ok {
				return fmt.Errorf("visual %q interaction references unknown field %q", name, visual.Interaction.Field)
			}
		}
	}
	for name, kpi := range m.KPIs {
		if kpi.Title == "" || kpi.Dataset == "" || kpi.Measure == "" {
			return fmt.Errorf("kpi %q requires title, dataset, and measure", name)
		}
		dataset, ok := m.Datasets[kpi.Dataset]
		if !ok {
			return fmt.Errorf("kpi %q references unknown dataset %q", name, kpi.Dataset)
		}
		if _, ok := dataset.Measures[kpi.Measure]; !ok {
			return fmt.Errorf("kpi %q references unknown measure %q", name, kpi.Measure)
		}
	}
	for name, table := range m.Tables {
		if table.Title == "" || table.Dataset == "" || len(table.Columns) == 0 {
			return fmt.Errorf("table %q requires title, dataset, and columns", name)
		}
		if _, ok := m.Datasets[table.Dataset]; !ok {
			return fmt.Errorf("table %q references unknown dataset %q", name, table.Dataset)
		}
	}
	for name, visual := range m.Visuals {
		for _, target := range visual.Interaction.Targets.Visuals {
			if _, ok := m.Visuals[target]; !ok {
				return fmt.Errorf("visual %q interaction references unknown target visual %q", name, target)
			}
		}
		for _, target := range visual.Interaction.Targets.Tables {
			if _, ok := m.Tables[target]; !ok {
				return fmt.Errorf("visual %q interaction references unknown target table %q", name, target)
			}
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
	seenPages := map[string]struct{}{}
	for index, page := range m.Pages {
		if page.ID == "" || page.Title == "" {
			return fmt.Errorf("page %d requires id and title", index)
		}
		if _, exists := seenPages[page.ID]; exists {
			return fmt.Errorf("duplicate page id %q", page.ID)
		}
		seenPages[page.ID] = struct{}{}
		for _, visual := range page.Visuals {
			if visual.ID == "" || visual.Kind == "" {
				return fmt.Errorf("page %q has a visual missing id or kind", page.ID)
			}
			switch visual.Kind {
			case "header", "kpi_strip":
			case "line_chart", "area_chart", "bar_chart", "column_chart", "pie_chart", "donut_chart", "scatter_chart", "funnel_chart", "treemap_chart", "gauge_chart":
				if visual.Visual == "" {
					return fmt.Errorf("page %q visual %q requires visual", page.ID, visual.ID)
				}
				if _, ok := m.Visuals[visual.Visual]; !ok {
					return fmt.Errorf("page %q references unknown visual %q", page.ID, visual.Visual)
				}
			case "table":
				if visual.Table == "" {
					return fmt.Errorf("page %q visual %q requires table", page.ID, visual.ID)
				}
				if _, ok := m.Tables[visual.Table]; !ok {
					return fmt.Errorf("page %q references unknown table %q", page.ID, visual.Table)
				}
			default:
				return fmt.Errorf("page %q visual %q has unsupported kind %q", page.ID, visual.ID, visual.Kind)
			}
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

func supportsChartType(chartType string) bool {
	switch chartType {
	case "line", "area", "bar", "column", "pie", "donut", "scatter", "funnel", "treemap", "gauge":
		return true
	default:
		return false
	}
}

func supportsSeries(chartType string) bool {
	switch chartType {
	case "line", "area", "bar", "column", "scatter":
		return true
	default:
		return false
	}
}
