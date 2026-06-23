package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
)

type Catalog struct {
	Workspace      CatalogWorkspace   `yaml:"workspace"`
	SemanticModels []CatalogModel     `yaml:"semantic_models"`
	Dashboards     []CatalogDashboard `yaml:"dashboards"`
}

type CatalogWorkspace struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

type CatalogModel struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Path        string `yaml:"path"`
	Description string `yaml:"description"`
}

type CatalogDashboard struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Path        string   `yaml:"path"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

type Definition struct {
	Catalog    Catalog
	Models     map[string]*model.Model
	Dashboards map[string]*report.Dashboard
	BaseDir    string
}

func (c Catalog) Validate(baseDir string) error {
	if len(c.SemanticModels) == 0 {
		return fmt.Errorf("catalog requires semantic_models")
	}
	if len(c.Dashboards) == 0 {
		return fmt.Errorf("catalog requires dashboards")
	}
	models := map[string]struct{}{}
	for index, model := range c.SemanticModels {
		if model.ID == "" || model.Title == "" || model.Path == "" {
			return fmt.Errorf("catalog semantic model %d requires id, title, and path", index)
		}
		if _, exists := models[model.ID]; exists {
			return fmt.Errorf("duplicate semantic model id %q", model.ID)
		}
		models[model.ID] = struct{}{}
		if _, err := os.Stat(filepath.Join(baseDir, model.Path)); err != nil {
			return fmt.Errorf("semantic model %q path %q: %w", model.ID, model.Path, err)
		}
	}

	dashboards := map[string]struct{}{}
	for index, report := range c.Dashboards {
		if report.ID == "" || report.Title == "" || report.Path == "" {
			return fmt.Errorf("catalog dashboard %d requires id, title, and path", index)
		}
		if _, exists := dashboards[report.ID]; exists {
			return fmt.Errorf("duplicate dashboard id %q", report.ID)
		}
		dashboards[report.ID] = struct{}{}
		if _, err := os.Stat(filepath.Join(baseDir, report.Path)); err != nil {
			return fmt.Errorf("dashboard %q path %q: %w", report.ID, report.Path, err)
		}
	}
	return nil
}
