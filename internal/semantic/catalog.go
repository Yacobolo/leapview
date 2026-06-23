package semantic

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/workspace"
	"gopkg.in/yaml.v3"
)

func LoadWorkspace(path string) (*workspace.Definition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := rejectLegacyCatalogContract(content); err != nil {
		return nil, err
	}
	var catalog workspace.Catalog
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&catalog); err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)
	if err := catalog.Validate(baseDir); err != nil {
		return nil, err
	}

	definition := &workspace.Definition{
		Catalog:    catalog,
		Models:     map[string]*model.Model{},
		Dashboards: map[string]*report.Dashboard{},
		BaseDir:    baseDir,
	}

	for _, entry := range catalog.SemanticModels {
		model, err := Load(filepath.Join(baseDir, entry.Path))
		if err != nil {
			return nil, fmt.Errorf("loading semantic model %q: %w", entry.ID, err)
		}
		if model.Name != entry.ID {
			return nil, fmt.Errorf("catalog model %q path loads model %q", entry.ID, model.Name)
		}
		definition.Models[entry.ID] = model
	}

	for _, entry := range catalog.Dashboards {
		report, err := LoadDashboardWithModels(filepath.Join(baseDir, entry.Path), definition.Models)
		if err != nil {
			return nil, fmt.Errorf("loading dashboard %q: %w", entry.ID, err)
		}
		if report.ID != entry.ID {
			return nil, fmt.Errorf("catalog dashboard %q path loads dashboard %q", entry.ID, report.ID)
		}
		definition.Dashboards[entry.ID] = report
	}

	return definition, nil
}

func rejectLegacyCatalogContract(content []byte) error {
	var node yaml.Node
	if err := yaml.Unmarshal(content, &node); err != nil {
		return err
	}
	root := mappingNode(&node)
	if root == nil {
		return nil
	}
	if mappingValue(root, "metric_views") != nil || mappingValue(root, "metrics_views") != nil {
		return fmt.Errorf("catalog uses legacy metric views; use semantic_models and dashboards")
	}
	return nil
}
