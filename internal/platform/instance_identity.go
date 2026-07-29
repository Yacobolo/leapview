package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/flidai/leapview/internal/platform/db"
)

const instanceIDSetting = "instance.id"

var instanceIDPattern = regexp.MustCompile(`^instance_[0-9a-f]{32}$`)

// EnsureInstanceID returns the immutable opaque identity of this installation,
// creating it exactly once when a platform store is first initialized.
func (s *Store) EnsureInstanceID(ctx context.Context) (string, error) {
	current, err := s.GetSetting(ctx, instanceIDSetting)
	if err == nil {
		return validateInstanceID(current)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read instance id: %w", err)
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate instance id: %w", err)
	}
	candidate := "instance_" + hex.EncodeToString(random)
	if err := s.q.InsertPlatformSettingIfMissing(ctx, db.InsertPlatformSettingIfMissingParams{
		Key: instanceIDSetting, Value: candidate,
	}); err != nil {
		return "", fmt.Errorf("persist instance id: %w", err)
	}
	current, err = s.GetSetting(ctx, instanceIDSetting)
	if err != nil {
		return "", fmt.Errorf("verify instance id: %w", err)
	}
	return validateInstanceID(current)
}

func validateInstanceID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !instanceIDPattern.MatchString(value) {
		return "", fmt.Errorf("stored instance id is invalid")
	}
	return value, nil
}
