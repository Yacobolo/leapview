package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/spf13/cobra"
)

// DeployOptions are the Project-owned inputs to deployment orchestration.
type DeployOptions struct {
	ProjectPath string
	Revisions   map[string]string
	Credentials cliapi.Credentials
	Environment string
}

// DeployOperations performs the cross-capability release/deployment workflow
// assembled by the application.
type DeployOperations interface {
	Deploy(context.Context, DeployOptions, io.Writer) error
}

// DeployCommand constructs the project deployment command.
func DeployCommand(ctx context.Context, client cliapi.Client, operations DeployOperations, forbiddenWorkspaceID string) *cobra.Command {
	values := DeployOptions{ProjectPath: filepath.Join("dashboards", "leapview.yaml")}
	var revisions []string
	command := &cobra.Command{
		Use:   "deploy",
		Short: "Atomically deploy a configuration-as-code project",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if strings.TrimSpace(forbiddenWorkspaceID) != "" {
				return fmt.Errorf("deploy is project-wide and does not accept --workspace")
			}
			if client == nil {
				return fmt.Errorf("Project CLI API client is required")
			}
			if operations == nil {
				return fmt.Errorf("Project deploy operations are required")
			}
			credentials, err := client.Resolve(ctx, values.Credentials)
			if err != nil {
				return err
			}
			pins, err := parseManagedRevisionPins(revisions)
			if err != nil {
				return err
			}
			values.Credentials = credentials
			values.Revisions = pins
			return operations.Deploy(ctx, values, command.OutOrStdout())
		},
	}
	command.Flags().StringVar(&values.Credentials.Target, "target", "", "LeapView server URL")
	command.Flags().StringVar(&values.Credentials.Token, "token", "", "API token")
	command.Flags().StringVar(&values.ProjectPath, "project", values.ProjectPath, "project path")
	command.Flags().StringVar(&values.Environment, "environment", "", "assert the target instance environment")
	command.Flags().StringArrayVar(&revisions, "revision", nil, "managed revision pin as connection=sha256:<digest> (repeatable)")
	return command
}

func parseManagedRevisionPins(values []string) (map[string]string, error) {
	pins := make(map[string]string, len(values))
	for _, value := range values {
		name, revision, ok := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		revision = strings.TrimSpace(revision)
		if !ok || name == "" || !canonicalRevisionID(revision) {
			return nil, fmt.Errorf("revision must use connection=sha256:<64 lowercase hex>")
		}
		if _, duplicate := pins[name]; duplicate {
			return nil, fmt.Errorf("duplicate revision for managed connection %q", name)
		}
		pins[name] = revision
	}
	return pins, nil
}

func canonicalRevisionID(value string) bool {
	const prefix = "sha256:"
	if len(value) != len(prefix)+sha256.Size*2 || !strings.HasPrefix(value, prefix) {
		return false
	}
	digest := value[len(prefix):]
	if strings.ToLower(digest) != digest {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
