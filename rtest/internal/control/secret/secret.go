package secret

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Ensure(path string, bytes int) ([]byte, error) {
	if bytes < 32 {
		return nil, errors.New("secret must contain at least 32 bytes")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) < bytes {
			return nil, fmt.Errorf("%s contains only %d bytes", path, len(data))
		}
		return data, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	data = make([]byte, bytes)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return os.ReadFile(path)
		}
		return nil, err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	return data, nil
}
