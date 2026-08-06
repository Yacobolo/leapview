package transport

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONNormalizesTimestampsAndRequiredCollections(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteJSON(recorder, 200, map[string]any{
		"createdAt":  "2026-01-02 03:04:05",
		"items":      nil,
		"bindings":   nil,
		"workspaces": nil,
		"optional":   nil,
	})
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["createdAt"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("createdAt = %#v", body["createdAt"])
	}
	if items, ok := body["items"].([]any); !ok || items == nil {
		t.Fatalf("items = %#v, want empty array", body["items"])
	}
	for _, field := range []string{"bindings", "workspaces"} {
		if value, ok := body[field].([]any); !ok || value == nil {
			t.Fatalf("%s = %#v, want empty array", field, body[field])
		}
	}
	if body["optional"] != nil {
		t.Fatalf("optional = %#v, want null", body["optional"])
	}
}

func TestKeysetPagePreservesEmptyArray(t *testing.T) {
	items, next, err := KeysetPage([]string(nil), nil, nil, func(value string) string { return value })
	encoded, marshalErr := json.Marshal(items)
	if err != nil || marshalErr != nil || next != nil || string(encoded) != "[]" {
		t.Fatalf("empty page = %s, next=%v, error=%v/%v; want []", encoded, next, err, marshalErr)
	}
}

func TestKeysetPageRejectsCursorFromAnotherCollection(t *testing.T) {
	items := []string{"a", "b", "c"}
	_, token, err := KeysetPage(items, int32Pointer(1), nil, func(value string) string { return value })
	if err != nil || token == nil {
		t.Fatalf("first page token = %v, %v", token, err)
	}
	if _, _, err := KeysetPage([]string{"x", "y"}, nil, token, func(value string) string { return value }); err == nil {
		t.Fatal("expected unavailable cursor key to fail")
	}
}

func int32Pointer(value int32) *int32 { return &value }
