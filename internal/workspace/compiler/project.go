package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	analyticsmaterialize "github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/configschema"
	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/workspace"
	"gopkg.in/yaml.v3"
)

const projectAPIVersion = "libredash.dev/v1"

type Project struct {
	Name        string
	BaseDir     string
	Connections map[string]semanticmodel.Connection
	Sources     map[string]semanticmodel.Source
	Workspaces  map[string]*WorkspaceProject
}

type WorkspaceProject struct {
	ID                    string
	Title                 string
	Description           string
	AllowedSources        map[string]struct{}
	Models                map[string]semanticmodel.Table
	SemanticModels        map[string]projectSemanticModelSpec
	Dashboards            map[string]*report.Dashboard
	ModelTitles           map[string]string
	ModelDescriptions     map[string]string
	DashboardTitles       map[string]string
	DashboardDescriptions map[string]string
	DashboardTags         map[string][]string
}

type CompiledProject struct {
	Project    Project
	Workspaces map[string]CompiledWorkspace
}

type ProjectPlan struct {
	Project    string                 `json:"project"`
	Workspaces []ProjectPlanWorkspace `json:"workspaces"`
}

type ProjectPlanWorkspace struct {
	ID             string   `json:"id"`
	Connections    []string `json:"connections"`
	Sources        []string `json:"sources"`
	ModelTables    []string `json:"modelTables"`
	SemanticModels []string `json:"semanticModels"`
	Dashboards     []string `json:"dashboards"`
}

type resourceEnvelope struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   metadata  `yaml:"metadata"`
	Spec       yaml.Node `yaml:"spec"`
}

