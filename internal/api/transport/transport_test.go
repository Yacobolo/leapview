package transport

import (
	"encoding/json"
	"testing"
)

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
