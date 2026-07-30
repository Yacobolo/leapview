package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/spf13/cobra"
)

type PublishOptions struct {
	ProjectPath string
	Credentials cliapi.Credentials
	Checkpoint  CandidateCheckpoint
}

// PublishOperations is the Project-owned port for requesting policy-governed
// publication of an exact candidate.
type PublishOperations interface {
	Publish(context.Context, PublishOptions, io.Writer) error
}

// PublishCommand promotes the last exact candidate synchronized by dev.
func PublishCommand(
	ctx context.Context,
	client cliapi.Client,
	store *CandidateCheckpointStore,
	operations PublishOperations,
) *cobra.Command {
	values := PublishOptions{ProjectPath: filepath.Join("dashboards", "leapview.yaml")}
	command := &cobra.Command{
		Use:   "publish",
		Short: "Publish the exact candidate last synchronized by dev",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return RunPublish(
				ctx,
				client,
				store,
				operations,
				values,
				command.OutOrStdout(),
			)
		},
	}
	command.Flags().StringVar(
		&values.ProjectPath, "project", values.ProjectPath,
		"project manifest path used by leapview dev",
	)
	command.Flags().StringVar(
		&values.Credentials.Target, "target", "",
		"authenticated target profile or LeapView target URL",
	)
	command.Flags().StringVar(
		&values.Credentials.Token, "token", "",
		"ephemeral API token compatibility path",
	)
	return command
}

// RunPublish promotes the exact candidate checkpoint produced by RunDev. It is
// shared by the public command and target bootstrap adapters that must not
// bypass policy-governed publication.
func RunPublish(
	ctx context.Context,
	client cliapi.Client,
	store *CandidateCheckpointStore,
	operations PublishOperations,
	options PublishOptions,
	out io.Writer,
) error {
	if client == nil {
		return fmt.Errorf("Project CLI API client is required")
	}
	if store == nil {
		return fmt.Errorf("Project candidate checkpoint store is required")
	}
	if operations == nil {
		return fmt.Errorf("Project publish operations are required")
	}
	credentials, err := client.Resolve(ctx, options.Credentials)
	if err != nil {
		return err
	}
	checkpoint, err := store.Load(options.ProjectPath, credentials.Target)
	if errors.Is(err, ErrCandidateCheckpointNotFound) {
		return fmt.Errorf(
			"%w for this project and target; run leapview dev first",
			err,
		)
	}
	if err != nil {
		return err
	}
	options.Credentials = credentials
	options.Checkpoint = checkpoint
	return operations.Publish(ctx, options, out)
}
