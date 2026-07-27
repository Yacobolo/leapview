package digest

import (
	"strings"
	"testing"
)

func TestValidateSHA256Identity(t *testing.T) {
	t.Parallel()

	valid := "sha256:" + strings.Repeat("a", 64)
	if err := ValidateSHA256Identity(valid); err != nil {
		t.Fatalf("ValidateSHA256Identity(%q) error = %v", valid, err)
	}

	for _, value := range []string{
		"",
		strings.Repeat("a", 64),
		"sha512:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("A", 64),
		"sha256:" + strings.Repeat("g", 64),
		"sha256:" + strings.Repeat("a", 63),
		"sha256:" + strings.Repeat("a", 65),
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if err := ValidateSHA256Identity(value); err == nil {
				t.Fatalf("ValidateSHA256Identity(%q) error = nil", value)
			}
		})
	}
}
