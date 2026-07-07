package securefs

import (
	"fmt"
	"os"
)

const (
	PrivateDirMode  os.FileMode = 0o700
	PrivateFileMode os.FileMode = 0o600
)

func EnsurePrivateDir(path string) error {
	if path == "" {
		return fmt.Errorf("private directory path is required")
	}
	if err := os.MkdirAll(path, PrivateDirMode); err != nil {
		return err
	}
	return os.Chmod(path, PrivateDirMode)
}
