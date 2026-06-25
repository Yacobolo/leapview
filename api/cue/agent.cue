package api

import "list"

schemas: {
	"AgentConversationCreateRequest": {
		type: "object"
		properties: {
			"title": {schema: {type: "string"}}
		}
	}
	"AgentConversationPatchRequest": {
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
			"status": {schema: {type: "string", enum: ["active", "archived"]}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"updatedAt": {schema: {type: "string", format: "date-time"}}
			"archivedAt": {schema: {type: "string", format: "date-time"}}
			"messageCount": {schema: {type: "integer", format: "int32"}}
			"lastMessageText": {schema: {type: "string"}}
			"titlePending": {schema: {type: "boolean"}}
		}
		required: ["id", "workspaceId", "principalId", "title", "status", "createdAt", "updatedAt"]
	}
	"AgentConversationListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AgentConversationResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"AgentRunResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"conversationId": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"principalId": {schema: {type: "string"}}
			"status": {schema: {type: "string"}}
			"model": {schema: {type: "string"}}
			"stopReason": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"startedAt": {schema: {type: "string", format: "date-time"}}
			"completedAt": {schema: {type: "string", format: "date-time"}}
			"error": {schema: {type: "string"}}
		}
		required: ["id", "conversationId", "workspaceId", "principalId", "status", "createdAt"]
	}
	"AgentRunListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AgentRunResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"AgentMessageResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"runId": {schema: {type: "string"}}
			"seq": {schema: {type: "integer", format: "int64"}}
			"role": {schema: {type: "string"}}
			"contentText": {schema: {type: "string"}}
			"content": {schema: {type: "object", additional_properties: {any: true}}}
			"toolCallId": {schema: {type: "string"}}
			"toolName": {schema: {type: "string"}}
			"isError": {schema: {type: "boolean"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "seq", "role", "createdAt"]
	}
	"AgentMessageListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AgentMessageResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
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
			"seq": {schema: {type: "integer", format: "int64"}}
			"eventType": {schema: {type: "string"}}
			"severity": {schema: {type: "string", enum: ["debug", "info", "warning", "error"]}}
			"payload": {schema: {type: "object", additional_properties: {any: true}}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "runId", "seq", "eventType", "severity", "payload", "createdAt"]
	}
	"AgentEventListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AgentEventResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#agentEndpoints: [
	{method: "post", path: "/workspaces/{workspace}/agent/conversations", operation_id: "createAgentConversation", summary: "Create an agent conversation", tags: ["Agent"], security: #secure, parameters: [#workspaceParam], request_body: {required: true, schema: {ref: "AgentConversationCreateRequest"}}, responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "AgentConversationResponse"}}], #errorResponses]), extensions: #agentUse},
	{method: "get", path: "/workspaces/{workspace}/agent/conversations", operation_id: "listAgentConversations", summary: "List agent conversations", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentConversationListResponse"}}], #errorResponses]), cli: {command: ["agent", "conversations"], output: {mode: "raw"}}, extensions: #agentRead},
	{method: "get", path: "/workspaces/{workspace}/agent/conversations/{conversation}", operation_id: "getAgentConversation", summary: "Get an agent conversation", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentConversationResponse"}}], #errorResponses]), extensions: #agentRead},
	{method: "patch", path: "/workspaces/{workspace}/agent/conversations/{conversation}", operation_id: "updateAgentConversation", summary: "Update an agent conversation", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam], request_body: {required: true, schema: {ref: "AgentConversationPatchRequest"}}, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentConversationResponse"}}], #errorResponses]), extensions: #agentUse},
	{method: "delete", path: "/workspaces/{workspace}/agent/conversations/{conversation}", operation_id: "archiveAgentConversation", summary: "Archive an agent conversation", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentConversationResponse"}}], #errorResponses]), extensions: #agentUse},
	{method: "get", path: "/workspaces/{workspace}/agent/conversations/{conversation}/messages", operation_id: "listAgentMessages", summary: "List agent messages", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentMessageListResponse"}}], #errorResponses]), extensions: #agentRead},
	{method: "post", path: "/workspaces/{workspace}/agent/conversations/{conversation}/turns", operation_id: "createAgentTurn", summary: "Create an agent turn", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam], request_body: {required: true, schema: {ref: "AgentTurnRequest"}}, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentTurnResponse"}}], #errorResponses]), extensions: #agentUse},
	{method: "get", path: "/workspaces/{workspace}/agent/conversations/{conversation}/runs", operation_id: "listAgentRuns", summary: "List agent runs", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentRunListResponse"}}], #errorResponses]), extensions: #agentRead},
	{method: "get", path: "/workspaces/{workspace}/agent/conversations/{conversation}/runs/{run}", operation_id: "getAgentRun", summary: "Get an agent run", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam, #runParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentRunResponse"}}], #errorResponses]), extensions: #agentRead},
	{method: "get", path: "/workspaces/{workspace}/agent/conversations/{conversation}/runs/{run}/events", operation_id: "listAgentEvents", summary: "List agent run events", tags: ["Agent"], security: #secure, parameters: [#workspaceParam, #conversationParam, #runParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AgentEventListResponse"}}], #errorResponses]), extensions: #agentRead},
]
