import "@yacobolo/apigen";

@apigen.`package`(#{
  title: "LibreDash Visualization IR",
  version: "1.0.0",
  description: "Renderer-independent visualization specifications and runtime data contracts.",
})
namespace LibreDashVisualization;

enum VisualizationFieldRole {
  dimension: "dimension",
  measure: "measure",
  metadata: "metadata",
  identity: "identity",
}

enum VisualizationDataType {
  string: "string",
  boolean: "boolean",
  integer: "integer",
  decimal: "decimal",
  temporal: "temporal",
  date: "date",
  geographic: "geographic",
}

enum VisualizationCompleteness {
  complete: "complete",
  truncated: "truncated",
  partial: "partial",
  empty: "empty",
}

enum VisualizationTemporalMeaning {
  instant: "instant",
  bucket: "bucket",
}

enum VisualizationWeekStart {
  monday: "monday",
  tuesday: "tuesday",
  wednesday: "wednesday",
  thursday: "thursday",
  friday: "friday",
  saturday: "saturday",
  sunday: "sunday",
}

enum VisualizationCartesianMark {
  line: "line",
  area: "area",
  bar: "bar",
  column: "column",
  scatter: "scatter",
  histogram: "histogram",
  combo: "combo",
  waterfall: "waterfall",
  candlestick: "candlestick",
  boxplot: "boxplot",
  heatmap: "heatmap",
}

enum VisualizationProportionalMark {
  pie: "pie",
  donut: "donut",
  funnel: "funnel",
}

enum VisualizationHierarchyMark {
  treemap: "treemap",
  sunburst: "sunburst",
  tree: "tree",
  sankey: "sankey",
  graph: "graph",
}

enum VisualizationPolarMark {
  radar: "radar",
  gauge: "gauge",
}

enum VisualizationGeographicLayerKind {
  choropleth: "choropleth",
  point: "point",
  heat: "heat",
  density: "density",
}

enum VisualizationInteractionKind {
  select: "select",
  filter: "filter",
  drill: "drill",
  navigate: "navigate",
}

enum VisualizationSelectionMode {
  single: "single",
  multiple: "multiple",
}

enum VisualizationKPITrend {
  neutral: "neutral",
  positive: "positive",
  negative: "negative",
}

enum VisualizationTone {
  neutral: "neutral",
  ink: "ink",
  success: "success",
  warning: "warning",
  danger: "danger",
}

enum VisualizationLabelPosition {
  automatic: "automatic",
  inside: "inside",
  outside: "outside",
  top: "top",
}

enum VisualizationHierarchyLayout {
  standard: "standard",
  circular: "circular",
}

enum VisualizationGraphFocus {
  none: "none",
  adjacency: "adjacency",
}

enum VisualizationAxis {
  primary: "primary",
  secondary: "secondary",
}

enum VisualizationOrientation {
  horizontal: "horizontal",
  vertical: "vertical",
}

enum VisualizationLegendPosition {
  hidden: "hidden",
  top: "top",
  right: "right",
  bottom: "bottom",
  left: "left",
}

enum VisualizationCustomEngine {
  vegaLite: "vega_lite",
}

enum VisualizationSortDirection {
  ascending: "ascending",
  descending: "descending",
}

enum VisualizationCardinalityKind {
  unknownValue: "unknown",
  lowerBound: "lower_bound",
  estimated: "estimated",
  exact: "exact",
}

enum VisualizationStatusKind {
  idle: "idle",
  loading: "loading",
  ready: "ready",
  noData: "no_data",
  partial: "partial",
  error: "error",
}

enum VisualizationDiagnosticSeverity {
  info: "info",
  warning: "warning",
  error: "error",
}

model VisualizationTemporalMetadata {
  `grain`: string;
  `timezone`: string;
  `calendar`: string;
  `weekStart`: VisualizationWeekStart;
  `meaning`: VisualizationTemporalMeaning;
}

model VisualizationGeographicMetadata {
  `identifierSystem`: string;
  `geometryAsset`: string;
  `joinField`: string;
  `unmatchedPolicy`: string;
}

@discriminator("kind")
model VisualizationFormat {
  `kind`: string;
}

model NumberVisualizationFormat extends VisualizationFormat {
  `kind`: "number";
  `minimumFractionDigits`?: int32;
  `maximumFractionDigits`?: int32;
}

