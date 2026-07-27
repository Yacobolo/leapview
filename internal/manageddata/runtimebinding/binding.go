// Package runtimebinding validates and applies trusted managed-data runtime
// roots through a consumer-owned binding target.
package runtimebinding

import (
	"fmt"
	"path/filepath"
)

type Connection struct {
	ModelID string
	Name    string
}

type Target interface {
	ManagedConnections() []Connection
	BindManagedRoot(connection Connection, root string) error
}

func BindRoots(target Target, roots map[string]string) error {
	if target == nil {
		return fmt.Errorf("managed-data binding target is required")
	}
	for _, connection := range target.ManagedConnections() {
		resolvedRoot := roots[connection.Name]
		if resolvedRoot == "" {
			return fmt.Errorf("semantic model %q managed connection %q has no bound revision", connection.ModelID, connection.Name)
		}
		root := filepath.Clean(resolvedRoot)
		if !filepath.IsAbs(root) {
			return fmt.Errorf("semantic model %q managed connection %q revision root must be absolute", connection.ModelID, connection.Name)
		}
		if err := target.BindManagedRoot(connection, root); err != nil {
			return err
		}
	}
	return nil
}
