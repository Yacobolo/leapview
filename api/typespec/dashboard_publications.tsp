using Http;
using TypeSpec.OpenAPI;

@route("/api/v1")
namespace LeapViewAPI;

enum DashboardPublicationStatus {
  active;
  suspended;
  unconfigured;
}

model DashboardPublicationResponse {
  name: string;
  workspaceId: string;
  projectId: string;
  dashboard: string;
  defaultPage: string;
  status: DashboardPublicationStatus;
  configured: boolean;
  allowedOrigins: string[];
  activeServingStateId?: string;
  publicUrl: string;
  embedUrl: string;
  iframeSnippet: string;
  createdAt: utcDateTime;
  updatedAt: utcDateTime;
  configuredAt?: utcDateTime;
  disabledAt?: utcDateTime;
  suspendedAt?: utcDateTime;
  suspendedBy?: string;
  rotatedAt?: utcDateTime;
}

model DashboardPublicationListResponse {
  items: DashboardPublicationResponse[];
}

@tag("Publications")
@summary("List dashboard publications in a workspace")
@route("/workspaces/{workspace}/dashboard-publications")
@get
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_PUBLICATIONS" })
op listDashboardPublications(@path workspace: string): OkJson<DashboardPublicationListResponse> | CommonErrors;

@tag("Publications")
@summary("Get a dashboard publication")
@route("/workspaces/{workspace}/dashboard-publications/{publication}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_PUBLICATIONS" })
op getDashboardPublication(@path workspace: string, @path publication: string): OkJson<DashboardPublicationResponse> | CommonErrors;

@tag("Publications")
@summary("Suspend a dashboard publication immediately")
@route("/workspaces/{workspace}/dashboard-publications/{publication}/suspend")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_PUBLICATIONS" })
op suspendDashboardPublication(@path workspace: string, @path publication: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<DashboardPublicationResponse> | CommonErrors;

@tag("Publications")
@summary("Resume a configured dashboard publication")
@route("/workspaces/{workspace}/dashboard-publications/{publication}/resume")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_PUBLICATIONS" })
op resumeDashboardPublication(@path workspace: string, @path publication: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<DashboardPublicationResponse> | CommonErrors;

@tag("Publications")
@summary("Rotate a dashboard publication public ID")
@route("/workspaces/{workspace}/dashboard-publications/{publication}/rotate")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_PUBLICATIONS" })
op rotateDashboardPublication(@path workspace: string, @path publication: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<DashboardPublicationResponse> | CommonErrors;
