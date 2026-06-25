using Http;
using TypeSpec.OpenAPI;

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
  dashboards: DashboardSummary[];
}

model SemanticModelSummary {
  id: string;
  title: string;
  description: string;
}

model SemanticModelListResponse {
  models: SemanticModelSummary[];
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

model DashboardPageQueryResponse {
  filters: Record<string>;
  filterOptions?: Record<string>;
  status?: Record<string>;
  visuals: Record<string>;
}

model DashboardTableQueryResponse {
  title: string;
  availableRows: int32;
  blocks: Record<string>;
}

model ListDashboardsOK {
  @statusCode statusCode: 200;
  @body body: DashboardListResponse;
}

@tag("BI")
@summary("List dashboards")
@route("/api/v1/workspaces/{workspace}/dashboards")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "list"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_dashboards", risk: "read", tags: #["dashboard", "catalog"] })
op listDashboards(@path workspace: string): ListDashboardsOK | BadRequest | Unauthorized | Forbidden | NotFound | Conflict | RateLimited | InternalServerError;

model GetDashboardOK {
  @statusCode statusCode: 200;
  @body body: DashboardManifestResponse;
}

@tag("BI")
@summary("Describe dashboard")
@route("/api/v1/workspaces/{workspace}/dashboards/{dashboard}")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "describe"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "describe_dashboard", risk: "read", tags: #["dashboard", "catalog"] })
op getDashboard(@path workspace: string, @path dashboard: string): GetDashboardOK | BadRequest | Unauthorized | Forbidden | NotFound | Conflict | RateLimited | InternalServerError;

model ListSemanticModelsOK {
  @statusCode statusCode: 200;
  @body body: SemanticModelListResponse;
}

@tag("BI")
@summary("List semantic models")
@route("/api/v1/workspaces/{workspace}/semantic-models")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["semantic-models", "list"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_semantic_models", risk: "read", tags: #["semantic-model", "catalog"] })
op listSemanticModels(@path workspace: string): ListSemanticModelsOK | BadRequest | Unauthorized | Forbidden | NotFound | Conflict | RateLimited | InternalServerError;

model GetSemanticModelOK {
  @statusCode statusCode: 200;
  @body body: SemanticModelDescriptionResponse;
}

@tag("BI")
@summary("Describe semantic model")
@route("/api/v1/workspaces/{workspace}/semantic-models/{model}")
@get
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["semantic-models", "describe"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "model", displayName: "model" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "describe_model", risk: "read", tags: #["semantic-model", "catalog"] })
op getSemanticModel(@path workspace: string, @path("model") modelName: string): GetSemanticModelOK | BadRequest | Unauthorized | Forbidden | NotFound | Conflict | RateLimited | InternalServerError;

model QueryDashboardPageOK {
  @statusCode statusCode: 200;
  @body body: DashboardPageQueryResponse;
}

@tag("BI")
@summary("Query dashboard page")
@route("/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/query")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "query-page"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "page", displayName: "page" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "query_dashboard_page", risk: "read", tags: #["dashboard", "query"] })
op queryDashboardPage(@path workspace: string, @path dashboard: string, @path page: string, @body body?: DashboardPageQueryRequest): QueryDashboardPageOK | BadRequest | Unauthorized | Forbidden | NotFound | Conflict | RateLimited | InternalServerError;

model QueryDashboardTableOK {
  @statusCode statusCode: 200;
  @body body: DashboardTableQueryResponse;
}

@tag("BI")
@summary("Query dashboard table")
@route("/api/v1/workspaces/{workspace}/dashboards/{dashboard}/tables/{table}/query")
@post
@apigen.authz(#{ mode: "permission", permission: "asset:read" })
@apigen.cli(#{ command: #["dashboards", "query-table"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }, #{ source: "path", name: "dashboard", displayName: "dashboard" }, #{ source: "path", name: "table", displayName: "table" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "query_table", risk: "read", tags: #["dashboard", "query", "table"] })
op queryDashboardTable(@path workspace: string, @path dashboard: string, @path table: string, @body body?: DashboardTableQueryRequest): QueryDashboardTableOK | BadRequest | Unauthorized | Forbidden | NotFound | Conflict | RateLimited | InternalServerError;
