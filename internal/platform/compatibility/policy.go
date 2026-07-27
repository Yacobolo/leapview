package compatibility

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ReleasedV010Image     = "ghcr.io/yacobolo/libredash@sha256:677caaf256cb3a0d61efd47b289debbd91984976a5a5c4b372196a5d79ce7153"
	LegacyV010Database    = "libredash.db"
	currentDatabase       = "leapview.db"
	freshInstallDirection = "preserve the v0.1.0 state and provision a fresh LeapView instance"
)

var ErrV010FreshInstallOnly = errors.New("v0.1.0 state is fresh-install-only")

// RejectLegacyState prevents the renamed LeapView process from silently
// creating a new database beside a released v0.1.0 LibreDash database.
func RejectLegacyState(databasePath string) error {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" || databasePath == ":memory:" {
		return nil
	}
	databasePath, _, _ = strings.Cut(databasePath, "?")
	if filepath.Base(databasePath) != currentDatabase {
		return nil
	}
	legacyPath := filepath.Join(filepath.Dir(databasePath), LegacyV010Database)
	if _, err := os.Lstat(legacyPath); err == nil {
		return fmt.Errorf("%w: found %s; %s", ErrV010FreshInstallOnly, legacyPath, freshInstallDirection)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect v0.1.0 state marker %s: %w", legacyPath, err)
	}
	return nil
}

// ValidateUpgradeImages rejects the released v0.1.0 image before the
// controller stops a service, writes a checkpoint, or changes its image.
func ValidateUpgradeImages(current, next string) error {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == ReleasedV010Image {
		return fmt.Errorf("%w: the Compose controller cannot upgrade %s in place; %s", ErrV010FreshInstallOnly, current, freshInstallDirection)
	}
	if next == ReleasedV010Image {
		return fmt.Errorf("%w: the Compose controller cannot run current LeapView state with %s", ErrV010FreshInstallOnly, next)
	}
	return nil
}
