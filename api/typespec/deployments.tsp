using Http;
using TypeSpec.OpenAPI;

@route("/api/v1")
namespace LibreDashAPI;

model DeploymentCreateRequest {
  title?: string;
  description?: string;
}

model DeploymentArtifactResponse {
  deploymentId: string;
  sizeBytes: int64;
}

model DeploymentArtifactUploadRequest {
  artifact: bytes;
}

model DeploymentResponse {
  id: string;
  workspaceId: string;
  status: string;
  digest: string;
  createdAt: utcDateTime;
  activatedAt?: utcDateTime;
  error?: string;
}

model DeploymentListResponse {
  items: DeploymentResponse[];
  page: PageInfo;
}

alias ListDeploymentsOK = OkJson<DeploymentListResponse>;

@tag("Deployments")
@summary("List deployments")
@route("/workspaces/{workspace}/deployments")
@get
@apigen.authz(#{ mode: "permission", permission: "deployment:read" })
@apigen.cli(#{ command: #["deployments", "list"], args: #[#{ source: "path", name: "workspace", displayName: "workspace" }], output: #{ mode: "raw" } })
@extension("x-agent", #{ enabled: true, name: "list_deployments", risk: "read", tags: #["deployment"], defaultLimit: 25 })
op listDeployments(@path workspace: string, @query limit?: int32, @query pageToken?: string): ListDeploymentsOK | CommonErrors;

alias CreateDeploymentCreated = CreatedJson<DeploymentResponse>;

@tag("Deployments")
@summary("Create a deployment")
@route("/workspaces/{workspace}/deployments")
@post
@apigen.authz(#{ mode: "permission", permission: "deployment:write" })
op createDeployment(@path workspace: string, @body body?: DeploymentCreateRequest): CreateDeploymentCreated | CommonErrors;

alias GetDeploymentOK = OkJson<DeploymentResponse>;

@tag("Deployments")
@summary("Get a deployment")
@route("/workspaces/{workspace}/deployments/{deployment}")
@get
@apigen.authz(#{ mode: "permission", permission: "deployment:read" })
@extension("x-agent", #{ enabled: true, name: "get_deployment", risk: "read", tags: #["deployment"] })
op getDeployment(@path workspace: string, @path deployment: string): GetDeploymentOK | CommonErrors;

alias ActivateDeploymentOK = OkJson<DeploymentResponse>;

@tag("Deployments")
@summary("Activate a deployment")
@route("/workspaces/{workspace}/deployments/{deployment}/activate")
@post
@apigen.authz(#{ mode: "permission", permission: "deployment:activate" })
op activateDeployment(@path workspace: string, @path deployment: string): ActivateDeploymentOK | CommonErrors;

alias UploadDeploymentArtifactOK = OkJson<DeploymentArtifactResponse>;

@tag("Deployments")
@summary("Upload a deployment artifact")
@route("/workspaces/{workspace}/deployments/{deployment}/artifact")
@put
@apigen.authz(#{ mode: "permission", permission: "deployment:write" })
@extension("x-libredash-dispatch", "raw-body")
op uploadDeploymentArtifact(@path workspace: string, @path deployment: string, @header contentType: "application/octet-stream", @body body: DeploymentArtifactUploadRequest): UploadDeploymentArtifactOK | CommonErrors;

alias ValidateDeploymentOK = OkJson<DeploymentResponse>;

@tag("Deployments")
@summary("Validate a deployment")
@route("/workspaces/{workspace}/deployments/{deployment}/validate")
@post
@apigen.authz(#{ mode: "permission", permission: "deployment:write" })
op validateDeployment(@path workspace: string, @path deployment: string): ValidateDeploymentOK | CommonErrors;
