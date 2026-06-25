package api

import "list"

schemas: {
	"AuditEventResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"principalId": {schema: {type: "string"}}
			"action": {schema: {type: "string"}}
			"targetType": {schema: {type: "string"}}
			"targetId": {schema: {type: "string"}}
			"metadata": {schema: {type: "object", additional_properties: {any: true}}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "workspaceId", "action", "targetType", "targetId", "metadata", "createdAt"]
	}
	"AuditEventListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "AuditEventResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#auditEndpoints: [
	{method: "get", path: "/workspaces/{workspace}/audit-events", operation_id: "listAuditEvents", summary: "List workspace audit events", tags: ["Audit"], security: #secure, parameters: [#workspaceParam, {name: "actor", in: "query", description: "Filter by principal ID.", schema: {type: "string"}}, {name: "action", in: "query", description: "Filter by action.", schema: {type: "string"}}, {name: "targetType", in: "query", description: "Filter by target type.", schema: {type: "string"}}, {name: "targetId", in: "query", description: "Filter by target ID.", schema: {type: "string"}}, {name: "from", in: "query", description: "Filter from timestamp.", schema: {type: "string", format: "date-time"}}, {name: "to", in: "query", description: "Filter to timestamp.", schema: {type: "string", format: "date-time"}}, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "AuditEventListResponse"}}], #errorResponses]), extensions: #auditRead},
]
