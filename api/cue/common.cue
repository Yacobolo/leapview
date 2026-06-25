package api

#secure: [{BearerAuth: []}]

schemas: {
	"Error": {
		type: "object"
		properties: {
			"code": {schema: {type: "integer", format: "int32"}}
			"message": {schema: {type: "string"}}
			"details": {schema: {type: "object", additional_properties: {any: true}}}
			"requestId": {schema: {type: "string"}}
		}
		required: ["code", "message"]
	}
	"PageInfo": {
		type: "object"
		properties: {
			"nextCursor": {schema: {type: "string"}}
		}
	}
	"StatusResponse": {
		type: "object"
		properties: {
			"status": {schema: {type: "string"}}
		}
		required: ["status"]
	}
}

#errorResponses: [
	{status_code: 400, description: "bad request", schema: {ref: "Error"}},
	{status_code: 401, description: "unauthorized", schema: {ref: "Error"}},
	{status_code: 403, description: "forbidden", schema: {ref: "Error"}},
	{status_code: 404, description: "not found", schema: {ref: "Error"}},
	{status_code: 409, description: "conflict", schema: {ref: "Error"}},
	{status_code: 429, description: "rate limited", schema: {ref: "Error"}},
	{status_code: 500, description: "internal server error", schema: {ref: "Error"}},
]

#workspaceParam: {name: "workspace", in: "path", required: true, description: "Workspace ID.", schema: {type: "string"}}
#deploymentParam: {name: "deployment", in: "path", required: true, description: "Deployment ID.", schema: {type: "string"}}
#conversationParam: {name: "conversation", in: "path", required: true, description: "Agent conversation ID.", schema: {type: "string"}}
#runParam: {name: "run", in: "path", required: true, description: "Agent run ID.", schema: {type: "string"}}
#principalParam: {name: "principal", in: "path", required: true, description: "Principal ID.", schema: {type: "string"}}
#groupParam: {name: "group", in: "path", required: true, description: "Group ID.", schema: {type: "string"}}
#bindingParam: {name: "binding", in: "path", required: true, description: "Role binding ID.", schema: {type: "string"}}
#tokenParam: {name: "token", in: "path", required: true, description: "API token ID.", schema: {type: "string"}}
#sessionParam: {name: "session", in: "path", required: true, description: "Session ID.", schema: {type: "string"}}
#limitParam: {name: "limit", in: "query", description: "Maximum items to return.", schema: {type: "integer", format: "int32"}}
#cursorParam: {name: "pageToken", in: "query", description: "Opaque pagination cursor.", schema: {type: "string"}}

#workspaceRead: {"x-authz": {mode: "permission", permission: "workspace:read"}}
#assetRead: {"x-authz": {mode: "permission", permission: "asset:read"}}
#deploymentRead: {"x-authz": {mode: "permission", permission: "deployment:read"}}
#deploymentWrite: {"x-authz": {mode: "permission", permission: "deployment:write"}}
#deploymentActivate: {"x-authz": {mode: "permission", permission: "deployment:activate"}}
#rbacRead: {"x-authz": {mode: "permission", permission: "rbac:read"}}
#rbacWrite: {"x-authz": {mode: "permission", permission: "rbac:write"}}
#agentUse: {"x-authz": {mode: "permission", permission: "agent:use"}}
#agentRead: {"x-authz": {mode: "permission", permission: "agent:read"}}
#materializationRun: {"x-authz": {mode: "permission", permission: "materialization:run"}}
#auditRead: {"x-authz": {mode: "permission", permission: "audit:read"}}
#tokenManage: {"x-authz": {mode: "permission", permission: "token:manage"}}
