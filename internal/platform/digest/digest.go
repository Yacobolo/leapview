// Package digest validates canonical content identities shared by platform
// protocols. It does not assign domain meaning to those identities.
package digest

import (
	"fmt"

	ocidigest "github.com/opencontainers/go-digest"
)

// ValidateSHA256Identity requires a canonical sha256:<lowercase-hex> identity.
func ValidateSHA256Identity(value string) error {
	parsed, err := ocidigest.Parse(value)
	if err != nil {
		return fmt.Errorf("parse content identity: %w", err)
	}
	if parsed.Algorithm() != ocidigest.SHA256 {
		return fmt.Errorf("identity must use the sha256 scheme")
	}
	if parsed.String() != value {
		return fmt.Errorf("SHA-256 identity must be canonical lowercase sha256:<hex>")
	}
	return nil
}