model CurrencyVisualizationFormat extends VisualizationFormat {
  `kind`: "currency";
  `currency`: string;
  `minimumFractionDigits`?: int32;
  `maximumFractionDigits`?: int32;
}

model PercentVisualizationFormat extends VisualizationFormat {
  `kind`: "percent";
  `minimumFractionDigits`?: int32;
  `maximumFractionDigits`?: int32;
}

model CompactVisualizationFormat extends VisualizationFormat {
  `kind`: "compact";
  `maximumFractionDigits`?: int32;
}

model DurationVisualizationFormat extends VisualizationFormat {
  `kind`: "duration";
  `unit`: string;
}

model TemporalVisualizationFormat extends VisualizationFormat {
  `kind`: "temporal";
  `dateStyle`?: string;
  `timeStyle`?: string;
}

model VisualizationGridFieldMetadata {
  `group`?: string;
  `measure`?: string;
  `columnValue`?: string;
  `formatting`: TableVisualizationFormattingRule[];
}

model VisualizationField {
  `id`: string;
  `sourceRef`?: string;
  `role`: VisualizationFieldRole;
  `dataType`: VisualizationDataType;
  `nullable`: boolean;
  `label`: string;
  `format`?: VisualizationFormat;
  `time`?: VisualizationTemporalMetadata;
  `geographic`?: VisualizationGeographicMetadata;
  `grid`?: VisualizationGridFieldMetadata;
}

model VisualizationDatasetSchema {
  `id`: string;
  `fields`: VisualizationField[];
}

model VisualizationFieldRef {
  `dataset`: string;
  `field`: string;
}

model VisualizationDataBudget {
  `maxRows`: int64;
  `requiredCompleteness`: VisualizationCompleteness;
}

model VisualizationAccessibility {
  `title`: string;
  `description`: string;
  `summary`?: string;
  `announceChanges`?: boolean;
}

model VisualizationInteractionMapping {
  `source`: VisualizationFieldRef;
  `targetFieldID`: string;
  `targetFactID`?: string;
  `grain`?: string;
  `label`?: VisualizationFieldRef;
}

model VisualizationInteraction {
  `id`: string;
  `kind`: VisualizationInteractionKind;
  `mappings`: VisualizationInteractionMapping[];
  `targets`: string[];
  `mode`: VisualizationSelectionMode;
  `requiresStableIdentity`: boolean;
}

model VisualizationPresentation {
  `legend`: VisualizationLegendPosition;
  `showLabels`: boolean;
}

model VisualizationThreshold {
  `value`: float64;
  `tone`: VisualizationTone;
}

model VisualizationComboSeries {
  `seriesValue`: string;
  `mark`: VisualizationCartesianMark;
  `axis`: VisualizationAxis;
}

model CartesianVisualizationPresentation extends VisualizationPresentation {
  `smooth`: boolean;
  `stacked`: boolean;
  `showSymbols`: boolean;
  `dataZoom`: boolean;
  `area`: boolean;
  `step`: boolean;
  `orientation`?: VisualizationOrientation;
  `labelPosition`?: VisualizationLabelPosition;
  `symbolSize`?: float64;
  `histogramBins`?: int32;
  `comboSeries`?: VisualizationComboSeries[];
}

model ProportionalVisualizationPresentation extends VisualizationPresentation {
  `orientation`: VisualizationOrientation;
  `rose`: boolean;
  `centerLabel`?: string;
  `labelPosition`?: VisualizationLabelPosition;
  `innerRadius`?: float64;
  `outerRadius`?: float64;
  `align`?: string;
  `sort`?: VisualizationSortDirection;
}

model HierarchyVisualizationPresentation extends VisualizationPresentation {
  `orientation`: VisualizationOrientation;
  `initialDepth`?: int32;
  `roam`: boolean;
  `layout`?: VisualizationHierarchyLayout;
  `breadcrumb`?: boolean;
  `nodeGap`?: float64;
  `curveness`?: float64;
  `focus`?: VisualizationGraphFocus;
}

model PolarVisualizationPresentation extends VisualizationPresentation {
  `minimum`?: float64;
  `maximum`?: float64;
  `showPointer`: boolean;
  `area`?: boolean;
  `progressWidth`?: float64;
  `thresholds`?: VisualizationThreshold[];
}

model GridVisualizationPresentation {
  `rowHeight`: int64;
  `striped`: boolean;
  `showHeader`: boolean;
}

