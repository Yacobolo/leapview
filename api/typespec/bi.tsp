using Http;
using TypeSpec.OpenAPI;

@route("/api/v1")
namespace LibreDashAPI;

model DashboardSummary {
  id: string;
  title: string;
  description: string;
  semanticModel: string;
  tags: string[];
  pageCount: int32;
}

model DashboardListResponse {
  items: DashboardSummary[];
  page: PageInfo;
}

model SemanticModelSummary {
  id: string;
  title: string;
  description: string;
}

model SemanticModelListResponse {
  items: SemanticModelSummary[];
  page: PageInfo;
}

model ModelRef {
  id: string;
  title: string;
}

model DashboardManifestCounts {
  pages: int32;
  visuals: int32;
  tables: int32;
  filters: int32;
}

model DashboardManifestComponent {
  id: string;
  kind: string;
  ref: string;
  title?: string;
}

model DashboardComponentPlacement {
  col?: int32;
  row?: int32;
  colSpan?: int32;
  rowSpan?: int32;
}

model DashboardComponentResponse {
  id: string;
  kind: string;
  ref?: string;
  title?: string;
  description?: string;
  placement?: DashboardComponentPlacement;
  x?: float64;
  y?: float64;
  width?: float64;
  height?: float64;
}

model DashboardComponentListResponse {
  items: DashboardComponentResponse[];
  page: PageInfo;
}

model DashboardVisualDescribeResponse {
  id: string;
  componentId?: string;
  kind?: string;
  shape?: string;
  renderer?: string;
  type?: string;
  title?: string;
  description?: string;
  query?: Record<string>;
  options?: Record<string>;
  rendererOptions?: Record<string>;
  interaction?: Record<string>;
  placement?: DashboardComponentPlacement;
  x?: float64;
  y?: float64;
  width?: float64;
  height?: float64;
}

model DashboardManifestPage {
  id: string;
  title: string;
  description?: string;
  components: DashboardManifestComponent[];
}

model DashboardManifestResponse {
  id: string;
  title: string;
  description?: string;
  `semantic_model`?: string;
  `model`?: ModelRef;
  counts: DashboardManifestCounts;
  pages: DashboardManifestPage[];
  `detail_tools`: Record<string>;
}

model SemanticModelCounts {
  sources: int32;
  `model_tables`: int32;
  fields: int32;
  measures: int32;
  relationships: int32;
}

model SemanticModelTableSummary {
  id: string;
  kind: string;
  source: string;
  description: string;
  fields: int32;
}

model ModelDashboardUsage {
  id: string;
  title: string;
  `semantic_model`: string;
  pages: int32;
}

model SemanticModelDescriptionResponse {
  id: string;
  title: string;
  description: string;
  dashboards: ModelDashboardUsage[];
  counts?: SemanticModelCounts;
  tables?: SemanticModelTableSummary[];
}

model DashboardFilters {
  controls?: Record<string>;
  selections?: Record<string>[];
}

model DashboardPageQueryRequest {
  filters?: DashboardFilters;
}

model DashboardTableQueryRequest {
  pageId?: string;
  count?: int32;
  filters?: DashboardFilters;
}

model DashboardTableDataRequest {
  count?: int32;
  filters?: DashboardFilters;
}

model DashboardPageQueryResponse {
  filters: Record<string>;
  filterOptions?: Record<string>;
  status?: Record<string>;
  visuals: Record<string>;
}

model DashboardVisualDatum {}

model DashboardVisualDataResponse {
  version?: int32;
  id?: string;
  kind?: string;
  shape?: string;
  renderer?: string;
  type?: string;
  title?: string;
  unit?: string;
  format?: string;
  interaction?: Record<string>;
  dimensions?: string[];
  measure?: string;
  measures?: string[];
  series?: string[];
  options?: Record<string>;
  rendererOptions?: Record<string>;
  selection?: Record<string>[];
  data: DashboardVisualDatum[];
}

model DashboardTableQueryResponse {
  title: string;
  availableRows: int32;
  blocks: Record<string>;
}

model DashboardFilterOptionResponse {
  value: string;
  label: string;
}

model DashboardFilterOptionListResponse {
  items: DashboardFilterOptionResponse[];
  page: PageInfo;
}

alias ListDashboardsOK = OkJson<DashboardListResponse>;

@tag("BI")
@summary("List dashboards")
@route("/workspaces/{workspace}/dashboards")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "list"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_dashboards", risk: "read", tags: #["dashboard", "catalog"], defaultLimit: 25 })
op listDashboards(@path workspace: string, @query limit?: int32, @query pageToken?: string): ListDashboardsOK | CommonErrors;

alias GetDashboardOK = OkJson<DashboardManifestResponse>;

@tag("BI")
@summary("Describe dashboard")
@route("/workspaces/{workspace}/dashboards/{dashboard}")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "describe"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "describe_dashboard", risk: "read", tags: #["dashboard", "catalog"] })
op getDashboard(@path workspace: string, @path dashboard: string): GetDashboardOK | CommonErrors;

