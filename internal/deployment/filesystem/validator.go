package filesystem

import (
	"os"

	"github.com/Yacobolo/libredash/internal/deployment"
)

type Validator struct{}

func (Validator) ValidateArtifact(path string, workspaceID deployment.WorkspaceID, deploymentID deployment.ID) (deployment.Validation, error) {
	return ValidateArtifact(path, workspaceID, deploymentID)
}

func (Validator) Cleanup(validation deployment.Validation) error {
	if validation.RootDir == "" {
		return nil
	}
	return os.RemoveAll(validation.RootDir)
}
