package api

import "list"

schemas: {
	"DeploymentCreateRequest": {
		type: "object"
		properties: {
			"title": {schema: {type: "string"}}
			"description": {schema: {type: "string"}}
		}
	}
	"DeploymentArtifactResponse": {
		type: "object"
		properties: {
			"deploymentId": {schema: {type: "string"}}
			"sizeBytes": {schema: {type: "integer", format: "int64"}}
		}
		required: ["deploymentId", "sizeBytes"]
	}
	"DeploymentArtifactUploadRequest": {
		type:   "string"
		format: "binary"
	}
	"DeploymentResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"status": {schema: {type: "string", enum: ["draft", "validated", "active", "inactive", "failed"]}}
			"digest": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"activatedAt": {schema: {type: "string", format: "date-time"}}
			"error": {schema: {type: "string"}}
		}
		required: ["id", "workspaceId", "status", "digest", "createdAt"]
	}
	"DeploymentListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "DeploymentResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#deploymentEndpoints: [
	{method: "post", path: "/workspaces/{workspace}/deployments", operation_id: "createDeployment", summary: "Create a deployment", tags: ["Deployments"], security: #secure, parameters: [#workspaceParam], request_body: {required: false, schema: {ref: "DeploymentCreateRequest"}}, responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "DeploymentResponse"}}], #errorResponses]), extensions: #deploymentWrite},
	{method: "get", path: "/workspaces/{workspace}/deployments", operation_id: "listDeployments", summary: "List deployments", tags: ["Deployments"], security: #secure, parameters: [#workspaceParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentListResponse"}}], #errorResponses]), cli: {command: ["deployments", "list"], output: {mode: "raw"}}, extensions: #deploymentRead},
	{method: "get", path: "/workspaces/{workspace}/deployments/{deployment}", operation_id: "getDeployment", summary: "Get a deployment", tags: ["Deployments"], security: #secure, parameters: [#workspaceParam, #deploymentParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses]), extensions: #deploymentRead},
	{method: "put", path: "/workspaces/{workspace}/deployments/{deployment}/artifact", operation_id: "uploadDeploymentArtifact", summary: "Upload a deployment artifact", tags: ["Deployments"], security: #secure, parameters: [#workspaceParam, #deploymentParam], request_body: {required: true, content_type: "application/octet-stream", schema: {ref: "DeploymentArtifactUploadRequest"}}, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentArtifactResponse"}}], #errorResponses]), extensions: {"x-authz": {mode: "permission", permission: "deployment:write"}, "x-libredash-dispatch": "raw-body"}},
	{method: "post", path: "/workspaces/{workspace}/deployments/{deployment}/validate", operation_id: "validateDeployment", summary: "Validate a deployment", tags: ["Deployments"], security: #secure, parameters: [#workspaceParam, #deploymentParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses]), extensions: #deploymentWrite},
	{method: "post", path: "/workspaces/{workspace}/deployments/{deployment}/activate", operation_id: "activateDeployment", summary: "Activate a deployment", tags: ["Deployments"], security: #secure, parameters: [#workspaceParam, #deploymentParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses]), extensions: #deploymentActivate},
]