model KPIVisualizationPresentation {
  `trend`: VisualizationKPITrend;
  `note`?: string;
  `tone`?: VisualizationTone;
  `thresholds`?: VisualizationThreshold[];
}

model GeographicVisualizationPresentation extends VisualizationPresentation {
  `roam`: boolean;
  `basemap`?: VisualizationGeometryAsset;
}

@apigen.contract(#{ kind: "visualization-spec", tags: #["visualization", "ir"] })
@apigen.`metadata`(#{ `x-libredash-contract-role`: "visualization-spec" })
@discriminator("kind")
model VisualizationSpec {
  `kind`: string;
  `title`: string;
  `datasets`: VisualizationDatasetSchema[];
  `dataBudget`: VisualizationDataBudget;
  `accessibility`: VisualizationAccessibility;
  `interactions`: VisualizationInteraction[];
}

model CartesianVisualizationSpec extends VisualizationSpec {
  `kind`: "cartesian";
  `mark`: VisualizationCartesianMark;
  `x`: VisualizationFieldRef;
  `y`: VisualizationFieldRef[];
  `series`?: VisualizationFieldRef;
  `presentation`: CartesianVisualizationPresentation;
}

model ProportionalVisualizationSpec extends VisualizationSpec {
  `kind`: "proportional";
  `mark`: VisualizationProportionalMark;
  `category`: VisualizationFieldRef;
  `value`: VisualizationFieldRef;
  `series`?: VisualizationFieldRef;
  `presentation`: ProportionalVisualizationPresentation;
}

model HierarchyVisualizationSpec extends VisualizationSpec {
  `kind`: "hierarchy";
  `mark`: VisualizationHierarchyMark;
  `node`: VisualizationFieldRef;
  `parent`?: VisualizationFieldRef;
  `source`?: VisualizationFieldRef;
  `target`?: VisualizationFieldRef;
  `value`?: VisualizationFieldRef;
  `presentation`: HierarchyVisualizationPresentation;
}

model PolarVisualizationSpec extends VisualizationSpec {
  `kind`: "polar";
  `mark`: VisualizationPolarMark;
  `category`?: VisualizationFieldRef;
  `value`: VisualizationFieldRef;
  `series`?: VisualizationFieldRef;
  `presentation`: PolarVisualizationPresentation;
}

@discriminator("kind")
model TableVisualizationFormattingRule {
  `kind`: string;
}

model TableBadgeFormattingRule extends TableVisualizationFormattingRule {
  `kind`: "badge";
  `values`: Record<string>;
}

model TableTextColorFormattingRule extends TableVisualizationFormattingRule {
  `kind`: "text_color";
  `color`: string;
  `values`?: Record<string>;
  `minimum`?: float64;
  `maximum`?: float64;
}

model TableBackgroundScaleFormattingRule extends TableVisualizationFormattingRule {
  `kind`: "background_scale";
  `minimum`?: float64;
  `maximum`?: float64;
  `lowColor`?: string;
  `highColor`?: string;
}

model TableDataBarFormattingRule extends TableVisualizationFormattingRule {
  `kind`: "data_bar";
  `minimum`?: float64;
  `maximum`?: float64;
  `color`: string;
  `background`?: string;
}

model TableVisualizationColumn {
  `field`: VisualizationFieldRef;
  `label`: string;
  `width`?: int64;
  `group`?: string;
  `measure`?: string;
  `columnValue`?: string;
  `formatting`: TableVisualizationFormattingRule[];
}

model VisualizationSort {
  `field`: VisualizationFieldRef;
  `direction`: VisualizationSortDirection;
}

model TableVisualizationSpec extends VisualizationSpec {
  `kind`: "table";
  `columns`: TableVisualizationColumn[];
  `defaultSort`?: VisualizationSort[];
  `presentation`: GridVisualizationPresentation;
}

model MatrixVisualizationSpec extends VisualizationSpec {
  `kind`: "matrix";
  `rows`: VisualizationFieldRef[];
  `columns`: VisualizationFieldRef[];
  `measures`: VisualizationFieldRef[];
  `measureFormatting`: Record<TableVisualizationFormattingRule[]>;
  `presentation`: GridVisualizationPresentation;
}

model PivotVisualizationSpec extends VisualizationSpec {
  `kind`: "pivot";
  `rows`: VisualizationFieldRef[];
  `columns`: VisualizationFieldRef[];
  `measures`: VisualizationFieldRef[];
  `measureFormatting`: Record<TableVisualizationFormattingRule[]>;
  `presentation`: GridVisualizationPresentation;
}

model KPIVisualizationSpec extends VisualizationSpec {
  `kind`: "kpi";
  `value`: VisualizationFieldRef;
  `comparison`?: VisualizationFieldRef;
  `trend`?: VisualizationFieldRef;
  `presentation`: KPIVisualizationPresentation;
}

model VisualizationGeometryAsset {
  `id`: string;
  `digest`: string;
  `source`: string;
  `license`: string;
  `attribution`: string;
  `identifierSystem`: string;
  `url`: string;
}

model VisualizationGeographicLayer {
  `id`: string;
  `kind`: VisualizationGeographicLayerKind;
  `geometry`?: VisualizationGeometryAsset;
  `join`?: VisualizationFieldRef;
  `value`?: VisualizationFieldRef;
  `latitude`?: VisualizationFieldRef;
  `longitude`?: VisualizationFieldRef;
}

model GeographicVisualizationSpec extends VisualizationSpec {
  `kind`: "geographic";
  `layers`: VisualizationGeographicLayer[];
  `presentation`: GeographicVisualizationPresentation;
}

model CustomVisualizationSpec extends VisualizationSpec {
  `kind`: "custom";
  `engine`: VisualizationCustomEngine;
  `program`: string;
  `programDigest`: string;
}

model VisualizationInlineDataset {
  `id`: string;
  `specRevision`: string;
  `dataRevision`: int64;
  `generation`: int64;
  `columns`: string[];
  `rows`: unknown[][];
  `completeness`: VisualizationCompleteness;
}

model VisualizationCardinality {
  `kind`: VisualizationCardinalityKind;
  `count`?: int64;
}

model VisualizationWindowBlock {
  `id`: string;
  `start`: int64;
  `rows`: unknown[][];
  `requestSeq`: int64;
  `resetVersion`: int64;
  `sort`: VisualizationSort[];
}

model VisualizationWindowRequest {
  `visualID`: string;
  `specRevision`: string;
  `dataRevision`: int64;
  `requestSeq`: int64;
  `resetVersion`: int64;
  `start`: int64;
  `limit`: int64;
  `sort`: VisualizationSort[];
  `blockID`: string;
}

@discriminator("kind")
model VisualizationDataState {
  `kind`: string;
  `specRevision`: string;
  `dataRevision`: int64;
  `generation`: int64;
}

model InlineVisualizationDataState extends VisualizationDataState {
  `kind`: "inline";
  `datasets`: VisualizationInlineDataset[];
}

model WindowedVisualizationDataState extends VisualizationDataState {
  `kind`: "windowed";
  `schema`: VisualizationDatasetSchema;
  `cardinality`: VisualizationCardinality;
  `availableRows`: int64;
  `rowCap`: int64;
  `chunkSize`: int64;
  `resetVersion`: int64;
  `sort`: VisualizationSort[];
  `blocks`: Record<VisualizationWindowBlock>;
}

model VisualizationDatumRef {
  `dataset`: string;
  `dataRevision`: int64;
  `identity`: Record<unknown>;
  `resetVersion`?: int64;
  `blockID`?: string;
}

model VisualizationSelectionEntry {
  `datum`: VisualizationDatumRef;
  `label`?: string;
}

model VisualizationStatus {
  `kind`: VisualizationStatusKind;
  `message`?: string;
}

model VisualizationDiagnostic {
  `code`: string;
  `severity`: VisualizationDiagnosticSeverity;
  `message`: string;
  `fieldID`?: string;
}

@apigen.contract(#{ kind: "visualization-envelope", tags: #["visualization", "ir"] })
@apigen.`metadata`(#{ `x-libredash-contract-role`: "visualization-envelope" })
model VisualizationEnvelope {
  `schemaVersion`: 1;
  `visualID`: string;
  `rendererID`: string;
  `specRevision`: string;
  `spec`: VisualizationSpec;
  `dataRevision`: int64;
  `dataState`: VisualizationDataState;
  `selection`: VisualizationSelectionEntry[];
  `status`: VisualizationStatus;
  `diagnostics`: VisualizationDiagnostic[];
}
