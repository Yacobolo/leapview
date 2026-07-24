package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	toon "github.com/toon-format/toon-go"
)

func TestConvertPreservesToolResultStructure(t *testing.T) {
	input := map[string]any{
		"items":   []any{map[string]any{"name": "Sales", "count": int64(42)}},
		"hasMore": false,
	}
	encoded, err := toon.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := convert(bytes.NewReader(encoded), &output); err != nil {
		t.Fatalf("convert(): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	want := map[string]any{
		"items":   []any{map[string]any{"name": "Sales", "count": float64(42)}},
		"hasMore": false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("converted result = %#v, want %#v", got, want)
	}
}
