package api

import "list"

schemas: {
	"MaterializationRunCreateRequest": {
		type: "object"
		properties: {
			"modelId": {schema: {type: "string"}}
			"deploymentId": {schema: {type: "string"}}
		}
		required: ["modelId"]
	}
	"MaterializationRunResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"modelId": {schema: {type: "string"}}
			"deploymentId": {schema: {type: "string"}}
			"status": {schema: {type: "string", enum: ["queued", "running", "succeeded", "failed"]}}
			"error": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"startedAt": {schema: {type: "string", format: "date-time"}}
			"finishedAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "workspaceId", "modelId", "status", "createdAt"]
	}
	"MaterializationRunListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "MaterializationRunResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#materializationEndpoints: [
	{method: "post", path: "/workspaces/{workspace}/materialization-runs", operation_id: "createMaterializationRun", summary: "Create a materialization run", tags: ["Materializations"], security: #secure, parameters: [#workspaceParam], request_body: {required: true, schema: {ref: "MaterializationRunCreateRequest"}}, responses: list.Concat([[{status_code: 202, description: "accepted", schema: {ref: "MaterializationRunResponse"}}], #errorResponses]), extensions: #materializationRun},
	{method: "get", path: "/workspaces/{workspace}/materialization-runs", operation_id: "listMaterializationRuns", summary: "List materialization runs", tags: ["Materializations"], security: #secure, parameters: [#workspaceParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "MaterializationRunListResponse"}}], #errorResponses]), extensions: #materializationRun},
	{method: "get", path: "/workspaces/{workspace}/materialization-runs/{run}", operation_id: "getMaterializationRun", summary: "Get a materialization run", tags: ["Materializations"], security: #secure, parameters: [#workspaceParam, #runParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "MaterializationRunResponse"}}], #errorResponses]), extensions: #materializationRun},
]
