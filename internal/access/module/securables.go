package module

import (
	"context"

	"github.com/Yacobolo/leapview/internal/access"
)

type SecurableWriter struct {
	repository access.Repository
}

func (m *Module) Securables() *SecurableWriter {
	repository := m.repositoryValue()
	if repository == nil {
		return nil
	}
	return &SecurableWriter{repository: repository}
}

func (w *SecurableWriter) UpsertSecurableObject(ctx context.Context, object access.ObjectRef, ownerPrincipalID string) (access.SecurableObject, error) {
	return w.repository.UpsertSecurableObject(ctx, object, ownerPrincipalID)
}

func (m *Module) RegisterSecurables(ctx context.Context, objects []access.ObjectRef) error {
	writer := m.Securables()
	if writer == nil {
		return nil
	}
	for _, object := range objects {
		if _, err := writer.UpsertSecurableObject(ctx, object, ""); err != nil {
			return err
		}
	}
	return nil
}
