using Http;
using TypeSpec.OpenAPI;
using LeapViewAPI.Protocol;

namespace LeapViewAPI.Deployment;

enum DeploymentStatus {
  queued;
  running;
  active;
  failed;
  cancelled;
  superseded;
}

enum CandidateStatus {
  preparing;
  ready;
  failed;
  cancelled;
  expired;
}

model CandidateStartRequest {
  artifactDigest: string;
}

model CandidateArtifactRequest {
  expectedArtifactDigest: string;
  artifactDigest: string;
}

model CandidateSourceArtifact {
  path: string;
  digest: string;
}

model CandidateSynchronizationRequest {
  projectFile: string;
  artifactDigest: string;
  expectedCandidateId?: string;
  expectedArtifactDigest?: string;
  artifacts: CandidateSourceArtifact[];
}

model CandidateSynchronizationPlanResponse {
  artifactDigest: string;
  missingDigests: string[];
}

model CandidateSourceBlobResponse {
  digest: string;
  sizeBytes: int64;
}

model CandidateResponse {
  id: string;
  projectId: string;
  targetId: string;
  environment: string;
  ownerId: string;
  baseGeneration: string;
  artifactDigest: string;
  status: CandidateStatus;
  failureReason?: string;
  previewUrl: url;
  expiresAt: utcDateTime;
  createdAt: utcDateTime;
  updatedAt: utcDateTime;
  revision: int64;
  resumed?: boolean;
}

model DeploymentCreateRequest {
  releaseId: string;
}

model DeploymentTargetResponse {
  workspaceId: string;
  servingStateId?: string;
  priorServingStateId?: string;
  status: string;
  error?: string;
}

model DeploymentConnectionResponse {
  connectionId: string;
  revisionId: string;
  priorRevisionId?: string;
}

model DeploymentProgress {
  current: int32;
  total: int32;
  unit: string;
}

model DeploymentResponse {
  id: string;
  projectId: string;
  releaseId: string;
  environment: string;
  status: DeploymentStatus;
  progress?: DeploymentProgress;
  targets: DeploymentTargetResponse[];
  connections: DeploymentConnectionResponse[];
  createdBy: string;
  createdAt: utcDateTime;
  startedAt?: utcDateTime;
  finishedAt?: utcDateTime;
  error?: string;
}

model DeploymentListResponse {
  items: DeploymentResponse[];
  page: PageInfo;
}

@tag("Deployments")
@summary("List project deployments")
@route("/projects/{project}/deployments")
@get
@apigen.authz(#{ mode: "privilege", privilege: "VIEW_ITEM" })
@operationId("listDeployments")
op listDeployments(@path project: string, @query limit?: ListLimit, @query pageToken?: PageToken): OkJson<DeploymentListResponse> | CommonErrors;

@tag("Deployments")
@summary("Deploy a ready project release")
@route("/projects/{project}/deployments")
@post
@apigen.authz(#{ mode: "privilege", privilege: "ACTIVATE_DEPLOYMENT" })
@operationId("createDeployment")
op createDeployment(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: DeploymentCreateRequest): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Get a project deployment")
@route("/projects/{project}/deployments/{deployment}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "VIEW_ITEM" })
@operationId("getDeployment")
op getDeployment(@path project: string, @path deployment: string): OkJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("List or stream deployment events")
@route("/projects/{project}/deployments/{deployment}/events")
@get
@apigen.authz(#{ mode: "privilege", privilege: "VIEW_ITEM" })
@operationId("listDeploymentEvents")
op listDeploymentEvents(@path project: string, @path deployment: string, @header accept?: string, @header("Last-Event-ID") lastEventId?: string, @query limit?: ListLimit, @query pageToken?: PageToken): EventHistoryResponse<AsyncEventListResponse> | CommonErrors;

@tag("Deployments")
@summary("Cancel a deployment before cutover")
@route("/projects/{project}/deployments/{deployment}/cancel")
@post
@apigen.authz(#{ mode: "privilege", privilege: "ACTIVATE_DEPLOYMENT" })
@operationId("cancelDeployment")
op cancelDeployment(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Roll back by creating a deployment of the prior release")
@route("/projects/{project}/deployments/{deployment}/rollback")
@post
@apigen.authz(#{ mode: "privilege", privilege: "ACTIVATE_DEPLOYMENT" })
@operationId("rollbackDeployment")
op rollbackDeployment(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Create or resume the current author's private project candidate")
@route("/projects/{project}/candidates")
@post
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("startProjectCandidate")
op startProjectCandidate(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateStartRequest): CreatedJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Get an owned private project candidate")
@route("/projects/{project}/candidates/{candidate}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("getProjectCandidate")
op getProjectCandidate(@path project: string, @path candidate: string): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Replace a private candidate artifact with optimistic concurrency")
@route("/projects/{project}/candidates/{candidate}/artifact")
@put
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("replaceProjectCandidateArtifact")
op replaceProjectCandidateArtifact(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateArtifactRequest): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Retry preparation of a failed private candidate")
@route("/projects/{project}/candidates/{candidate}/retry")
@post
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("retryProjectCandidate")
op retryProjectCandidate(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Cancel an owned private project candidate")
@route("/projects/{project}/candidates/{candidate}/cancel")
@post
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("cancelProjectCandidate")
op cancelProjectCandidate(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Plan content-addressed synchronization for the current author's private candidate")
@route("/projects/{project}/candidate-sync/plan")
@post
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("planProjectCandidateSynchronization")
op planProjectCandidateSynchronization(@path project: string, @body body: CandidateSynchronizationRequest): OkJson<CandidateSynchronizationPlanResponse> | CommonErrors;

@tag("Deployments")
@summary("Upload a missing immutable private-candidate source blob")
@route("/projects/{project}/candidate-sync/blobs/{digest}")
@put
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@apigen.manual
@operationId("uploadProjectCandidateSourceBlob")
op uploadProjectCandidateSourceBlob(@path project: string, @path digest: string, @header contentType: "application/octet-stream", @header("Content-Digest") contentDigest: string, @body body: bytes): CreatedJson<CandidateSourceBlobResponse> | CommonErrors;

@tag("Deployments")
@summary("Atomically commit a complete source snapshot to the current author's private candidate")
@route("/projects/{project}/candidate-sync/commit")
@post
@apigen.authz(#{ mode: "privilege", privilege: "DEPLOY" })
@operationId("commitProjectCandidateSynchronization")
op commitProjectCandidateSynchronization(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateSynchronizationRequest): OkJson<CandidateResponse> | CommonErrors;
