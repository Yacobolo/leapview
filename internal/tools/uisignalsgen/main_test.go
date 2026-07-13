package main

import (
	"reflect"
	"testing"

	"github.com/Yacobolo/libredash/internal/dashboard"
)

func TestInteractionSelectionValueGeneratesJSONScalarUnion(t *testing.T) {
	typeOfValue := reflect.TypeOf((*dashboard.InteractionSelectionValue)(nil)).Elem()
	g := &generator{}
	if got := g.tsType(typeOfValue); got != "string | number | boolean | null" {
		t.Fatalf("TypeScript type = %q", got)
	}
	schema, ok := g.schemaForType(typeOfValue).(map[string]any)
	if !ok {
		t.Fatalf("schema = %#v", g.schemaForType(typeOfValue))
	}
	types, ok := schema["type"].([]string)
	if !ok || !reflect.DeepEqual(types, []string{"string", "number", "boolean", "null"}) {
		t.Fatalf("schema types = %#v", schema["type"])
	}
}
