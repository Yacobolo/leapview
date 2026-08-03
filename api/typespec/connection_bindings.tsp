using Http;
using TypeSpec.OpenAPI;
using LeapViewAPI.Protocol;

namespace LeapViewAPI.Analytics;

@minLength(1)
@maxLength(128)
@pattern("^[A-Za-z_][A-Za-z0-9_]{0,127}$")
scalar LogicalConnectionId extends string;

@minLength(1)
@maxLength(128)
@pattern("^[A-Za-z_][A-Za-z0-9_.:-]{0,127}$")
scalar TargetConnectionIdentifier extends string;

@minValue(1)
scalar TargetConnectionRevision extends int64;

enum TargetConnectionAuthenticationMode {
  none;
  externalBundle: "external_bundle";
  workloadIdentity: "workload_identity";
}

enum TargetConnectionHealth {
  pending;
  healthy;
  degraded;
  disabled;
}

model TargetConnectionEndpoint {
  host?: string;
  port?: int32;
  database?: string;
  objectScope?: string;
  sourceIdentity?: string;
  tlsMode?: string;
  options?: Record<string>;
}

@doc("A provider-side reference only. LeapView never returns or accepts the referenced secret value.")
model TargetConnectionCredentialReference {
  projectId: string;
  environment: string;
  secretPath: string;
  secretKey: string;
}

model TargetConnectionConfiguration {
  connectorKind: TargetConnectionIdentifier;
  authenticationMode: TargetConnectionAuthenticationMode;
  endpoint: TargetConnectionEndpoint;
  credentialReference?: TargetConnectionCredentialReference;
}

model TargetConnectionBindingCreateRequest {
  id: TargetConnectionIdentifier;
  logicalConnection: LogicalConnectionId;
  configuration: TargetConnectionConfiguration;
  enabled: boolean;
}

model TargetConnectionBindingUpdateRequest {
  configuration: TargetConnectionConfiguration;
  expectedRevision: TargetConnectionRevision;
  confirmationToken?: string;
}

model TargetConnectionBindingPlanRequest {
  configuration: TargetConnectionConfiguration;
}

model TargetConnectionDependency {
  kind: string;
  id: string;
  label: string;
}

model TargetConnectionBindingChangePlanResponse {
  bindingId: string;
  expectedRevision: TargetConnectionRevision;
  requiresConfirmation: boolean;
  confirmationToken?: string;
  dependencies: TargetConnectionDependency[];
}

@doc("Target binding metadata. Credential references identify provider objects but never contain secret values.")
model TargetConnectionBindingResponse {
  id: string;
  targetId: string;
  logicalConnection: LogicalConnectionId;
  connectorKind: TargetConnectionIdentifier;
  authenticationMode: TargetConnectionAuthenticationMode;
  workspaceId: string;
  environment: TargetConnectionIdentifier;
  endpoint: TargetConnectionEndpoint;
  credentialReference?: TargetConnectionCredentialReference;
  enabled: boolean;
  validatedVersion?: string;
  health: TargetConnectionHealth;
  lastValidatedAt?: utcDateTime;
  createdAt: utcDateTime;
  updatedAt: utcDateTime;
  revision: TargetConnectionRevision;
}

model TargetConnectionBindingListResponse {
  items: TargetConnectionBindingResponse[];
}

@doc("Redacted runtime health. Diagnostic codes are bounded LeapView codes, never raw provider errors.")
model TargetConnectionBindingHealthResponse {
  bindingId: string;
  targetId: string;
  logicalConnection: LogicalConnectionId;
  connectorKind: TargetConnectionIdentifier;
  workspaceId: string;
  environment: TargetConnectionIdentifier;
  bindingRevision: TargetConnectionRevision;
  validatedVersion?: string;
  health: TargetConnectionHealth;
  diagnosticCode?: string;
  lastAttemptAt?: utcDateTime;
  lastValidatedAt?: utcDateTime;
  staleAgeSeconds?: int64;
  hasActivePool: boolean;
}

@tag("Connections")
@summary("List target connection bindings")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings")
@get
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("listTargetConnectionBindings")
op listTargetConnectionBindings(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
): OkJson<TargetConnectionBindingListResponse> | CommonErrors;

@tag("Connections")
@summary("Create a target connection binding")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("createTargetConnectionBinding")
op createTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @header("Idempotency-Key") idempotencyKey: IdempotencyKey,
  @body body: TargetConnectionBindingCreateRequest,
): CreatedJson<TargetConnectionBindingResponse> | CommonErrors;

@tag("Connections")
@summary("Get target connection binding metadata")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("getTargetConnectionBinding")
op getTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
): OkJson<TargetConnectionBindingResponse> | CommonErrors;

@tag("Connections")
@summary("Plan a dependency-aware target connection change")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}/plan")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("planTargetConnectionBindingChange")
op planTargetConnectionBindingChange(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
  @body body: TargetConnectionBindingPlanRequest,
): OkJson<TargetConnectionBindingChangePlanResponse> | CommonErrors;

@tag("Connections")
@summary("Update target connection binding metadata")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}")
@put
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("updateTargetConnectionBinding")
op updateTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
  @body body: TargetConnectionBindingUpdateRequest,
): OkJson<TargetConnectionBindingResponse> | CommonErrors;

@tag("Connections")
@summary("Test a target connection through the candidate runtime path")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}/test")
@post
@apigen.authz(#{ mode: "privilege", privilege: "TEST_CONNECTION" })
@operationId("testTargetConnectionBinding")
op testTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
  @header("Idempotency-Key") idempotencyKey: IdempotencyKey,
): OkJson<TargetConnectionBindingHealthResponse> | CommonErrors;

@tag("Connections")
@summary("Refresh credentials and replace the validated target pool")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}/refresh")
@post
@apigen.authz(#{ mode: "privilege", privilege: "TEST_CONNECTION" })
@operationId("refreshTargetConnectionBinding")
op refreshTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
  @header("Idempotency-Key") idempotencyKey: IdempotencyKey,
): OkJson<TargetConnectionBindingHealthResponse> | CommonErrors;

@tag("Connections")
@summary("Enable a target connection binding")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}/enable")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("enableTargetConnectionBinding")
op enableTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
  @header("Idempotency-Key") idempotencyKey: IdempotencyKey,
): OkJson<TargetConnectionBindingResponse> | CommonErrors;

@tag("Connections")
@summary("Disable a target connection binding")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}/disable")
@post
@apigen.authz(#{ mode: "privilege", privilege: "MANAGE_CONNECTION_METADATA" })
@operationId("disableTargetConnectionBinding")
op disableTargetConnectionBinding(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
  @header("Idempotency-Key") idempotencyKey: IdempotencyKey,
): OkJson<TargetConnectionBindingResponse> | CommonErrors;

@tag("Connections")
@summary("Get redacted target connection runtime health")
@route("/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings/{connection}/health")
@get
@apigen.authz(#{ mode: "privilege", privilege: "VIEW_CONNECTION_HEALTH" })
@operationId("getTargetConnectionBindingHealth")
op getTargetConnectionBindingHealth(
  @path workspace: string,
  @path target: TargetConnectionIdentifier,
  @path environment: TargetConnectionIdentifier,
  @path connection: LogicalConnectionId,
): OkJson<TargetConnectionBindingHealthResponse> | CommonErrors;
