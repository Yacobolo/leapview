package filesystem

import (
	"encoding/json"
	"os"

	projectbundle "github.com/Yacobolo/leapview/internal/project/bundle"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
	"github.com/Yacobolo/leapview/internal/snapshot"
)

type Validator struct{}

type ValidateOptions struct {
	Environment servingstate.Environment
}

func (v Validator) ValidateArtifact(path string, workspaceID servingstate.WorkspaceID, environment servingstate.Environment, servingStateID servingstate.ID) (servingstate.Validation, error) {
	return ValidateArtifactWithOptions(path, workspaceID, servingStateID, ValidateOptions{Environment: environment})
}

func ValidateArtifactWithOptions(path string, workspaceID servingstate.WorkspaceID, servingStateID servingstate.ID, options ValidateOptions) (servingstate.Validation, error) {
	validation, err := projectbundle.ValidateArtifactWithOptions(path, string(workspaceID), string(servingStateID), projectbundle.ValidateOptions{
		Environment: string(options.Environment),
	})
	if err != nil {
		return servingstate.Validation{}, err
	}
	accessJSON, err := json.Marshal(validation.AccessPolicy)
	if err != nil {
		return servingstate.Validation{}, err
	}
	accessPolicy, err := snapshot.DecodeAccessPolicy(accessJSON)
	if err != nil {
		return servingstate.Validation{}, err
	}
	graphJSON, err := json.Marshal(validation.Graph)
	if err != nil {
		return servingstate.Validation{}, err
	}
	graph, err := snapshot.DecodeAssetGraph(graphJSON)
	if err != nil {
		return servingstate.Validation{}, err
	}
	return servingstate.Validation{
		Digest: validation.Digest, ManifestJSON: validation.ManifestJSON, RootDir: validation.RootDir,
		ProjectID: validation.ProjectID, ProjectDigest: validation.ProjectDigest,
		ProjectWorkspaces: append([]string(nil), validation.ProjectWorkspaces...),
		AccessPolicy:      accessPolicy, DashboardPublicationsJSON: validation.DashboardPublicationsJSON,
		ManagedDataRevisions: cloneStringMap(validation.ManagedDataRevisions), Graph: graph,
	}, nil
}

func (Validator) Cleanup(validation servingstate.Validation) error {
	if validation.RootDir == "" {
		return nil
	}
	return os.RemoveAll(validation.RootDir)
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