alias ListDashboardComponentsOK = OkJson<DashboardComponentListResponse>;

@tag("BI")
@summary("List dashboard page components")
@route("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/components")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "components"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_dashboard_components", risk: "read", tags: #["dashboard", "visual"], defaultLimit: 25 })
op listDashboardComponents(@path workspace: string, @path dashboard: string, @path page: string, @query limit?: int32, @query pageToken?: string): ListDashboardComponentsOK | CommonErrors;

alias GetDashboardVisualOK = OkJson<DashboardVisualDescribeResponse>;

@tag("BI")
@summary("Describe dashboard visual")
@route("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/visuals/{visual}")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "visual"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }, #{ source: "path", name: "visual", displayName: "visual" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "describe_dashboard_visual", risk: "read", tags: #["dashboard", "visual"] })
op getDashboardVisual(@path workspace: string, @path dashboard: string, @path page: string, @path visual: string): GetDashboardVisualOK | CommonErrors;

alias ListSemanticModelsOK = OkJson<SemanticModelListResponse>;

@tag("BI")
@summary("List semantic models")
@route("/workspaces/{workspace}/semantic-models")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["semantic-models", "list"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_semantic_models", risk: "read", tags: #["semantic-model", "catalog"], defaultLimit: 25 })
op listSemanticModels(@path workspace: string, @query limit?: int32, @query pageToken?: string): ListSemanticModelsOK | CommonErrors;

alias GetSemanticModelOK = OkJson<SemanticModelDescriptionResponse>;

@tag("BI")
@summary("Describe semantic model")
@route("/workspaces/{workspace}/semantic-models/{model}")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["semantic-models", "describe"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "model", displayName: "model" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "describe_model", risk: "read", tags: #["semantic-model", "catalog"] })
op getSemanticModel(@path workspace: string, @path("model") modelName: string): GetSemanticModelOK | CommonErrors;

alias QueryDashboardPageOK = OkJson<DashboardPageQueryResponse>;

@tag("BI")
@summary("Query dashboard page")
@route("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/query")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "query-page"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "query_dashboard_page", risk: "read", tags: #["dashboard", "query"] })
op queryDashboardPage(@path workspace: string, @path dashboard: string, @path page: string, @body body?: DashboardPageQueryRequest): QueryDashboardPageOK | CommonErrors;

alias QueryDashboardVisualDataOK = OkJson<DashboardVisualDataResponse>;

@tag("BI")
@summary("Query dashboard visual data")
@route("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/visuals/{visual}/data")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "visual-data"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }, #{ source: "path", name: "visual", displayName: "visual" }], output: #{ mode: "collection" }, pagination: #{ itemsField: "data" } })
@extension("x-agent", #{ enabled: true, name: "query_dashboard_visual_data", risk: "read", tags: #["dashboard", "visual", "query"] })
op queryDashboardVisualData(@path workspace: string, @path dashboard: string, @path page: string, @path visual: string, @body body?: DashboardPageQueryRequest): QueryDashboardVisualDataOK | CommonErrors;

alias QueryDashboardTableOK = OkJson<DashboardTableQueryResponse>;

@tag("BI")
@summary("Query dashboard table")
@route("/workspaces/{workspace}/dashboards/{dashboard}/tables/{table}/query")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "query-table"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "table", displayName: "table" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "query_table", risk: "read", tags: #["dashboard", "query", "table"] })
op queryDashboardTable(@path workspace: string, @path dashboard: string, @path table: string, @body body?: DashboardTableQueryRequest): QueryDashboardTableOK | CommonErrors;

alias QueryDashboardTableDataOK = OkJson<DashboardTableQueryResponse>;

@tag("BI")
@summary("Query dashboard table data")
@route("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/tables/{table}/data")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "table-data"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }, #{ source: "path", name: "table", displayName: "table" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "query_dashboard_table_data", risk: "read", tags: #["dashboard", "query", "table"] })
op queryDashboardTableData(@path workspace: string, @path dashboard: string, @path page: string, @path table: string, @body body?: DashboardTableDataRequest): QueryDashboardTableDataOK | CommonErrors;

alias ListDashboardFilterOptionsOK = OkJson<DashboardFilterOptionListResponse>;

@tag("BI")
@summary("List dashboard filter options")
@route("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/filters/{filter}/options")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "filter-options"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }, #{ source: "path", name: "filter", displayName: "filter" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_dashboard_filter_options", risk: "read", tags: #["dashboard", "filter"], defaultLimit: 25 })
op listDashboardFilterOptions(@path workspace: string, @path dashboard: string, @path page: string, @path filter: string, @query limit?: int32, @query pageToken?: string, @body body?: DashboardPageQueryRequest): ListDashboardFilterOptionsOK | CommonErrors;
