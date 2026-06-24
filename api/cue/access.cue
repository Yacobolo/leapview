package api

import "list"

schemas: {
	"PrincipalResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"email": {schema: {type: "string"}}
			"displayName": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"updatedAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "email", "displayName", "createdAt", "updatedAt"]
	}
	"PrincipalListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "PrincipalResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"PrincipalPatchRequest": {
		type: "object"
		properties: {
			"displayName": {schema: {type: "string"}}
		}
	}
	"RoleResponse": {
		type: "object"
		properties: {
			"name": {schema: {type: "string"}}
			"permissions": {schema: {type: "array", items: {type: "string"}}}
		}
		required: ["name", "permissions"]
	}
	"RoleListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "RoleResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"GroupCreateRequest": {
		type: "object"
		properties: {
			"name": {schema: {type: "string"}}
			"displayName": {schema: {type: "string"}}
		}
		required: ["name"]
	}
	"GroupPatchRequest": {
		type: "object"
		properties: {
			"displayName": {schema: {type: "string"}}
		}
	}
	"GroupResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"name": {schema: {type: "string"}}
			"displayName": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
			"updatedAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "name", "displayName", "createdAt", "updatedAt"]
	}
	"GroupListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "GroupResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"GroupMemberListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "PrincipalResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
	"RoleBindingRequest": {
		type: "object"
		properties: {
			"subjectType": {schema: {type: "string", enum: ["principal", "group"]}}
			"subjectId": {schema: {type: "string"}}
			"role": {schema: {type: "string"}}
		}
		required: ["subjectType", "subjectId", "role"]
	}
	"RoleBindingResponse": {
		type: "object"
		properties: {
			"id": {schema: {type: "string"}}
			"workspaceId": {schema: {type: "string"}}
			"subjectType": {schema: {type: "string", enum: ["principal", "group"]}}
			"subjectId": {schema: {type: "string"}}
			"email": {schema: {type: "string"}}
			"displayName": {schema: {type: "string"}}
			"role": {schema: {type: "string"}}
			"createdAt": {schema: {type: "string", format: "date-time"}}
		}
		required: ["id", "workspaceId", "subjectType", "subjectId", "role", "createdAt"]
	}
	"RoleBindingListResponse": {
		type: "object"
		properties: {
			"items": {schema: {type: "array", items: {ref: "RoleBindingResponse"}}}
			"page": {schema: {ref: "PageInfo"}}
		}
		required: ["items", "page"]
	}
}

#accessEndpoints: [
	{method: "get", path: "/principals", operation_id: "listPrincipals", summary: "List principals", tags: ["Access"], security: #secure, parameters: [{name: "email", in: "query", description: "Filter by email.", schema: {type: "string"}}, {name: "q", in: "query", description: "Search query.", schema: {type: "string"}}, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "PrincipalListResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "get", path: "/principals/{principal}", operation_id: "getPrincipal", summary: "Get a principal", tags: ["Access"], security: #secure, parameters: [#principalParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "PrincipalResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "patch", path: "/principals/{principal}", operation_id: "updatePrincipal", summary: "Update a principal", tags: ["Access"], security: #secure, parameters: [#principalParam], request_body: {required: true, schema: {ref: "PrincipalPatchRequest"}}, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "PrincipalResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "get", path: "/workspaces/{workspace}/roles", operation_id: "listWorkspaceRoles", summary: "List workspace roles", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "RoleListResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "get", path: "/workspaces/{workspace}/groups", operation_id: "listGroups", summary: "List groups", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "GroupListResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "post", path: "/workspaces/{workspace}/groups", operation_id: "createGroup", summary: "Create a group", tags: ["Access"], security: #secure, parameters: [#workspaceParam], request_body: {required: true, schema: {ref: "GroupCreateRequest"}}, responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "GroupResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "get", path: "/workspaces/{workspace}/groups/{group}", operation_id: "getGroup", summary: "Get a group", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #groupParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "GroupResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "patch", path: "/workspaces/{workspace}/groups/{group}", operation_id: "updateGroup", summary: "Update a group", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #groupParam], request_body: {required: true, schema: {ref: "GroupPatchRequest"}}, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "GroupResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "delete", path: "/workspaces/{workspace}/groups/{group}", operation_id: "deleteGroup", summary: "Delete a group", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #groupParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "get", path: "/workspaces/{workspace}/groups/{group}/members", operation_id: "listGroupMembers", summary: "List group members", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #groupParam, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "GroupMemberListResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "put", path: "/workspaces/{workspace}/groups/{group}/members/{principal}", operation_id: "addGroupMember", summary: "Add a group member", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #groupParam, #principalParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "delete", path: "/workspaces/{workspace}/groups/{group}/members/{principal}", operation_id: "removeGroupMember", summary: "Remove a group member", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #groupParam, #principalParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "get", path: "/workspaces/{workspace}/role-bindings", operation_id: "listRoleBindings", summary: "List role bindings", tags: ["Access"], security: #secure, parameters: [#workspaceParam, {name: "subjectType", in: "query", description: "Filter by subject type.", schema: {type: "string"}}, {name: "subjectId", in: "query", description: "Filter by subject ID.", schema: {type: "string"}}, {name: "role", in: "query", description: "Filter by role.", schema: {type: "string"}}, #limitParam, #cursorParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "RoleBindingListResponse"}}], #errorResponses]), extensions: #rbacRead},
	{method: "post", path: "/workspaces/{workspace}/role-bindings", operation_id: "createRoleBinding", summary: "Create a role binding", tags: ["Access"], security: #secure, parameters: [#workspaceParam], request_body: {required: true, schema: {ref: "RoleBindingRequest"}}, responses: list.Concat([[{status_code: 201, description: "created", schema: {ref: "RoleBindingResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "patch", path: "/workspaces/{workspace}/role-bindings/{binding}", operation_id: "updateRoleBinding", summary: "Update a role binding", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #bindingParam], request_body: {required: true, schema: {ref: "RoleBindingRequest"}}, responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "RoleBindingResponse"}}], #errorResponses]), extensions: #rbacWrite},
	{method: "delete", path: "/workspaces/{workspace}/role-bindings/{binding}", operation_id: "deleteRoleBinding", summary: "Delete a role binding", tags: ["Access"], security: #secure, parameters: [#workspaceParam, #bindingParam], responses: list.Concat([[{status_code: 200, description: "ok", schema: {ref: "StatusResponse"}}], #errorResponses]), extensions: #rbacWrite},
]
