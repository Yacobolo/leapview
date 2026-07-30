package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/app/site/visualdocs"
	"github.com/flidai/leapview/internal/dashboard"
	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

func TestParseVisualExamplesUsesMarkedYAMLAsSource(t *testing.T) {
	t.Parallel()

	source := []byte("" +
		"# Line chart\n\n" +
		"## Basic\n\n" +
		"{{< visual id=\"line_basic\" >}}\n\n" +
		"```yaml visual-example=line_basic\n" +
		"visuals:\n" +
		"  line_basic:\n" +
		"    title: Revenue\n" +
		"    type: line\n" +
		"    query:\n" +
		"      dimensions:\n" +
		"        month: orders.month\n" +
		"      measures:\n" +
		"        revenue: null\n" +
		"```\n")

	examples, err := parseVisualExamples("line.md", source)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(examples), 1; got != want {
		t.Fatalf("examples = %d, want %d", got, want)
	}
	example := examples[0]
	if example.ID != "line_basic" || example.Chart == nil || example.Chart.Type != "line" {
		t.Fatalf("example = %#v", example)
	}
	if got := example.Chart.Query.Dimensions[0].Field; got != "orders.month" {
		t.Fatalf("dimension = %q, want orders.month", got)
	}
}

func TestGenerateVisualExamplesExecutesEveryDocumentedQuery(t *testing.T) {
	docsDir := filepath.Join("..", "..", "..", "..", "docs", "visuals")
	artifact, err := generateVisualExamples(docsDir, filepath.Join("testdata", "project", "leapview.yaml"), filepath.Join("testdata", "data"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := artifact.Version, visualdocs.ArtifactVersion; got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	lineReference := artifact.References["visuals/line"]
	if got, want := lineReference.Kind, "chart"; got != want {
		t.Fatalf("line reference kind = %q, want %q", got, want)
	}
	if got, want := strings.Join(lineReference.Shapes, ","), "category_series_value,category_value"; got != want {
		t.Fatalf("line reference shapes = %q, want %q", got, want)
	}
	if got := strings.Join(lineReference.Examples["revenue_line_step"].KeyFields, ","); !strings.Contains(got, "presentation.step") || strings.Contains(got, "query.series") {
		t.Fatalf("stepped line key fields = %q", got)
	}
	fields := make(map[string]visualdocs.FieldReference, len(lineReference.Fields))
	for _, field := range lineReference.Fields {
		fields[field.Path] = field
	}
	if got, want := fields["query.dimensions"].Type, "field mapping"; got != want {
		t.Fatalf("query.dimensions type = %q, want %q", got, want)
	}
	if got, want := fields["query.limit"].Default, "no limit"; got != want {
		t.Fatalf("query.limit default = %q, want %q", got, want)
	}
	if got, want := fields["presentation.step"].Type, "boolean"; got != want {
		t.Fatalf("presentation.step type = %q, want %q", got, want)
	}
	if got, want := strings.Join(fields["presentation.step"].AllowedValues, ","), "true,false"; got != want {
		t.Fatalf("presentation.step values = %q, want %q", got, want)
	}
	if fields["presentation.step"].Description == "" {
		t.Fatal("presentation.step description is empty")
	}
	if got := artifact.References["visuals/map"].Accessibility; !strings.Contains(got, "coordinate fields") {
		t.Fatalf("map accessibility guidance = %q", got)
	}
	if got := artifact.References["visuals/kpi"].Accessibility; !strings.Contains(got, "tone as the only") {
		t.Fatalf("KPI accessibility guidance = %q", got)
	}
	if got, want := len(artifact.Documents), 27; got != want {
		t.Fatalf("documents = %d, want %d", got, want)
	}
	count := 0
	for slug, examples := range artifact.Documents {
		if len(examples) == 0 {
			t.Fatalf("%s has no examples", slug)
		}
		for _, example := range examples {
			count++
			if visualizationEnvelopeRowCount(example) == 0 {
				t.Fatalf("%s/%s has no query data", slug, example.VisualID)
			}
			base, err := visualizationir.SpecificationBase(example.Spec)
			if err != nil {
				t.Fatalf("%s/%s specification base: %v", slug, example.VisualID, err)
			}
			if len(base.Interactions) != 0 {
				t.Fatalf("%s/%s has %d runtime interactions; visual documentation previews must be render-only", slug, example.VisualID, len(base.Interactions))
			}
		}
	}
	if got, want := count, 76; got != want {
		t.Fatalf("examples = %d, want %d", got, want)
	}
	if got, want := len(artifact.Showcase), 27; got != want {
		t.Fatalf("showcase examples = %d, want %d", got, want)
	}
	line := artifact.Documents["visuals/line"]
	seriesSpec, ok := line[1].Spec.Value.(*visualizationir.CartesianVisualizationSpec)
	if !ok || seriesSpec.Series == nil {
		t.Fatalf("series line spec = %#v", line[1].Spec.Value)
	}
	stepSpec, ok := line[2].Spec.Value.(*visualizationir.CartesianVisualizationSpec)
	if !ok || !stepSpec.Presentation.Step {
		t.Fatalf("stepped line presentation was not compiled: %#v", line[2].Spec.Value)
	}
	barRows := envelopeRows(artifact.Documents["visuals/bar"][0])
	for index := 1; index < len(barRows); index++ {
		previous, previousOK := barRows[index-1]["value"].(float64)
		current, currentOK := barRows[index]["value"].(float64)
		if !previousOK || !currentOK {
			t.Fatalf("ranked bar values are not numeric: %#v", barRows)
		}
		if previous < current {
			t.Fatalf("ranked bar query order was not preserved at row %d: %#v", index, barRows)
		}
	}
	histogramRows := envelopeRows(artifact.Documents["visuals/histogram"][0])
	for index := 1; index < len(histogramRows); index++ {
		previous, previousOK := histogramRows[index-1]["binStart"].(float64)
		current, currentOK := histogramRows[index]["binStart"].(float64)
		if !previousOK || !currentOK {
			t.Fatalf("histogram bin starts are not numeric: %#v", histogramRows)
		}
		if previous > current {
			t.Fatalf("histogram numeric order was not preserved at row %d: %#v", index, histogramRows)
		}
	}
	donut := artifact.Documents["visuals/donut"]
	basicDonut, ok := donut[0].Spec.Value.(*visualizationir.ProportionalVisualizationSpec)
	if !ok {
		t.Fatalf("basic donut spec = %#v", donut[0].Spec.Value)
	}
	revenueDonut, ok := donut[1].Spec.Value.(*visualizationir.ProportionalVisualizationSpec)
	if !ok {
		t.Fatalf("revenue donut spec = %#v", donut[1].Spec.Value)
	}
	if got, want := visualFieldSourceRef(basicDonut.VisualizationSpecBase, basicDonut.Value), "order_count"; got != want {
		t.Fatalf("basic donut value source = %q, want %q", got, want)
	}
	if got, want := visualFieldSourceRef(revenueDonut.VisualizationSpecBase, revenueDonut.Value), "revenue"; got != want {
		t.Fatalf("revenue donut value source = %q, want %q", got, want)
	}
	if !revenueDonut.Presentation.ShowLabels {
		t.Fatal("revenue donut must expose its distinct currency measure with labels")
	}
	scatter := artifact.Documents["visuals/scatter"]
	basicScatter, ok := scatter[0].Spec.Value.(*visualizationir.CartesianVisualizationSpec)
	if !ok {
		t.Fatalf("basic scatter spec = %#v", scatter[0].Spec.Value)
	}
	if got, want := visualFieldSourceRef(basicScatter.VisualizationSpecBase, basicScatter.X), "orders.delivery_bucket"; got != want {
		t.Fatalf("basic scatter x source = %q, want %q", got, want)
	}
	if got, want := visualFieldSourceRef(basicScatter.VisualizationSpecBase, basicScatter.Y[0]), "review_score"; got != want {
		t.Fatalf("basic scatter y source = %q, want %q", got, want)
	}
	scatterRows := envelopeRows(scatter[0])
	for index := 1; index < len(scatterRows); index++ {
		previous, previousOK := documentationNumber(scatterRows[index-1]["value"])
		current, currentOK := documentationNumber(scatterRows[index]["value"])
		if !previousOK || !currentOK || previous < current {
			t.Fatalf("basic scatter must rank delivery bands by review score: %#v", scatterRows)
		}
	}
	funnelRows := envelopeRows(artifact.Documents["visuals/funnel"][0])
	wantStages := []string{"Order placed", "Payment confirmed", "Shipped", "Delivered"}
	for index, want := range wantStages {
		if index >= len(funnelRows) || funnelRows[index]["label"] != want {
			t.Fatalf("funnel stages = %#v, want %q at row %d", funnelRows, want, index)
		}
		if index > 0 {
			previous, previousOK := documentationNumber(funnelRows[index-1]["value"])
			current, currentOK := documentationNumber(funnelRows[index]["value"])
			if !previousOK || !currentOK || previous <= current {
				t.Fatalf("funnel values must strictly decrease by stage: %#v", funnelRows)
			}
		}
	}
	areaRows := envelopeRows(artifact.Documents["visuals/area"][1])
	quarterSeries := map[any]map[any]struct{}{}
	for _, row := range areaRows {
		if quarterSeries[row["label"]] == nil {
			quarterSeries[row["label"]] = map[any]struct{}{}
		}
		quarterSeries[row["label"]][row["series"]] = struct{}{}
	}
	if len(quarterSeries) != 4 {
		t.Fatalf("stacked area quarters = %#v, want four complete quarters", quarterSeries)
	}
	for quarter, series := range quarterSeries {
		if len(series) != 4 {
			t.Fatalf("stacked area quarter %v has %d series, want 4: %#v", quarter, len(series), areaRows)
		}
	}
	treeRows := envelopeRows(artifact.Documents["visuals/tree"][0])
	roots := 0
	children := 0
	for _, row := range treeRows {
		if row["parent"] == nil {
			roots++
		} else {
			children++
		}
	}
	if roots < 3 || roots > 8 || children < roots {
		t.Fatalf("basic tree must be a compact branching hierarchy, got roots=%d children=%d rows=%#v", roots, children, treeRows)
	}
	basicTree, ok := artifact.Documents["visuals/tree"][0].Spec.Value.(*visualizationir.HierarchyVisualizationSpec)
	if !ok || !basicTree.Presentation.ShowLabels {
		t.Fatalf("basic tree must label its category and status nodes: %#v", artifact.Documents["visuals/tree"][0].Spec.Value)
	}
	deepTree, ok := artifact.Documents["visuals/tree"][2].Spec.Value.(*visualizationir.HierarchyVisualizationSpec)
	if !ok || deepTree.Presentation.InitialDepth == nil || *deepTree.Presentation.InitialDepth != 0 {
		t.Fatalf("three-level tree must start at category depth and expand on demand: %#v", artifact.Documents["visuals/tree"][2].Spec.Value)
	}
	for _, example := range artifact.Documents["visuals/sankey"] {
		spec, ok := example.Spec.Value.(*visualizationir.HierarchyVisualizationSpec)
		if !ok || !spec.Presentation.ShowLabels {
			t.Fatalf("sankey example %q must label both sides of its flow: %#v", example.VisualID, example.Spec.Value)
		}
	}
	mapExamples := artifact.Documents["visuals/map"]
	if got, want := len(mapExamples), 6; got != want {
		t.Fatalf("map examples = %d, want %d render-only layer examples", got, want)
	}
	first, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	regenerated, err := generateVisualExamples(docsDir, filepath.Join("testdata", "project", "leapview.yaml"), filepath.Join("testdata", "data"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(regenerated)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("artifact JSON is not deterministic")
	}
}

func TestVisualDocumentationCoversEveryPublicTypeAndGeographicLayer(t *testing.T) {
	docsDir := filepath.Join("..", "..", "..", "..", "docs", "visuals")
	schemaPath := filepath.Join("..", "..", "..", "..", "schemas", "json", "dashboard.schema.json")
	publicTypes, publicGeographicLayers := publicVisualizationDiscriminators(t, schemaPath)
	catalogContents, err := os.ReadFile(filepath.Join(docsDir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog visualCatalog
	if err := json.Unmarshal(catalogContents, &catalog); err != nil {
		t.Fatal(err)
	}
	documentedTypes := map[string]bool{}
	documentedGeographicLayers := map[string]bool{}
	for _, document := range catalog.Documents {
		contents, err := os.ReadFile(filepath.Join(docsDir, document.Source+".md"))
		if err != nil {
			t.Fatal(err)
		}
		examples, err := parseVisualExamples(document.Source+".md", contents)
		if err != nil {
			t.Fatal(err)
		}
		for _, example := range examples {
			if example.Tabular != nil {
				documentedTypes[example.Type] = true
				continue
			}
			if example.Chart == nil {
				continue
			}
			documentedTypes[example.Chart.Type] = true
			for _, layer := range example.Chart.Geo.Layers {
				documentedGeographicLayers[layer.Kind] = true
			}
		}
	}
	if got, want := strings.Join(publicTypes, ","), strings.Join(reportdef.SupportedVisualizationTypes(), ","); got != want {
		t.Fatalf("runtime visualization types = %q, public schema = %q", want, got)
	}
	if got, want := strings.Join(publicGeographicLayers, ","), strings.Join(reportdef.SupportedGeographicLayerKinds(), ","); got != want {
		t.Fatalf("runtime geographic layer kinds = %q, public schema = %q", want, got)
	}
	for _, visualType := range publicTypes {
		if !documentedTypes[visualType] {
			t.Errorf("public visualization type %q has no executable documentation example", visualType)
		}
	}
	for _, kind := range publicGeographicLayers {
		if !documentedGeographicLayers[kind] {
			t.Errorf("public geographic layer kind %q has no executable documentation example", kind)
		}
	}
}

func publicVisualizationDiscriminators(t *testing.T, schemaPath string) ([]string, []string) {
	t.Helper()
	type schemaNode struct {
		Ref        string                `json:"$ref"`
		Const      any                   `json:"const"`
		Enum       []string              `json:"enum"`
		AnyOf      []schemaNode          `json:"anyOf"`
		Properties map[string]schemaNode `json:"properties"`
	}
	var schema struct {
		Definitions map[string]schemaNode `json:"$defs"`
	}
	contents, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(contents, &schema); err != nil {
		t.Fatal(err)
	}
	values := func(node schemaNode) []string {
		out := append([]string{}, node.Enum...)
		if value, ok := node.Const.(string); ok && value != "" {
			out = append(out, value)
		}
		for _, candidate := range node.AnyOf {
			if value, ok := candidate.Const.(string); ok && value != "" {
				out = append(out, value)
			}
		}
		return out
	}
	types := []string{}
	for _, variant := range schema.Definitions["#Visual"].AnyOf {
		name := strings.TrimPrefix(variant.Ref, "#/$defs/")
		name = strings.ReplaceAll(name, "%23", "#")
		types = append(types, values(schema.Definitions[name].Properties["type"])...)
	}
	layers := []string{}
	for _, variant := range schema.Definitions["#GeographicLayer"].AnyOf {
		layers = append(layers, values(variant.Properties["kind"])...)
	}
	slices.Sort(types)
	slices.Sort(layers)
	return types, layers
}

func visualizationEnvelopeRowCount(envelope visualizationir.VisualizationEnvelope) int {
	switch state := envelope.DataState.Value.(type) {
	case *visualizationir.InlineVisualizationDataState:
		count := 0
		for _, dataset := range state.Datasets {
			count += len(dataset.Rows)
		}
		return count
	case *visualizationir.WindowedVisualizationDataState:
		count := 0
		for _, block := range state.Blocks {
			count += len(block.Rows)
		}
		return count
	default:
		return 0
	}
}

func TestValidateVisualPayloadRejectsInvalidGeneratedData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		visual  visualExample
		payload []dashboard.Datum
		want    string
	}{
		{
			name:    "non finite metric",
			visual:  visualExample{ID: "bad_number", Chart: reportVisualPointer("category_value", "line", nil)},
			payload: []dashboard.Datum{{"label": "Jan", "value": math.NaN()}},
			want:    `non-finite number at data[0].value`,
		},
		{
			name:    "unknown map region",
			visual:  visualExample{ID: "bad_map", Chart: reportVisualPointer("geo", "map", map[string]any{"map": "brazil_states"})},
			payload: []dashboard.Datum{{"name": "CA", "value": 2.0}},
			want:    `region "CA" is not defined by map "brazil_states"`,
		},
		{
			name:    "incomplete map coverage",
			visual:  visualExample{ID: "incomplete_map", Chart: reportVisualPointer("geo", "map", map[string]any{"map": "brazil_states"})},
			payload: []dashboard.Datum{{"name": "SP", "value": 2.0}},
			want:    `does not provide data for map region`,
		},
		{
			name:    "no numeric values",
			visual:  visualExample{ID: "empty_series", Chart: reportVisualPointer("category_value", "line", nil)},
			payload: []dashboard.Datum{{"label": "Jan"}},
			want:    `has no finite numeric values`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVisualData(tt.visual, tt.payload)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func reportVisual(shape, visualType string, options map[string]any) reportdef.Visual {
	value := reportdef.Visual{Type: visualType}
	if mapID, ok := options["map"].(string); ok {
		value.Geo.Layers = []reportdef.VisualGeoLayer{{ID: "regions", Kind: "choropleth", GeometryAsset: mapID, Join: "name", Value: "value"}}
	}
	return value
}

func reportVisualPointer(shape, visualType string, options map[string]any) *reportdef.Visual {
	value := reportVisual(shape, visualType, options)
	return &value
}

func TestPersistVisualExamplesCheckDetectsStaleArtifact(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "examples.gen.json")
	artifact := visualExamplesArtifact{Version: 2, Documents: map[string][]visualdocs.Payload{}, Showcase: []visualdocs.Payload{}}
	if err := persistVisualExamples(path, artifact, false); err != nil {
		t.Fatal(err)
	}
	if err := persistVisualExamples(path, artifact, true); err != nil {
		t.Fatalf("current artifact: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := persistVisualExamples(path, artifact, true); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("stale artifact error = %v", err)
	}
}

func TestVisualDocumentationUsesPatternHeadingsAndSpecificGuidance(t *testing.T) {
	t.Parallel()
	docsDir := filepath.Join("..", "..", "..", "..", "docs", "visuals")
	files, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Base(file) == "index.md" {
			continue
		}
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(contents)
		for _, boilerplate := range []string{
			"Start with the default presentation and keep the query focused",
			"to create this variation while leaving the renderer contract unchanged",
		} {
			if strings.Contains(source, boilerplate) {
				t.Errorf("%s contains generic variation guidance %q", file, boilerplate)
			}
		}
		headings := map[string]struct{}{}
		for _, line := range strings.Split(source, "\n") {
			if strings.HasPrefix(line, "## ") {
				heading := strings.TrimPrefix(line, "## ")
				headings[heading] = struct{}{}
				if strings.HasPrefix(strings.ToLower(heading), "alternate") {
					t.Errorf("%s uses generic variation heading %q", file, heading)
				}
			}
		}
		for _, title := range regexp.MustCompile(`(?m)^    title: (.+)$`).FindAllStringSubmatch(source, -1) {
			if _, duplicate := headings[title[1]]; duplicate {
				t.Errorf("%s repeats rendered visual title %q as a variation heading", file, title[1])
			}
		}
	}
}

func visualFieldSourceRef(spec visualizationir.VisualizationSpecBase, ref visualizationir.VisualizationFieldRef) string {
	for _, dataset := range spec.Datasets {
		if dataset.ID != ref.Dataset {
			continue
		}
		for _, field := range dataset.Fields {
			if field.ID == ref.Field && field.SourceRef != nil {
				return *field.SourceRef
			}
		}
	}
	return ""
}

func documentationNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func TestParseVisualExamplesRejectsBrokenContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing fence",
			body: `{{< visual id="line_basic" >}}`,
			want: `shortcode "line_basic" has no matching visual example`,
		},
		{
			name: "missing shortcode",
			body: "```yaml visual-example=line_basic\nvisuals:\n  line_basic:\n    title: Line\n    type: line\n    query:\n      dimensions: [orders.month]\n      measures: [revenue]\n```",
			want: `visual example "line_basic" has no matching shortcode`,
		},
		{
			name: "multiple visuals",
			body: "{{< visual id=\"line_basic\" >}}\n```yaml visual-example=line_basic\nvisuals:\n  line_basic: {type: line}\n  other: {type: line}\n```",
			want: `must contain exactly one visual`,
		},
		{
			name: "key mismatch",
			body: "{{< visual id=\"line_basic\" >}}\n```yaml visual-example=line_basic\nvisuals:\n  other: {type: line}\n```",
			want: `must use visual key "line_basic"`,
		},
		{
			name: "duplicate shortcode",
			body: "{{< visual id=\"line_basic\" >}}\n{{< visual id=\"line_basic\" >}}\n```yaml visual-example=line_basic\nvisuals:\n  line_basic: {type: line}\n```",
			want: `duplicate visual shortcode "line_basic"`,
		},
		{
			name: "missing type",
			body: "{{< visual id=\"total\" >}}\n```yaml visual-example=total\nvisuals:\n  total:\n    shape: single_value\n    query:\n      measures: [revenue]\n```",
			want: `type`,
		},
		{
			name: "legacy kind",
			body: "{{< visual id=\"total\" >}}\n```yaml visual-example=total\nvisuals:\n  total:\n    kind: kpi\n    shape: single_value\n    query:\n      measures: [revenue]\n```",
			want: `kind`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVisualExamples("line.md", []byte(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestEnvelopeRowsReadsWindowBlocksInDatasetOrder(t *testing.T) {
	envelope := visualizationir.VisualizationEnvelope{DataState: visualizationir.VisualizationDataState{Value: &visualizationir.WindowedVisualizationDataState{
		Schema: visualizationir.VisualizationDatasetSchema{Fields: []visualizationir.VisualizationField{{ID: "order_id"}, {ID: "revenue"}}},
		Blocks: map[string]visualizationir.VisualizationWindowBlock{
			"b": {ID: "b", Start: 1, Rows: [][]any{{"o2", 20}}},
			"a": {ID: "a", Start: 0, Rows: [][]any{{"o1", 10}}},
		},
	}}}
	rows := envelopeRows(envelope)
	if len(rows) != 2 || rows[0]["order_id"] != "o1" || rows[1]["revenue"] != 20 {
		t.Fatalf("window rows = %#v", rows)
	}
}
