// Package digest validates canonical content identities shared by platform
// protocols. It does not assign domain meaning to those identities.
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const sha256Prefix = "sha256:"

// ValidateSHA256Identity requires a canonical sha256:<lowercase-hex> identity.
func ValidateSHA256Identity(value string) error {
	if !strings.HasPrefix(value, sha256Prefix) {
		return fmt.Errorf("identity must use the sha256 scheme")
	}
	raw := strings.TrimPrefix(value, sha256Prefix)
	if len(raw) != sha256.Size*2 || strings.ToLower(raw) != raw {
		return fmt.Errorf("SHA-256 must be 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return fmt.Errorf("SHA-256 must be 64 lowercase hexadecimal characters")
	}
	return nil
}
