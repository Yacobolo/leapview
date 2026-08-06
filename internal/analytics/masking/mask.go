// Package masking owns the closed set of column-mask operations and their SQL
// representation. Callers compile untrusted strings once, then carry Kind.
package masking

import (
	"fmt"
	"strings"
)

type Kind string

const (
	Null   Kind = "null"
	Redact Kind = "redact"
	Zero   Kind = "zero"
)

func Compile(value string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(Null):
		return Null, nil
	case string(Redact), "redacted":
		return Redact, nil
	case string(Zero):
		return Zero, nil
	default:
		return "", fmt.Errorf("unsupported column mask %q", value)
	}
}

func (kind Kind) SQL() string {
	switch kind {
	case Null:
		return "NULL"
	case Redact:
		return "'REDACTED'"
	case Zero:
		return "0"
	default:
		panic(fmt.Sprintf("uncompiled column mask %q", kind))
	}
}
