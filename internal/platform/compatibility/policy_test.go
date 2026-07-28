package compatibility

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectLegacyStateRecognizesSQLiteOptionsWithoutMutation(t *testing.T) {
	home := t.TempDir()
	legacyPath := filepath.Join(home, LegacyV010Database)
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(home, "leapview.db") + "?_pragma=busy_timeout(5000)"

	err := RejectLegacyState(currentPath)
	if !errors.Is(err, ErrV010FreshInstallOnly) {
		t.Fatalf("RejectLegacyState() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "leapview.db")); !os.IsNotExist(err) {
		t.Fatalf("compatibility check created current database: %v", err)
	}
}

func TestValidateUpgradeImagesRejectsReleasedV010InEitherDirection(t *testing.T) {
	current := "ghcr.io/yacobolo/leapview@sha256:" + strings.Repeat("a", 64)
	for _, test := range []struct {
		name    string
		current string
		next    string
	}{
		{name: "source", current: ReleasedV010Image, next: current},
		{name: "target", current: current, next: ReleasedV010Image},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateUpgradeImages(test.current, test.next); !errors.Is(err, ErrV010FreshInstallOnly) {
				t.Fatalf("ValidateUpgradeImages() error = %v", err)
			}
		})
	}
}
