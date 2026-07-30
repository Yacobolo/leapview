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

enum DeploymentApprovalStatus {
  pending;
  approved;
  denied;
  revoked;
  expired;
}

model CandidateStartRequest {
  artifactDigest: string;
  candidateKey?: string;
}

model CandidateArtifactRequest {
  expectedArtifactDigest: string;
  artifactDigest: string;
}

model CandidatePublishRequest {
  expectedRevision: int64;
  provenanceDigest: string;
  targetId: string;
}

model CandidateSourceArtifact {
  path: string;
  digest: string;
}

model CandidateSourceRevision {
  revision: string;
  repository?: string;
  ref?: string;
  changeId?: string;
}

model CandidateSynchronizationRequest {
  projectFile: string;
  artifactDigest: string;
  candidateKey?: string;
  sourceRevision?: CandidateSourceRevision;
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
  candidateKey: string;
  targetId: string;
  environment: string;
  ownerId: string;
  baseGeneration: string;
  artifactDigest: string;
  provenanceDigest?: string;
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

model DeploymentApprovalDecisionRequest {
  expectedRevision: int64;
}

model DeploymentApprovalResponse {
  id: string;
  projectId: string;
  deploymentId: string;
  environment: string;
  requestDigest: string;
  releaseId: string;
  status: DeploymentApprovalStatus;
  requestedBy: string;
  requestedAt: utcDateTime;
  approvedBy?: string;
  approvedAt?: utcDateTime;
  deniedBy?: string;
  deniedAt?: utcDateTime;
  revokedBy?: string;
  revokedAt?: utcDateTime;
  expiresAt: utcDateTime;
  revision: int64;
}

model DeploymentManagedDataPinEvidence {
  connectionId: string;
  revisionId: string;
}

model DeploymentBindingEvidence {
  bindingId: string;
  revision: int64;
  validatedVersion: string;
}

model DeploymentWorkspacePublishEvidence {
  workspaceId: string;
  servingStateId: string;
  artifactDigest: string;
  dataRevision: string;
  dataMode: string;
  managedDataPins: DeploymentManagedDataPinEvidence[];
  bindings: DeploymentBindingEvidence[];
}

model DeploymentPublishEvidence {
  releaseDigest: string;
  artifactDigest: string;
  planDigest: string;
  candidateId: string;
  candidateRevision: int64;
  targetId: string;
  baseGeneration: string;
  runtimeVersion: string;
  policyDigest: string;
  sourceRevision?: CandidateSourceRevision;
  workspaces: DeploymentWorkspacePublishEvidence[];
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

model DeploymentVerification {
  digest: string;
  verifiedAt: utcDateTime;
}

model DeploymentResponse {
  id: string;
  projectId: string;
  releaseId: string;
  environment: string;
  requestDigest: string;
  evidence: DeploymentPublishEvidence;
  status: DeploymentStatus;
  progress?: DeploymentProgress;
  targets: DeploymentTargetResponse[];
  connections: DeploymentConnectionResponse[];
  createdBy: string;
  activationPrincipal?: string;
  verification?: DeploymentVerification;
  createdAt: utcDateTime;
  startedAt?: utcDateTime;
  finishedAt?: utcDateTime;
  error?: string;
  approval?: DeploymentApprovalResponse;
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
@extension("x-leapview-object-scope", "project-environment")
@operationId("listDeployments")
op listDeployments(@path project: string, @query limit?: ListLimit, @query pageToken?: PageToken): OkJson<DeploymentListResponse> | CommonErrors;

@tag("Deployments")
@summary("Deploy a ready project release")
@route("/projects/{project}/deployments")
@post
@apigen.authz(#{ mode: "privilege", privilege: "REQUEST_DEPLOYMENT" })
@operationId("createDeployment")
op createDeployment(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: DeploymentCreateRequest): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Get a project deployment")
@route("/projects/{project}/deployments/{deployment}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "VIEW_ITEM" })
@extension("x-leapview-object-scope", "project-environment")
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
@apigen.authz(#{ mode: "privilege", privilege: "REQUEST_DEPLOYMENT" })
@operationId("cancelDeployment")
op cancelDeployment(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Retry a terminal publication request without changing its immutable release")
@route("/projects/{project}/deployments/{deployment}/retry")
@post
@apigen.authz(#{ mode: "privilege", privilege: "REQUEST_DEPLOYMENT" })
@operationId("retryDeployment")
op retryDeployment(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Roll back by creating a deployment of the prior release")
@route("/projects/{project}/deployments/{deployment}/rollback")
@post
@apigen.authz(#{ mode: "privilege", privilege: "ROLLBACK_DEPLOYMENT" })
@operationId("rollbackDeployment")
op rollbackDeployment(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Request approval for an immutable deployment plan")
@route("/projects/{project}/deployments/{deployment}/approval-requests")
@post
@apigen.authz(#{ mode: "privilege", privilege: "REQUEST_DEPLOYMENT" })
@operationId("requestDeploymentApproval")
op requestDeploymentApproval(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): CreatedJson<DeploymentApprovalResponse> | CommonErrors;

@tag("Deployments")
@summary("Approve an immutable deployment plan")
@route("/projects/{project}/deployments/{deployment}/approval-requests/{approval}/approve")
@post
@apigen.authz(#{ mode: "privilege", privilege: "APPROVE_DEPLOYMENT" })
@extension("x-leapview-object-scope", "project-environment")
@operationId("approveDeployment")
op approveDeployment(@path project: string, @path deployment: string, @path approval: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: DeploymentApprovalDecisionRequest): OkJson<DeploymentApprovalResponse> | CommonErrors;

@tag("Deployments")
@summary("Deny an immutable deployment plan")
@route("/projects/{project}/deployments/{deployment}/approval-requests/{approval}/deny")
@post
@apigen.authz(#{ mode: "privilege", privilege: "APPROVE_DEPLOYMENT" })
@extension("x-leapview-object-scope", "project-environment")
@operationId("denyDeploymentApproval")
op denyDeploymentApproval(@path project: string, @path deployment: string, @path approval: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: DeploymentApprovalDecisionRequest): OkJson<DeploymentApprovalResponse> | CommonErrors;

@tag("Deployments")
@summary("Revoke a pending or approved deployment decision")
@route("/projects/{project}/deployments/{deployment}/approval-requests/{approval}/revoke")
@post
@apigen.authz(#{ mode: "privilege", privilege: "APPROVE_DEPLOYMENT" })
@extension("x-leapview-object-scope", "project-environment")
@operationId("revokeDeploymentApproval")
op revokeDeploymentApproval(@path project: string, @path deployment: string, @path approval: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: DeploymentApprovalDecisionRequest): OkJson<DeploymentApprovalResponse> | CommonErrors;

@tag("Deployments")
@summary("Activate an approved immutable deployment plan")
@route("/projects/{project}/deployments/{deployment}/activate")
@post
@apigen.authz(#{ mode: "privilege", privilege: "ACTIVATE_DEPLOYMENT" })
@extension("x-leapview-object-scope", "project-environment")
@operationId("activateDeployment")
op activateDeployment(@path project: string, @path deployment: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Create or resume the current author's private project candidate")
@route("/projects/{project}/candidates")
@post
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("startProjectCandidate")
op startProjectCandidate(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateStartRequest): CreatedJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Get an owned private project candidate")
@route("/projects/{project}/candidates/{candidate}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("getProjectCandidate")
op getProjectCandidate(@path project: string, @path candidate: string): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Review a private candidate without acquiring its data credentials")
@route("/projects/{project}/candidate-reviews/{candidate}")
@get
@apigen.authz(#{ mode: "privilege", privilege: "REVIEW_CANDIDATE" })
@operationId("reviewProjectCandidate")
op reviewProjectCandidate(@path project: string, @path candidate: string): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Replace a private candidate artifact with optimistic concurrency")
@route("/projects/{project}/candidates/{candidate}/artifact")
@put
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("replaceProjectCandidateArtifact")
op replaceProjectCandidateArtifact(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateArtifactRequest): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Retry preparation of a failed private candidate")
@route("/projects/{project}/candidates/{candidate}/retry")
@post
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("retryProjectCandidate")
op retryProjectCandidate(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Cancel an owned private project candidate")
@route("/projects/{project}/candidates/{candidate}/cancel")
@post
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("cancelProjectCandidate")
op cancelProjectCandidate(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Cancel the current principal's active candidate by stable authoring key")
@route("/projects/{project}/candidate-sessions/{candidateKey}/cancel")
@post
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("cancelProjectCandidateByKey")
op cancelProjectCandidateByKey(@path project: string, @path candidateKey: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey): OkJson<CandidateResponse> | CommonErrors;

@tag("Deployments")
@summary("Publish the exact ready candidate through target policy")
@route("/projects/{project}/candidates/{candidate}/publish")
@post
@apigen.authz(#{ mode: "privilege", privilege: "PUBLISH_RELEASE" })
@operationId("publishProjectCandidate")
op publishProjectCandidate(@path project: string, @path candidate: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidatePublishRequest): AcceptedJson<DeploymentResponse> | CommonErrors;

@tag("Deployments")
@summary("Plan content-addressed synchronization for the current author's private candidate")
@route("/projects/{project}/candidate-sync/plan")
@post
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("planProjectCandidateSynchronization")
op planProjectCandidateSynchronization(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateSynchronizationRequest): OkJson<CandidateSynchronizationPlanResponse> | CommonErrors;

@tag("Deployments")
@summary("Upload a missing immutable private-candidate source blob")
@route("/projects/{project}/candidate-sync/blobs/{digest}")
@put
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@apigen.manual
@operationId("uploadProjectCandidateSourceBlob")
op uploadProjectCandidateSourceBlob(@path project: string, @path digest: string, @header contentType: "application/octet-stream", @header("Content-Digest") contentDigest: string, @body body: bytes): CreatedJson<CandidateSourceBlobResponse> | CommonErrors;

@tag("Deployments")
@summary("Atomically commit a complete source snapshot to the current author's private candidate")
@route("/projects/{project}/candidate-sync/commit")
@post
@apigen.authz(#{ mode: "privilege", privilege: "AUTHOR_PROJECT" })
@operationId("commitProjectCandidateSynchronization")
op commitProjectCandidateSynchronization(@path project: string, @header("Idempotency-Key") idempotencyKey: IdempotencyKey, @body body: CandidateSynchronizationRequest): OkJson<CandidateResponse> | CommonErrors;
