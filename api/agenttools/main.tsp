import "@yacobolo/apigen";

@apigen.`package`(#{
  title: "LeapView Agent Tool Contracts",
  version: "1.0.0",
  description: "Compact, model-facing contracts for LeapView's curated read-only tools.",
})
namespace LeapViewAgentTools;

model QueryFreshness {
  @doc("Completion time of the last successful publish or refresh. This is not an upstream event-time watermark.")
  lastSuccessfulRefreshAt: utcDateTime;
  @doc("DuckLake snapshot ID recorded for the successful data version.")
  snapshotId: string;
  @doc("Serving state that produced the successful data version.")
  servingStateId: string;
  @doc("Whether the data version came from publish or refresh.")
  source: "publish" | "refresh";
  @doc("current when the successful data version belongs to the queried serving snapshot; stale otherwise.")
  status: "current" | "stale";
}

model DashboardVisualColumn {
  id: string;
  sourceRef?: string;
  label: string;
  role: "dimension" | "measure" | "metadata" | "identity";
  dataType: "string" | "boolean" | "integer" | "decimal" | "temporal" | "date" | "geographic";
  nullable: boolean;
  format?: Record<unknown>;
  grain?: string;
}

model DashboardFilterControl {
  type: string;
  operator?: string;
  preset?: string;
  from?: string;
  to?: string;
  value?: string;
  values?: string[];
}

model DashboardAppliedFilters {
  controls: Record<DashboardFilterControl>;
  selections: Record<unknown>[];
  spatialSelections: Record<unknown>[];
}

model DashboardVisualStatus {
  kind: "idle" | "loading" | "ready" | "no_data" | "partial" | "error";
  message?: string;
}

model DashboardVisualDiagnostic {
  code: string;
  severity: "info" | "warning" | "error";
  message: string;
  fieldId?: string;
}

model DashboardVisualCompleteness {
  returnedRows: int32;
  availableRows?: int64;
  cardinality: "unknown" | "lower_bound" | "estimated" | "exact";
  cardinalityCount?: int64;
  state: "complete" | "truncated" | "partial" | "empty";
}

@apigen.contract(#{ kind: "agent-tool-output", tags: #["dashboard", "visual", "query"] })
model DashboardVisualQueryResult {
  queryId: string;
  servingSnapshot: string;
  freshness?: QueryFreshness;
  visualId: string;
  title: string;
  type: string;
  mark?: string;
  datasetId: string;
  columns: DashboardVisualColumn[];
  rows: unknown[][];
  appliedFilters: DashboardAppliedFilters;
  status: DashboardVisualStatus;
  diagnostics: DashboardVisualDiagnostic[];
  completeness: DashboardVisualCompleteness;
  hasMore: boolean;
  nextCursor?: string;
}

model CatalogRef {
  workspaceId: string;
  type: "workspace" | "dashboard" | "page" | "visual" | "filter" | "semantic_model" | "semantic_table" | "field" | "measure";
  id: string;
}

model QueryVisualFieldUsage {
  ref: CatalogRef;
  role: "dimension" | "series" | "measure" | "table_field" | "table_row";
  `alias`?: string;
  label: string;
  dataType?: string;
  unit?: string;
  format?: string;
}

model QueryVisualFilterUsage {
  fieldRef: CatalogRef;
  factRef?: CatalogRef;
  operator: "equals" | "in" | "contains" | "not_contains" | "starts_with" | "greater_than_or_equal" | "less_than" | "is_null" | "is_not_null";
  values: string[];
  groupPath: int32[];
}

model QueryVisualCompleteness {
  returnedRows: int32;
  limit: int32;
  status: "complete" | "limit_reached" | "empty";
}

@apigen.contract(#{ kind: "agent-tool-output", tags: #["semantic-model", "visual", "query"] })
model QueryVisualResult {
  ok: true;
  queryId: string;
  servingSnapshot: string;
  freshness?: QueryFreshness;
  type: string;
  id: string;
  title: string;
  modelRef: CatalogRef;
  datasetRef: CatalogRef;
  fields: QueryVisualFieldUsage[];
  filters: QueryVisualFilterUsage[];
  completeness: QueryVisualCompleteness;
  status: DashboardVisualStatus;
  diagnostics: DashboardVisualDiagnostic[];
  summary: string;
  signal: string;
}
