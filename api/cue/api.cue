package api

import "list"

schema_version: "v1"

api: {
	base_path: "/api"
}

info: {
	title:       "LibreDash Headless API"
	version:     "1.0.0"
	description: "Headless API for LibreDash workspaces, deployments, RBAC, and agent operations."
}

openapi: {
	version:   "3.0.0"
	tag_order: ["Workspaces", "Agent", "RBAC", "Deployments"]
	security_schemes: {
		BearerAuth: {
			type:   "http"
			scheme: "bearer"
		}
	}
}

tags: [
	{name: "Workspaces", description: "Workspace and lineage discovery."},
	{name: "Agent", description: "Headless agent conversation operations."},
	{name: "RBAC", description: "Workspace role and role-binding operations."},
	{name: "Deployments", description: "Dashboard-as-code deployment operations."},
]

schemas: {
	"Error": {
		type: "object"
		properties: {
			"code": {schema: {type: "integer", format: "int32"}}
			"message": {schema: {type: "string"}}
			"error": {schema: {type: "string"}}
		}
		required: ["code", "message"]
	}
	"StatusResponse": {
		type: "object"
		properties: {
			"status": {schema: {type: "string"}}
		}
		required: ["status"]
	}
	"PrincipalResponse": {
		type: "object"
		properties: {
			"principalId": {schema: {type: "string"}}
		}
		required: ["principalId"]
	}
	"WorkspaceResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"title": {schema: {type: "string"}}
			"description": {schema: {type: "string"}}
			"activeDeploymentId": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string"}}
			"updatedAt": {schema: {type: "string"}}
		}
		required: ["id", "title", "description", "createdAt", "updatedAt"]
	}
	"WorkspaceResponseList": {
		type: "array"
		items: {ref: "WorkspaceResponse"}
	}
	"AssetResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"deploymentId": {schema: {type: "string"}}
			"type": {schema: {type: "string"}}
			"key": {schema: {type: "string"}}
			"parentId": {schema: {type: "string"}}
			"title": {schema: {type: "string"}}
			"description": {schema: {type: "string"}}
			"meta": {schema: {type: "object", additional_properties: {any: true}}}
			"href": {schema: {type: "string"}}
		}
		required: ["id", "workspaceId", "deploymentId", "type", "key", "title", "description"]
	}
	"AssetResponseList": {
		type: "array"
		items: {ref: "AssetResponse"}
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
	"AssetEdgeResponseList": {
		type: "array"
		items: {ref: "AssetEdgeResponse"}
	}
	"RoleResponse": {
		type: "object"
		properties: {
			"name": {schema: {type: "string"}}
			"permissions": {schema: {type: "array", items: {type: "string"}}}
		}
		required: ["name", "permissions"]
	}
	"RoleResponseList": {
		type: "array"
		items: {ref: "RoleResponse"}
	}
	"RoleBindingResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"principalId": {schema: {type: "string"}}
			"email": {schema: {type: "string"}}
			"displayName": {schema: {type: "string"}}
			"role": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string"}}
		}
		required: ["id", "workspaceId", "principalId", "email", "displayName", "role", "createdAt"]
	}
	"RoleBindingResponseList": {
		type: "array"
		items: {ref: "RoleBindingResponse"}
	}
	"RoleBindingUpsertRequest": {
		type: "object"
		properties: {
			"email": {schema: {type: "string"}}
			"displayName": {schema: {type: "string"}}
			"role": {schema: {type: "string"}}
		}
		required: ["email", "role"]
	}
	"DeploymentCreateRequest": {
		type: "object"
		properties: {
			"workspaceId": {schema: {type: "string"}}
			"title": {schema: {type: "string"}}
			"description": {schema: {type: "string"}}
		}
	}
	"DeploymentArtifactResponse": {
		type: "object"
		properties: {
			"deploymentId": {schema: {type: "string"}}
			"sizeBytes": {schema: {type: "int64"}}
		}
		required: ["deploymentId", "sizeBytes"]
	}
	"DeploymentResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"status": {schema: {type: "string"}}
			"digest": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string"}}
			"activatedAt": {schema: {type: "string"}}
			"error": {schema: {type: "string"}}
		}
		required: ["id", "workspaceId", "status", "digest", "createdAt"]
	}
	"DeploymentResponseList": {
		type: "array"
		items: {ref: "DeploymentResponse"}
	}
	"AgentConversationCreateRequest": {
		type: "object"
		properties: {
			"title": {schema: {type: "string"}}
		}
	}
	"AgentConversationResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"principalId": {schema: {type: "string"}}
			"title": {schema: {type: "string"}}
			"status": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string"}}
			"updatedAt": {schema: {type: "string"}}
			"archivedAt": {schema: {type: "string"}}
			"messageCount": {schema: {type: "integer", format: "int32"}}
			"lastMessageText": {schema: {type: "string"}}
			"titlePending": {schema: {type: "boolean"}}
		}
		required: ["id", "workspaceId", "principalId", "title", "status", "createdAt", "updatedAt"]
	}
	"AgentConversationResponseList": {
		type: "array"
		items: {ref: "AgentConversationResponse"}
	}
	"AgentMessageResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"runId": {schema: {type: "string"}}
			"seq": {schema: {type: "int64"}}
			"role": {schema: {type: "string"}}
			"contentText": {schema: {type: "string"}}
			"contentJson": {schema: {type: "string"}}
			"toolCallId": {schema: {type: "string"}}
			"toolName": {schema: {type: "string"}}
			"isError": {schema: {type: "boolean"}}
			"createdAt": {schema: {type: "string"}}
		}
		required: ["id", "seq", "role", "createdAt"]
	}
	"AgentMessageResponseList": {
		type: "array"
		items: {ref: "AgentMessageResponse"}
	}
	"AgentTurnRequest": {
		type: "object"
		properties: {
			"input": {schema: {type: "string"}}
			"correlationId": {schema: {type: "string"}}
		}
		required: ["input"]
	}
	"AgentTurnResponse": {
		type: "object"
		properties: {
			"conversationId": {schema: {type: "string"}}
			"runId": {schema: {type: "string"}}
			"stopReason": {schema: {type: "string"}}
			"content": {schema: {type: "string"}}
		}
		required: ["conversationId", "runId", "stopReason", "content"]
	}
	"AgentEventResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"runId": {schema: {type: "string"}}
			"seq": {schema: {type: "int64"}}
			"eventType": {schema: {type: "string"}}
			"severity": {schema: {type: "string"}}
			"payloadJson": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string"}}
		}
		required: ["id", "runId", "seq", "eventType", "severity", "payloadJson", "createdAt"]
	}
	"AgentEventResponseList": {
		type: "array"
		items: {ref: "AgentEventResponse"}
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

#workspaceParam: {
	name:        "workspace"
	in:          "path"
	required:    true
	description: "Workspace ID."
	schema: {type: "string"}
}

