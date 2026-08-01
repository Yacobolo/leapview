package credentialfiles

import (
	"errors"
	"os"
	"path/filepath"
)

type Files struct {
	Root        string
	CA          string
	Certificate string
	Key         string
}

func Write(ca, certificate, key []byte) (Files, error) {
	if len(ca) == 0 || len(certificate) == 0 || len(key) == 0 {
		return Files{}, errors.New("CA, certificate, and private key are required")
	}
	root, err := os.MkdirTemp("", "rtest-credentials-*")
	if err != nil {
		return Files{}, err
	}
	files := Files{Root: root, CA: filepath.Join(root, "ca.pem"), Certificate: filepath.Join(root, "cert.pem"), Key: filepath.Join(root, "key.pem")}
	for path, data := range map[string][]byte{files.CA: ca, files.Certificate: certificate, files.Key: key} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			_ = files.Cleanup()
			return Files{}, err
		}
	}
	return files, nil
}

func (f Files) Cleanup() error {
	if f.Root == "" {
		return nil
	}
	return os.RemoveAll(f.Root)
}
