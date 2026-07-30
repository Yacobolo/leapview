using Http;
using TypeSpec.OpenAPI;
using LeapViewAPI.Protocol;

@route("/api/v1")
namespace LeapViewAPI;

enum AuthenticationMode {
  bearer,
}

enum QueryFormat {
  json: "application/json",
  arrow: "application/vnd.apache.arrow.stream",
}

enum UploadProtocol {
  tus,
  s3Multipart: "s3_multipart",
}

enum VisualizationRendererID {
  echarts,
  tanstack,
  html,
  maplibre,
  vegaLiteSandbox: "vega-lite-sandbox",
}

enum VisualizationSpecKind {
  cartesian,
  proportional,
  hierarchy,
  polar,
  table,
  matrix,
  pivot,
  kpi,
  geographic,
  custom,
}

model VisualizationRendererCapability {
  id: VisualizationRendererID;
  version: string;
  schemaVersion: 4;
  kinds: VisualizationSpecKind[];
}

model VisualizationCapabilities {
  schemaVersion: 4;
  renderers: VisualizationRendererCapability[];
}

model CapabilitiesResponse {
  apiVersion: string;
  buildVersion: string;
  buildRevision: string;
  buildTime: string;
  buildDirty: boolean;
  buildDevelopment: boolean;
  environment: string;
  authentication: AuthenticationMode[];
  queryFormats: QueryFormat[];
  uploadProtocols: UploadProtocol[];
  visualization: VisualizationCapabilities;
}

@tag("System")
@summary("Describe server capabilities")
@route("/capabilities")
@get
@apigen.authz(#{ mode: "privilege", privilege: "USE_WORKSPACE" })
@operationId("getCapabilities")
op getCapabilities(): OkJson<CapabilitiesResponse> | CommonErrors;
