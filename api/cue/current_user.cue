package api

import "list"

schemas: {
	"PermissionListResponse": {
		type: "object"
		properties: {
			"workspaceId": {schema: {type: "string"}}
			"permissions": {schema: {type: "array", items: {type: "string"}}}
		}
		required: ["workspaceId", "permissions"]
	}
	"APITokenCreateRequest": {
		type: "object"
		properties: {
			"name": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"permissions": {schema: {type: "array", items: {type: "string"}}}
			"expiresAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["name"]
	}
	"APITokenResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"name": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"permissions": {schema: {type: "array", items: {type: "string"}}}
			"expiresAt": {schema: {type: "string", format: "date-time"}}
			"revokedAt": {schema: {type: "string", format: "date-time"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"lastUsedAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "name", "permissions", "createdAt"]
	}
	"APITokenCreateResponse": {
		type: "object"
		properties: {
			"token": {schema: {type: "string"}}
			"apiToken": {schema: {ref: "APITokenResponse"}}
		}
		required: ["token", "apiToken"]
	}
	"APITokenListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "APITokenResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"SessionResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"expiresAt": {schema: {type: "string", format: "date-time"}}
			"lastSeenAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "createdAt", "expiresAt"]
	}
	"SessionListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "SessionResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#currentUserEndpoints: [
	{method: "get", path: "/me", operation_id: "getCurrentPrincipal", summary: "Get current principal", tags: ["Current User"], security: #secure, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "PrincipalResponse"}}], #errorResponses]), extensions: #workspaceRead},
	{method: "get", path: "/me/permissions", operation_id: "listCurrentPermissions", summary: "List current permissions", tags: ["Current User"], security: #secure, parameters: [{name: "workspace", in: "query", description: "Workspace ID.", schema: {type: "string"}}], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "PermissionListResponse"}}], #errorResponses]), extensions: #workspaceRead},
	{method: "get", path: "/me/api-tokens", operation_id: "listCurrentAPITokens", summary: "List current API tokens", tags: ["Current User"], security: #secure, parameters: [#limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "APITokenListResponse"}}], #errorResponses]), extensions: #tokenManage},
	{method: "post", path: "/me/api-tokens", operation_id: "createCurrentAPIToken", summary: "Create current API token", tags: ["Current User"], security: #secure, request_body: {required: true, schema: {ref: "APITokenCreateRequest"}}, responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "APITokenCreateResponse"}}], #errorResponses]), extensions: #tokenManage},
	{method: "delete", path: "/me/api-tokens/{token}", operation_id: "revokeCurrentAPIToken", summary: "Revoke current API token", tags: ["Current User"], security: #secure, parameters: [#tokenParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses]), extensions: #tokenManage},
	{method: "get", path: "/me/sessions", operation_id: "listCurrentSessions", summary: "List current sessions", tags: ["Current User"], security: #secure, parameters: [#limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "SessionListResponse"}}], #errorResponses]), extensions: #workspaceRead},
	{method: "delete", path: "/me/sessions/{session}", operation_id: "revokeCurrentSession", summary: "Revoke current session", tags: ["Current User"], security: #secure, parameters: [#sessionParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses]), extensions: #workspaceRead},
]
