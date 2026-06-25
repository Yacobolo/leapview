package api

import "list"

schemas: {
	"WorkspaceResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"title": {schema: {type: "string"}}
			"description": {schema: {type: "string"}}
			"activeDeploymentId": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"updatedAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "title", "description", "createdAt", "updatedAt"]
	}
	"WorkspaceListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "WorkspaceResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"AssetResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"snapshotId": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"deploymentId": {schema: {type: "string"}}
			"type": {schema: {type: "string"}}
			"key": {schema: {type: "string"}}
			"parentId": {schema: {type: "string"}}
			"title": {schema: {type: "string"}}
			"description": {schema: {type: "string"}}
			"payloadSchema": {schema: {type: "string"}}
			"payload": {schema: {type: "object", additional_properties: {any: true}}}
			"href": {schema: {type: "string"}}
		}
		required: ["id", "workspaceId", "deploymentId", "type", "key", "title", "description", "payloadSchema"]
	}
	"AssetListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AssetResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"AssetEdgeResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"deploymentId": {schema: {type: "string"}}
			"fromAssetId": {schema: {type: "string"}}
			"toAssetId": {schema: {type: "string"}}
			"type": {schema: {type: "string"}}
		}
		required: ["id", "workspaceId", "deploymentId", "fromAssetId", "toAssetId", "type"]
	}
	"AssetEdgeListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AssetEdgeResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#workspaceEndpoints: [
	{method: "get", path: "/workspaces", operation_id: "listWorkspaces", summary: "List workspaces", tags: ["Workspaces"], security: #secure, parameters: [#limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "WorkspaceListResponse"}}], #errorResponses]), cli: {command: ["workspaces", "list"], output: {mode: "raw"}}, extensions: #workspaceRead},
	{method: "get", path: "/workspaces/{workspace}/assets", operation_id: "listWorkspaceAssets", summary: "List workspace assets", tags: ["Workspaces"], security: #secure, parameters: [#workspaceParam, {name: "type", in: "query", description: "Filter by asset type.", schema: {type: "string"}}, {name: "q", in: "query", description: "Search query.", schema: {type: "string"}}, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AssetListResponse"}}], #errorResponses]), extensions: #assetRead},
	{method: "get", path: "/workspaces/{workspace}/asset-edges", operation_id: "listWorkspaceAssetEdges", summary: "List workspace asset edges", tags: ["Workspaces"], security: #secure, parameters: [#workspaceParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AssetEdgeListResponse"}}], #errorResponses]), extensions: #assetRead},
]