#deploymentParam: {
	name:        "deployment"
	in:          "path"
	required:    true
	description: "Deployment ID."
	schema: {type: "string"}
}

#conversationParam: {
	name:        "conversation"
	in:          "path"
	required:    true
	description: "Agent conversation ID."
	schema: {type: "string"}
}

#runParam: {
	name:        "run"
	in:          "path"
	required:    true
	description: "Agent run ID."
	schema: {type: "string"}
}

#principalParam: {
	name:        "principal"
	in:          "path"
	required:    true
	description: "Principal ID."
	schema: {type: "string"}
}

#dashboardView: {
	"x-authz": {
		mode:       "permission"
		permission: "dashboard:view"
	}
}

#rbacManage: {
	"x-authz": {
		mode:       "permission"
		permission: "rbac:manage"
	}
}

#deploymentCreate: {
	"x-authz": {
		mode:       "permission"
		permission: "deployment:create"
	}
}

#deploymentActivate: {
	"x-authz": {
		mode:       "permission"
		permission: "deployment:activate"
	}
}

#deploymentRollback: {
	"x-authz": {
		mode:       "permission"
		permission: "deployment:rollback"
	}
}

endpoints: [
	{
		method:       "get"
		path:         "/workspaces"
		operation_id: "listWorkspaces"
		summary:      "List workspaces"
		tags:         ["Workspaces"]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "WorkspaceResponseList"}}], #errorResponses])
		cli: {
			command: ["workspaces", "list"]
			output: {mode: "raw"}
		}
		extensions: #dashboardView
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/assets"
		operation_id: "listWorkspaceAssets"
		summary:      "List workspace assets"
		tags:         ["Workspaces"]
		parameters: [
			#workspaceParam,
			{name: "type", in: "query", description: "Filter by asset type.", schema: {type: "string"}},
			{name: "q", in: "query", description: "Search query.", schema: {type: "string"}},
		]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AssetResponseList"}}], #errorResponses])
		extensions: #dashboardView
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/asset-edges"
		operation_id: "listWorkspaceAssetEdges"
		summary:      "List workspace asset edges"
		tags:         ["Workspaces"]
		parameters: [#workspaceParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AssetEdgeResponseList"}}], #errorResponses])
		extensions: #dashboardView
	},
	{
		method:       "post"
		path:         "/workspaces/{workspace}/agent/conversations"
		operation_id: "createAgentConversation"
		summary:      "Create an agent conversation"
		tags:         ["Agent"]
		parameters: [#workspaceParam]
		request_body: {required: true, schema: {ref: "AgentConversationCreateRequest"}}
		responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "AgentConversationResponse"}}], #errorResponses])
		extensions: #dashboardView
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/agent/conversations"
		operation_id: "listAgentConversations"
		summary:      "List agent conversations"
		tags:         ["Agent"]
		parameters: [#workspaceParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentConversationResponseList"}}], #errorResponses])
		cli: {
			command: ["agent", "conversations"]
			output: {mode: "raw"}
		}
		extensions: #dashboardView
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/agent/conversations/{conversation}/messages"
		operation_id: "listAgentMessages"
		summary:      "List agent messages"
		tags:         ["Agent"]
		parameters: [#workspaceParam, #conversationParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentMessageResponseList"}}], #errorResponses])
		extensions: #dashboardView
	},
	{
		method:       "post"
		path:         "/workspaces/{workspace}/agent/conversations/{conversation}/turns"
		operation_id: "createAgentTurn"
		summary:      "Create an agent turn"
		tags:         ["Agent"]
		parameters: [#workspaceParam, #conversationParam]
		request_body: {required: true, schema: {ref: "AgentTurnRequest"}}
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentTurnResponse"}}], #errorResponses])
		extensions: #dashboardView
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/agent/runs/{run}/events"
		operation_id: "listAgentEvents"
		summary:      "List agent run events"
		tags:         ["Agent"]
		parameters: [#workspaceParam, #runParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentEventResponseList"}}], #errorResponses])
		extensions: #dashboardView
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/roles"
		operation_id: "listWorkspaceRoles"
		summary:      "List workspace roles"
		tags:         ["RBAC"]
		parameters: [#workspaceParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "RoleResponseList"}}], #errorResponses])
		extensions: #rbacManage
	},
	{
		method:       "get"
		path:         "/workspaces/{workspace}/role-bindings"
		operation_id: "listRoleBindings"
		summary:      "List role bindings"
		tags:         ["RBAC"]
		parameters: [#workspaceParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "RoleBindingResponseList"}}], #errorResponses])
		extensions: #rbacManage
	},
	{
		method:       "post"
		path:         "/workspaces/{workspace}/role-bindings"
		operation_id: "upsertRoleBinding"
		summary:      "Upsert a role binding"
		tags:         ["RBAC"]
		parameters: [#workspaceParam]
		request_body: {required: true, schema: {ref: "RoleBindingUpsertRequest"}}
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "PrincipalResponse"}}], #errorResponses])
		extensions: #rbacManage
	},
	{
		method:       "delete"
		path:         "/workspaces/{workspace}/role-bindings/{principal}"
		operation_id: "deleteRoleBinding"
		summary:      "Delete a role binding"
		tags:         ["RBAC"]
		parameters: [#workspaceParam, #principalParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses])
		extensions: #rbacManage
	},
	{
		method:       "post"
		path:         "/deployments"
		operation_id: "createDeployment"
		summary:      "Create a deployment"
		tags:         ["Deployments"]
		request_body: {required: false, schema: {ref: "DeploymentCreateRequest"}}
		responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "DeploymentResponse"}}], #errorResponses])
		extensions: #deploymentCreate
	},
	{
		method:       "get"
		path:         "/deployments"
		operation_id: "listDeployments"
		summary:      "List deployments"
		tags:         ["Deployments"]
		parameters: [{name: "workspace", in: "query", description: "Workspace ID.", schema: {type: "string"}}]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponseList"}}], #errorResponses])
		cli: {
			command: ["deployments", "list"]
			output: {mode: "raw"}
		}
		extensions: #deploymentCreate
	},
	{
		method:       "get"
		path:         "/deployments/{deployment}"
		operation_id: "getDeployment"
		summary:      "Get a deployment"
		tags:         ["Deployments"]
		parameters: [#deploymentParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses])
		extensions: #deploymentCreate
	},
	{
		method:       "put"
		path:         "/deployments/{deployment}/artifact"
		operation_id: "uploadDeploymentArtifact"
		summary:      "Upload a deployment artifact"
		tags:         ["Deployments"]
		parameters: [#deploymentParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentArtifactResponse"}}], #errorResponses])
		extensions: {
			"x-authz": {
				mode:       "permission"
				permission: "deployment:create"
			}
			"x-libredash-dispatch": "raw-body"
		}
	},
	{
		method:       "post"
		path:         "/deployments/{deployment}/validate"
		operation_id: "validateDeployment"
		summary:      "Validate a deployment"
		tags:         ["Deployments"]
		parameters: [#deploymentParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses])
		extensions: #deploymentCreate
	},
	{
		method:       "post"
		path:         "/deployments/{deployment}/activate"
		operation_id: "activateDeployment"
		summary:      "Activate a deployment"
		tags:         ["Deployments"]
		parameters: [#deploymentParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses])
		extensions: #deploymentActivate
	},
	{
		method:       "post"
		path:         "/deployments/{deployment}/rollback"
		operation_id: "rollbackDeployment"
		summary:      "Rollback to a deployment"
		tags:         ["Deployments"]
		parameters: [#deploymentParam]
		responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "DeploymentResponse"}}], #errorResponses])
		cli: {
			command: ["deployments", "rollback"]
			output: {mode: "raw"}
		}
		extensions: #deploymentRollback
	},
]
