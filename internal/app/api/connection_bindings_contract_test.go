package api_test

import "testing"

func TestTargetConnectionBindingAPIContract(t *testing.T) {
	spec := managedDataOpenAPISpec(t)
	paths := openAPIMap(t, spec, "paths")
	base := "/api/v1/workspaces/{workspace}/targets/{target}/environments/{environment}/connection-bindings"
	item := base + "/{connection}"

	operations := []struct {
		path      string
		method    string
		id        string
		privilege string
	}{
		{base, "get", "listTargetConnectionBindings", "MANAGE_CONNECTION_METADATA"},
		{base, "post", "createTargetConnectionBinding", "MANAGE_CONNECTION_METADATA"},
		{item, "get", "getTargetConnectionBinding", "MANAGE_CONNECTION_METADATA"},
		{item + "/plan", "post", "planTargetConnectionBindingChange", "MANAGE_CONNECTION_METADATA"},
		{item, "put", "updateTargetConnectionBinding", "MANAGE_CONNECTION_METADATA"},
		{item + "/test", "post", "testTargetConnectionBinding", "TEST_CONNECTION"},
		{item + "/refresh", "post", "refreshTargetConnectionBinding", "TEST_CONNECTION"},
		{item + "/enable", "post", "enableTargetConnectionBinding", "MANAGE_CONNECTION_METADATA"},
		{item + "/disable", "post", "disableTargetConnectionBinding", "MANAGE_CONNECTION_METADATA"},
		{item + "/health", "get", "getTargetConnectionBindingHealth", "VIEW_CONNECTION_HEALTH"},
	}
	for _, want := range operations {
		operation := openAPIOperation(t, paths, want.path, want.method)
		if operation["operationId"] != want.id {
			t.Fatalf("%s %s operation = %#v", want.method, want.path, operation)
		}
		if privilege := openAPIMap(t, operation, "x-authz")["privilege"]; privilege != want.privilege {
			t.Fatalf("%s privilege = %#v, want %q", want.id, privilege, want.privilege)
		}
	}

	for _, action := range []string{"test", "refresh", "enable", "disable"} {
		operation := openAPIOperation(t, paths, item+"/"+action, "post")
		if !operationHasParameter(operation, "header", "Idempotency-Key") {
			t.Fatalf("%s operation is missing Idempotency-Key", action)
		}
	}

	schemas := openAPIMap(t, openAPIMap(t, spec, "components"), "schemas")
	binding := openAPISchema(t, schemas, "TargetConnectionBindingResponse")
	for _, field := range []string{
		"id", "targetId", "logicalConnection", "connectorKind", "authenticationMode",
		"workspaceId", "environment", "endpoint", "enabled", "health", "revision",
	} {
		_ = schemaProperty(t, binding, field)
	}
	for _, forbidden := range []string{"secretValue", "credentials", "password", "token"} {
		if _, exists := openAPIMap(t, binding, "properties")[forbidden]; exists {
			t.Fatalf("connection binding response exposes forbidden field %q", forbidden)
		}
	}

	health := openAPISchema(t, schemas, "TargetConnectionBindingHealthResponse")
	for _, field := range []string{
		"bindingId", "targetId", "logicalConnection", "connectorKind", "workspaceId",
		"environment", "bindingRevision", "health", "hasActivePool",
	} {
		_ = schemaProperty(t, health, field)
	}
	for _, forbidden := range []string{"credential", "secret", "providerError", "rawError"} {
		if _, exists := openAPIMap(t, health, "properties")[forbidden]; exists {
			t.Fatalf("connection health response exposes forbidden field %q", forbidden)
		}
	}
}
