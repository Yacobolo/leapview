package semantic

import (
	"fmt"
	"os"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"gopkg.in/yaml.v3"
)

type Dashboard struct {
	ID            string                      `yaml:"id"`
	Title         string                      `yaml:"title"`
	Description   string                      `yaml:"description"`
	SemanticModel string                      `yaml:"semantic_model"`
	Filters       map[string]FilterDefinition `yaml:"filters"`
	KPIs          map[string]KPI              `yaml:"kpis"`
	Visuals       map[string]Visual           `yaml:"visuals"`
	Tables        map[string]TableVisual      `yaml:"tables"`
	Pages         []dashboard.Page            `yaml:"pages"`
}

type FilterDefinition struct {
	Type    string         `yaml:"type"`
	Label   string         `yaml:"label"`
	Default string         `yaml:"default"`
	Options []FilterOption `yaml:"options"`
}

type FilterOption struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
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

func LoadDashboard(path string, model *Model) (*Dashboard, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report Dashboard
	if err := yaml.Unmarshal(bytes, &report); err != nil {
		return nil, err
	}
	if err := report.Validate(model); err != nil {
		return nil, err
	}
	return &report, nil
}

func (d *Dashboard) Validate(model *Model) error {
	if d.ID == "" || d.Title == "" || d.SemanticModel == "" {
		return fmt.Errorf("dashboard requires id, title, and semantic_model")
	}
	if model == nil {
		return fmt.Errorf("dashboard %q requires semantic model %q", d.ID, d.SemanticModel)
	}
	if d.SemanticModel != model.Name {
		return fmt.Errorf("dashboard %q semantic_model %q does not match model %q", d.ID, d.SemanticModel, model.Name)
	}
	if len(d.KPIs) == 0 {
		return fmt.Errorf("dashboard %q requires kpis", d.ID)
	}
	if len(d.Visuals) == 0 {
		return fmt.Errorf("dashboard %q requires visuals", d.ID)
	}
	if len(d.Pages) == 0 {
		return fmt.Errorf("dashboard %q requires pages", d.ID)
	}
	for name, kpi := range d.KPIs {
		if kpi.Title == "" || kpi.Dataset == "" || kpi.Measure == "" {
			return fmt.Errorf("kpi %q requires title, dataset, and measure", name)
		}
		dataset, ok := model.Datasets[kpi.Dataset]
		if !ok {
			return fmt.Errorf("kpi %q references unknown dataset %q", name, kpi.Dataset)
		}
		if _, ok := dataset.Measures[kpi.Measure]; !ok {
			return fmt.Errorf("kpi %q references unknown measure %q", name, kpi.Measure)
		}
	}
	for name, visual := range d.Visuals {
		if visual.Title == "" || visual.Dataset == "" || visual.Type == "" {
			return fmt.Errorf("visual %q requires title, dataset, and type", name)
		}
		dataset, ok := model.Datasets[visual.Dataset]
		if !ok {
			return fmt.Errorf("visual %q references unknown dataset %q", name, visual.Dataset)
		}
		if !supportsChartType(visual.Type) {
			return fmt.Errorf("visual %q has unsupported type %q", name, visual.Type)
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
	for name, table := range d.Tables {
		if table.Title == "" || table.Dataset == "" || len(table.Columns) == 0 {
			return fmt.Errorf("table %q requires title, dataset, and columns", name)
		}
		if _, ok := model.Datasets[table.Dataset]; !ok {
			return fmt.Errorf("table %q references unknown dataset %q", name, table.Dataset)
		}
	}
	for name, visual := range d.Visuals {
		for _, target := range visual.Interaction.Targets.Visuals {
			if _, ok := d.Visuals[target]; !ok {
				return fmt.Errorf("visual %q interaction references unknown target visual %q", name, target)
			}
		}
		for _, target := range visual.Interaction.Targets.Tables {
			if _, ok := d.Tables[target]; !ok {
				return fmt.Errorf("visual %q interaction references unknown target table %q", name, target)
			}
		}
	}
	seenPages := map[string]struct{}{}
	for index, page := range d.Pages {
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
				if _, ok := d.Visuals[visual.Visual]; !ok {
					return fmt.Errorf("page %q references unknown visual %q", page.ID, visual.Visual)
				}
			case "table":
				if visual.Table == "" {
					return fmt.Errorf("page %q visual %q requires table", page.ID, visual.ID)
				}
				if _, ok := d.Tables[visual.Table]; !ok {
					return fmt.Errorf("page %q references unknown table %q", page.ID, visual.Table)
				}
			default:
				return fmt.Errorf("page %q visual %q has unsupported kind %q", page.ID, visual.ID, visual.Kind)
			}
		}
	}
	return nil
}
