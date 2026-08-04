package masking

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileParity(t *testing.T) {
	tests := []struct {
		name  string
		input string
		kind  Kind
		sql   string
	}{
		{name: "default", input: "", kind: Null, sql: "NULL"},
		{name: "null", input: "null", kind: Null, sql: "NULL"},
		{name: "redact", input: "redact", kind: Redact, sql: "'REDACTED'"},
		{name: "redacted alias", input: "redacted", kind: Redact, sql: "'REDACTED'"},
		{name: "zero", input: "zero", kind: Zero, sql: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, err := Compile(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.kind, kind)
			require.Equal(t, tt.sql, kind.SQL())
		})
	}
}

func TestCompileRejectsUnsupportedMask(t *testing.T) {
	_, err := Compile("hash")
	require.ErrorContains(t, err, `unsupported column mask "hash"`)
}