type metadata struct {
	Name        string   `yaml:"name"`
	Workspace   string   `yaml:"workspace"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Owner       string   `yaml:"owner"`
	Tags        []string `yaml:"tags"`
}

type projectResource struct {
	Connections includeList `yaml:"connections"`
	Sources     includeList `yaml:"sources"`
	Workspaces  includeList `yaml:"workspaces"`
}

type includeList struct {
	Include []string `yaml:"include"`
}

type workspaceSpec struct {
	Uses           workspaceUses `yaml:"uses"`
	Models         includeList   `yaml:"models"`
	SemanticModels includeList   `yaml:"semanticModels"`
	Dashboards     includeList   `yaml:"dashboards"`
}

type workspaceUses struct {
	Sources []string `yaml:"sources"`
}

type sourceSpec struct {
	Format      string                        `yaml:"format"`
	Description string                        `yaml:"description"`
	Path        string                        `yaml:"path"`
	Connection  string                        `yaml:"connection"`
	Object      string                        `yaml:"object"`
	Options     map[string]any                `yaml:"options"`
	Fields      map[string]projectSourceField `yaml:"fields"`
}

type projectSourceField struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

type projectSemanticModelSpec struct {
	BaseTable     string                       `yaml:"baseTable"`
	Tables        []string                     `yaml:"tables"`
	Relationships []semanticmodel.Relationship `yaml:"relationships"`
	Measures      projectSemanticModelMeasures `yaml:"measures"`
}

type projectSemanticModelMeasures struct {
	Defaults semanticmodel.MeasureDefaults
	Items    map[string]semanticmodel.MetricMeasure
}

type dashboardSpec struct {
	SemanticModel string                             `yaml:"semanticModel"`
	Filters       map[string]report.FilterDefinition `yaml:"filters"`
	Visuals       map[string]report.Visual           `yaml:"visuals"`
	Tables        map[string]report.TableVisual      `yaml:"tables"`
	Pages         []dashboard.Page                   `yaml:"pages"`
}

func CompileProject(projectPath string, opts Options) (CompiledProject, error) {
	project, err := LoadProject(projectPath)
	if err != nil {
		return CompiledProject{}, err
	}
	out := CompiledProject{Project: project, Workspaces: map[string]CompiledWorkspace{}}
	for id, workspaceProject := range project.Workspaces {
		definition, err := workspaceProject.definition(project)
		if err != nil {
			return CompiledProject{}, err
		}
		deploymentID := opts.DeploymentID
		workspaceID := workspace.WorkspaceID(id)
		graph, err := ExtractLineage(workspaceID, deploymentID, definition)
		if err != nil {
			return CompiledProject{}, err
		}
		out.Workspaces[id] = CompiledWorkspace{
			Workspace: workspace.Workspace{
				ID:          workspaceID,
				Title:       workspaceProject.Title,
				Description: workspaceProject.Description,
				BaseDir:     project.BaseDir,
				Graph:       graph,
			},
			Definition: definition,
		}
	}
	return out, nil
}

func PlanProject(projectPath string) (ProjectPlan, error) {
	project, err := LoadProject(projectPath)
	if err != nil {
		return ProjectPlan{}, err
	}
	plan := ProjectPlan{Project: project.Name}
	workspaceIDs := sortedMapKeys(project.Workspaces)
	for _, workspaceID := range workspaceIDs {
		workspaceProject := project.Workspaces[workspaceID]
		connections := workspaceConnections(project, workspaceProject)
		plan.Workspaces = append(plan.Workspaces, ProjectPlanWorkspace{
			ID:             workspaceID,
			Connections:    sortedMapKeys(connections),
			Sources:        sortedSetKeys(workspaceProject.AllowedSources),
			ModelTables:    sortedMapKeys(workspaceProject.Models),
			SemanticModels: sortedMapKeys(workspaceProject.SemanticModels),
			Dashboards:     sortedMapKeys(workspaceProject.Dashboards),
		})
	}
	return plan, nil
}

func IsProjectConfigFile(path string) bool {
	return projectConfigFile(path)
}

func LoadProject(projectPath string) (Project, error) {
	envelope, err := readEnvelope(projectPath)
	if err != nil {
		return Project{}, err
	}
	if envelope.Kind != "Project" {
		return Project{}, fmt.Errorf("%s kind = %q, want Project", projectPath, envelope.Kind)
	}
	var spec projectResource
	if err := envelope.Spec.Decode(&spec); err != nil {
		return Project{}, fmt.Errorf("%s spec: %w", projectPath, err)
	}
	baseDir := filepath.Dir(projectPath)
	project := Project{
		Name:        envelope.Metadata.Name,
		BaseDir:     baseDir,
		Connections: map[string]semanticmodel.Connection{},
		Sources:     map[string]semanticmodel.Source{},
		Workspaces:  map[string]*WorkspaceProject{},
	}
	if err := loadConnections(&project, spec.Connections.Include); err != nil {
		return Project{}, err
	}
	if err := loadSources(&project, spec.Sources.Include); err != nil {
		return Project{}, err
	}
	if err := loadWorkspaces(&project, spec.Workspaces.Include); err != nil {
		return Project{}, err
	}
	return project, validateProject(project)
}

func loadConnections(project *Project, includes []string) error {
	paths, err := expandIncludes(project.BaseDir, includes)
	if err != nil {
		return err
	}
	for _, path := range paths {
		envelope, err := readEnvelope(path)
		if err != nil {
			return err
		}
		if envelope.Kind != "Connection" {
			return fmt.Errorf("%s kind = %q, want Connection", path, envelope.Kind)
		}
		var spec semanticmodel.Connection
		if err := envelope.Spec.Decode(&spec); err != nil {
			return fmt.Errorf("%s spec: %w", path, err)
		}
		name := envelope.Metadata.Name
		if name == "" {
			return fmt.Errorf("%s metadata.name is required", path)
		}
		if _, exists := project.Connections[name]; exists {
			return fmt.Errorf("duplicate Connection %q", name)
		}
		project.Connections[name] = spec
	}
	return nil
}

func loadSources(project *Project, includes []string) error {
	paths, err := expandIncludes(project.BaseDir, includes)
	if err != nil {
		return err
	}
	for _, path := range paths {
		envelope, err := readEnvelope(path)
		if err != nil {
			return err
		}
		if envelope.Kind != "Source" {
			return fmt.Errorf("%s kind = %q, want Source", path, envelope.Kind)
		}
		var spec sourceSpec
		if err := envelope.Spec.Decode(&spec); err != nil {
			return fmt.Errorf("%s spec: %w", path, err)
		}
		name := envelope.Metadata.Name
		if name == "" {
			return fmt.Errorf("%s metadata.name is required", path)
		}
		if _, exists := project.Sources[name]; exists {
			return fmt.Errorf("duplicate Source %q", name)
		}
		source := semanticmodel.Source{
			Format:      spec.Format,
			Description: firstNonEmpty(spec.Description, envelope.Metadata.Description),
			Path:        spec.Path,
			Connection:  spec.Connection,
			Object:      spec.Object,
			Options:     spec.Options,
			Fields:      map[string]semanticmodel.SourceField{},
		}
		for field, cfg := range spec.Fields {
			source.Fields[field] = semanticmodel.SourceField{Description: cfg.Description}
		}
		project.Sources[name] = source
	}
	return nil
}

func loadWorkspaces(project *Project, includes []string) error {
	paths, err := expandIncludes(project.BaseDir, includes)
	if err != nil {
		return err
	}
	for _, path := range paths {
		envelope, err := readEnvelope(path)
		if err != nil {
			return err
		}
		if envelope.Kind != "Workspace" {
			return fmt.Errorf("%s kind = %q, want Workspace", path, envelope.Kind)
		}
		var spec workspaceSpec
		if err := envelope.Spec.Decode(&spec); err != nil {
			return fmt.Errorf("%s spec: %w", path, err)
		}
		id := envelope.Metadata.Name
		if id == "" {
			return fmt.Errorf("%s metadata.name is required", path)
		}
		if _, exists := project.Workspaces[id]; exists {
			return fmt.Errorf("duplicate Workspace %q", id)
		}
		workspaceProject := &WorkspaceProject{
			ID:                    id,
			Title:                 firstNonEmpty(envelope.Metadata.Title, id),
			Description:           envelope.Metadata.Description,
			AllowedSources:        map[string]struct{}{},
			Models:                map[string]semanticmodel.Table{},
			SemanticModels:        map[string]projectSemanticModelSpec{},
			Dashboards:            map[string]*report.Dashboard{},
			ModelTitles:           map[string]string{},
			ModelDescriptions:     map[string]string{},
			DashboardTitles:       map[string]string{},
			DashboardDescriptions: map[string]string{},
			DashboardTags:         map[string][]string{},
		}
		for _, source := range spec.Uses.Sources {
			workspaceProject.AllowedSources[source] = struct{}{}
		}
		workspaceDir := filepath.Dir(path)
		if err := loadWorkspaceModels(workspaceProject, workspaceDir, spec.Models.Include); err != nil {
			return err
		}
		if err := loadWorkspaceSemanticModels(workspaceProject, workspaceDir, spec.SemanticModels.Include); err != nil {
			return err
		}
		if err := loadWorkspaceDashboards(workspaceProject, workspaceDir, spec.Dashboards.Include); err != nil {
			return err
		}
		project.Workspaces[id] = workspaceProject
	}
	return nil
}

func loadWorkspaceModels(workspaceProject *WorkspaceProject, baseDir string, includes []string) error {
	paths, err := expandIncludes(baseDir, includes)
	if err != nil {
		return err
	}
	for _, path := range paths {
		envelope, err := readEnvelope(path)
		if err != nil {
			return err
		}
		if envelope.Kind != "ModelTable" {
			return fmt.Errorf("%s kind = %q, want ModelTable", path, envelope.Kind)
		}
		if envelope.Metadata.Workspace != "" && envelope.Metadata.Workspace != workspaceProject.ID {
			return fmt.Errorf("%s workspace = %q, want %q", path, envelope.Metadata.Workspace, workspaceProject.ID)
		}
		var table semanticmodel.Table
		if err := envelope.Spec.Decode(&table); err != nil {
			return fmt.Errorf("%s spec: %w", path, err)
		}
		name := envelope.Metadata.Name
		if _, exists := workspaceProject.Models[name]; exists {
			return fmt.Errorf("duplicate ModelTable %q in workspace %q", name, workspaceProject.ID)
		}
		workspaceProject.Models[name] = table
		workspaceProject.ModelTitles[name] = envelope.Metadata.Title
		workspaceProject.ModelDescriptions[name] = envelope.Metadata.Description
	}
	return nil
}

func loadWorkspaceSemanticModels(workspaceProject *WorkspaceProject, baseDir string, includes []string) error {
	paths, err := expandIncludes(baseDir, includes)
	if err != nil {
		return err
	}
	for _, path := range paths {
		envelope, err := readEnvelope(path)
		if err != nil {
			return err
		}
		if envelope.Kind != "SemanticModel" {
			return fmt.Errorf("%s kind = %q, want SemanticModel", path, envelope.Kind)
		}
		if envelope.Metadata.Workspace != "" && envelope.Metadata.Workspace != workspaceProject.ID {
			return fmt.Errorf("%s workspace = %q, want %q", path, envelope.Metadata.Workspace, workspaceProject.ID)
		}
		var spec projectSemanticModelSpec
		if err := envelope.Spec.Decode(&spec); err != nil {
			return fmt.Errorf("%s spec: %w", path, err)
		}
		name := envelope.Metadata.Name
		if _, exists := workspaceProject.SemanticModels[name]; exists {
			return fmt.Errorf("duplicate SemanticModel %q in workspace %q", name, workspaceProject.ID)
		}
		workspaceProject.SemanticModels[name] = spec
		workspaceProject.ModelTitles[name] = envelope.Metadata.Title
		workspaceProject.ModelDescriptions[name] = envelope.Metadata.Description
	}
	return nil
}

func loadWorkspaceDashboards(workspaceProject *WorkspaceProject, baseDir string, includes []string) error {
	paths, err := expandIncludes(baseDir, includes)
	if err != nil {
		return err
	}
	for _, path := range paths {
		envelope, err := readEnvelope(path)
		if err != nil {
			return err
		}
		if envelope.Kind != "Dashboard" {
			return fmt.Errorf("%s kind = %q, want Dashboard", path, envelope.Kind)
		}
		if envelope.Metadata.Workspace != "" && envelope.Metadata.Workspace != workspaceProject.ID {
			return fmt.Errorf("%s workspace = %q, want %q", path, envelope.Metadata.Workspace, workspaceProject.ID)
		}
		var spec dashboardSpec
		if err := envelope.Spec.Decode(&spec); err != nil {
			return fmt.Errorf("%s spec: %w", path, err)
		}
		name := envelope.Metadata.Name
		if _, exists := workspaceProject.Dashboards[name]; exists {
			return fmt.Errorf("duplicate Dashboard %q in workspace %q", name, workspaceProject.ID)
		}
		dashboard := &report.Dashboard{
			ID:            name,
			Title:         envelope.Metadata.Title,
			Description:   envelope.Metadata.Description,
			SemanticModel: spec.SemanticModel,
			Filters:       spec.Filters,
			Visuals:       spec.Visuals,
			Tables:        spec.Tables,
			Pages:         spec.Pages,
		}
		workspaceProject.Dashboards[name] = dashboard
		workspaceProject.DashboardTitles[name] = envelope.Metadata.Title
		workspaceProject.DashboardDescriptions[name] = envelope.Metadata.Description
		workspaceProject.DashboardTags[name] = append([]string{}, envelope.Metadata.Tags...)
	}
	return nil
}

func validateProject(project Project) error {
	for sourceName, source := range project.Sources {
		if _, ok := project.Connections[source.Connection]; !ok {
			return fmt.Errorf("Source %q references unknown Connection %q", sourceName, source.Connection)
		}
	}
	for _, workspaceProject := range project.Workspaces {
		for source := range workspaceProject.AllowedSources {
			if _, ok := project.Sources[source]; !ok {
				return fmt.Errorf("Workspace %q allows unknown Source %q", workspaceProject.ID, source)
			}
		}
		if len(workspaceProject.SemanticModels) == 0 {
			return fmt.Errorf("Workspace %q requires SemanticModel resources", workspaceProject.ID)
		}
		for tableName, table := range workspaceProject.Models {
			for _, source := range table.Sources {
				if _, ok := workspaceProject.AllowedSources[source]; !ok {
					return fmt.Errorf("ModelTable %q.%q reads source %q outside uses.sources", workspaceProject.ID, tableName, source)
				}
			}
			if table.Source != "" {
				if _, ok := workspaceProject.AllowedSources[table.Source]; !ok {
					return fmt.Errorf("ModelTable %q.%q reads source %q outside uses.sources", workspaceProject.ID, tableName, table.Source)
				}
			}
			if err := validateProjectTableSources(workspaceProject.ID, tableName, table); err != nil {
				return err
			}
		}
		for name, dashboard := range workspaceProject.Dashboards {
			if _, ok := workspaceProject.SemanticModels[dashboard.SemanticModel]; !ok {
				return fmt.Errorf("Dashboard %q.%q references unknown SemanticModel %q", workspaceProject.ID, name, dashboard.SemanticModel)
			}
		}
	}
	return nil
}

func validateProjectTableSources(workspaceID, tableName string, table semanticmodel.Table) error {
	sql := strings.TrimSpace(table.Transform.SQL)
	if sql == "" {
		sql = strings.TrimSpace(table.SQL)
	}
	if sql == "" {
		return nil
	}
	declared := append([]string{}, table.Sources...)
	if table.Source != "" {
		declared = append(declared, table.Source)
	}
	sort.Strings(declared)
	inferred, rawRefs, unqualifiedRefs := (&semanticmodel.Model{}).SQLSourceRefs(sql)
	if len(rawRefs) > 0 {
		return fmt.Errorf("ModelTable %q.%q SQL must reference sources through source.<name>; raw.<name> is internal", workspaceID, tableName)
	}
	if len(unqualifiedRefs) > 0 {
		return fmt.Errorf("ModelTable %q.%q SQL must reference sources through source.<name>; found unqualified relation %q", workspaceID, tableName, unqualifiedRefs[0])
	}
	if !sameStringList(declared, inferred) {
		return fmt.Errorf("ModelTable %q.%q SQL source references %v do not match declared sources %v", workspaceID, tableName, inferred, declared)
	}
	return nil
}

func (workspaceProject *WorkspaceProject) definition(project Project) (*workspace.Definition, error) {
	if len(workspaceProject.SemanticModels) != 1 {
		return nil, fmt.Errorf("workspace %q requires exactly one SemanticModel for runtime compilation", workspaceProject.ID)
	}
	var modelName string
	var semanticSpec projectSemanticModelSpec
	for name, spec := range workspaceProject.SemanticModels {
		modelName = name
		semanticSpec = spec
	}
	model := &semanticmodel.Model{
		Name:          modelName,
		Title:         firstNonEmpty(workspaceProject.ModelTitles[modelName], modelName),
		Description:   workspaceProject.ModelDescriptions[modelName],
		Connections:   workspaceConnections(project, workspaceProject),
		Sources:       map[string]semanticmodel.Source{},
		Tables:        copyTables(workspaceProject.Models),
		BaseTable:     semanticSpec.BaseTable,
		Relationships: append([]semanticmodel.Relationship{}, semanticSpec.Relationships...),
		Measures:      map[string]semanticmodel.MetricMeasure{},
	}
	model.DefaultConnection = firstConnectionName(model.Connections)
	sourceAliases := map[string]string{}
	sourceIDs := map[string]string{}
	for source := range workspaceProject.AllowedSources {
		alias := localSourceName(source)
		if existing, exists := sourceIDs[alias]; exists && existing != source {
			return nil, fmt.Errorf("workspace %q sources %q and %q resolve to duplicate runtime alias %q", workspaceProject.ID, existing, source, alias)
		}
		sourceAliases[source] = alias
		sourceIDs[alias] = source
		model.Sources[alias] = project.Sources[source]
	}
	model.Tables = translatedTablesForRuntime(model.Tables, sourceAliases)
	if err := applySemanticModelSpec(model, semanticSpec); err != nil {
		return nil, err
	}
	if err := model.Validate(); err != nil {
		return nil, err
	}
	if _, err := analyticsmaterialize.ModelTableOrder(model); err != nil {
		return nil, err
	}
	catalog := workspace.Catalog{
		Workspace: workspace.CatalogWorkspace{
			ID:          workspaceProject.ID,
			Title:       workspaceProject.Title,
			Description: workspaceProject.Description,
		},
		SemanticModels: []workspace.CatalogModel{{
			ID:          modelName,
			Title:       model.Title,
			Description: model.Description,
		}},
		Dashboards: []workspace.CatalogDashboard{},
	}
	definition := &workspace.Definition{
		Catalog:    catalog,
		Models:     map[string]*semanticmodel.Model{modelName: model},
		Dashboards: workspaceProject.Dashboards,
		BaseDir:    project.BaseDir,
		SourceIDs:  sourceIDs,
	}
	for name := range workspaceProject.Dashboards {
		dashboard := workspaceProject.Dashboards[name]
		if err := ValidateDashboard(dashboard, definition.Models); err != nil {
			return nil, fmt.Errorf("loading dashboard %q: %w", name, err)
		}
		definition.Catalog.Dashboards = append(definition.Catalog.Dashboards, workspace.CatalogDashboard{
			ID:          name,
			Title:       firstNonEmpty(workspaceProject.DashboardTitles[name], dashboard.Title),
			Description: workspaceProject.DashboardDescriptions[name],
			Tags:        append([]string{}, workspaceProject.DashboardTags[name]...),
		})
	}
	sort.Slice(definition.Catalog.Dashboards, func(i, j int) bool {
		return definition.Catalog.Dashboards[i].ID < definition.Catalog.Dashboards[j].ID
	})
	return definition, nil
}

func translatedTablesForRuntime(in map[string]semanticmodel.Table, sourceAliases map[string]string) map[string]semanticmodel.Table {
	out := make(map[string]semanticmodel.Table, len(in))
	for name, table := range in {
		if alias, ok := sourceAliases[table.Source]; ok {
			table.Source = alias
		}
		for index, source := range table.Sources {
			if alias, ok := sourceAliases[source]; ok {
				table.Sources[index] = alias
			}
		}
		table.SQL = rewriteSourceSQLForRuntime(table.SQL, sourceAliases)
		table.Transform.SQL = rewriteSourceSQLForRuntime(table.Transform.SQL, sourceAliases)
		out[name] = table
	}
	return out
}

func rewriteSourceSQLForRuntime(sql string, sourceAliases map[string]string) string {
	for global, local := range sourceAliases {
		sql = strings.ReplaceAll(sql, `source."`+global+`"`, "source."+local)
		sql = strings.ReplaceAll(sql, "source."+global, "source."+local)
	}
	return sql
}

func localSourceName(sourceID string) string {
	var builder strings.Builder
	for index, char := range sourceID {
		valid := char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || index > 0 && char >= '0' && char <= '9'
		if valid {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	out := builder.String()
	if out == "" || out[0] >= '0' && out[0] <= '9' {
		out = "source_" + out
	}
	return out
}

func workspaceConnections(project Project, workspaceProject *WorkspaceProject) map[string]semanticmodel.Connection {
	out := map[string]semanticmodel.Connection{}
	for sourceID := range workspaceProject.AllowedSources {
		source, ok := project.Sources[sourceID]
		if !ok {
			continue
		}
		connection, ok := project.Connections[source.Connection]
		if !ok {
			continue
		}
		out[source.Connection] = connection
	}
	return out
}

func applySemanticModelSpec(model *semanticmodel.Model, spec projectSemanticModelSpec) error {
	if spec.BaseTable == "" {
		return fmt.Errorf("SemanticModel %q requires baseTable", model.Name)
	}
	if len(spec.Tables) == 0 {
		return fmt.Errorf("SemanticModel %q requires tables", model.Name)
	}
	tables := map[string]semanticmodel.Table{}
	for _, tableName := range spec.Tables {
		table, ok := model.Tables[tableName]
		if !ok {
			return fmt.Errorf("SemanticModel %q references unknown ModelTable %q", model.Name, tableName)
		}
		tables[tableName] = table
	}
	defaults := spec.Measures.Defaults
	measures := map[string]semanticmodel.MetricMeasure{}
	for name, measure := range spec.Measures.Items {
		if measure.Expression == "" {
			measure.Expression = measure.Expr
		}
		measure.Table = firstNonEmpty(measure.Table, defaults.Table)
		measure.Grain = firstNonEmpty(measure.Grain, defaults.Grain)
		measure.Time = firstNonEmpty(measure.Time, defaults.Time)
		if len(measure.Grains) == 0 {
			measure.Grains = append([]string{}, defaults.Grains...)
		}
		measure.Field = name
		measure.Name = name
		measures[name] = measure
	}
	model.Tables = tables
	model.BaseTable = spec.BaseTable
	model.Relationships = append([]semanticmodel.Relationship{}, spec.Relationships...)
	model.Measures = measures
	return nil
}

func (m *projectSemanticModelMeasures) UnmarshalYAML(value *yaml.Node) error {
	m.Items = map[string]semanticmodel.MetricMeasure{}
	if value == nil || value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("semantic model measures must be a mapping")
	}
	for index := 0; index+1 < len(value.Content); index += 2 {
		key := value.Content[index].Value
		item := value.Content[index+1]
		if key == "defaults" {
			if err := item.Decode(&m.Defaults); err != nil {
				return err
			}
			continue
		}
		var measure semanticmodel.MetricMeasure
		if item.Kind != yaml.ScalarNode || item.Tag != "!!null" {
			if err := item.Decode(&measure); err != nil {
				return err
			}
		}
		m.Items[key] = measure
	}
	return nil
}

func readEnvelope(path string) (resourceEnvelope, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return resourceEnvelope{}, err
	}
	if kind, ok := schemaKindForEnvelope(content); ok {
		if err := configschema.ValidateBytes(kind, path, content); err != nil {
			return resourceEnvelope{}, err
		}
	}
	var envelope resourceEnvelope
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&envelope); err != nil {
		return resourceEnvelope{}, fmt.Errorf("%s: %w", path, err)
	}
	if envelope.APIVersion != projectAPIVersion {
		return resourceEnvelope{}, fmt.Errorf("%s apiVersion = %q, want %q", path, envelope.APIVersion, projectAPIVersion)
	}
	if envelope.Kind == "" {
		return resourceEnvelope{}, fmt.Errorf("%s kind is required", path)
	}
	return envelope, nil
}

func schemaKindForEnvelope(content []byte) (configschema.Kind, bool) {
	var header struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(content, &header); err != nil {
		return "", false
	}
	if header.APIVersion != projectAPIVersion {
		return "", false
	}
	switch header.Kind {
	case "Project":
		return configschema.KindProject, true
	case "Connection":
		return configschema.KindConnection, true
	case "Source":
		return configschema.KindSource, true
	case "Workspace":
		return configschema.KindWorkspace, true
	case "ModelTable":
		return configschema.KindModelTable, true
	case "SemanticModel":
		return configschema.KindSemanticModelResource, true
	case "Dashboard":
		return configschema.KindDashboardResource, true
	default:
		return "", false
	}
}

func projectConfigFile(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var envelope struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(content, &envelope); err != nil {
		return false
	}
	return envelope.APIVersion == projectAPIVersion && envelope.Kind == "Project"
}

func expandIncludes(baseDir string, includes []string) ([]string, error) {
	var paths []string
	for _, pattern := range includes {
		if strings.TrimSpace(pattern) == "" {
			return nil, fmt.Errorf("include pattern is required")
		}
		if filepath.IsAbs(pattern) {
			return nil, fmt.Errorf("include pattern %q must be relative", pattern)
		}
		if strings.Contains(filepath.ToSlash(pattern), "**") {
			return nil, fmt.Errorf("include pattern %q uses unsupported ** glob", pattern)
		}
		for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(pattern)), "/") {
			if part == ".." {
				return nil, fmt.Errorf("include pattern %q escapes project boundary", pattern)
			}
		}
		matches, err := filepath.Glob(filepath.Join(baseDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("include pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("include pattern %q matched no files", pattern)
		}
		sort.Strings(matches)
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, err
			}
			if info.IsDir() {
				return nil, fmt.Errorf("include pattern %q matched directory %s", pattern, match)
			}
			ext := strings.ToLower(filepath.Ext(match))
			if ext != ".yaml" && ext != ".yml" {
				return nil, fmt.Errorf("include pattern %q matched non-YAML file %s", pattern, match)
			}
		}
		paths = append(paths, matches...)
	}
	return paths, nil
}

func firstConnectionName(connections map[string]semanticmodel.Connection) string {
	names := make([]string, 0, len(connections))
	for name := range connections {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func copyConnections(in map[string]semanticmodel.Connection) map[string]semanticmodel.Connection {
	out := make(map[string]semanticmodel.Connection, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func copyTables(in map[string]semanticmodel.Table) map[string]semanticmodel.Table {
	out := make(map[string]semanticmodel.Table, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sameStringList(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
