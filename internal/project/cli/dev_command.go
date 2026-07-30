package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/flidai/leapview/internal/project/devloop"
	"github.com/flidai/leapview/internal/project/schema"
	"github.com/spf13/cobra"
)

type DevOptions struct {
	ProjectPath       string
	Credentials       cliapi.Credentials
	UploadConcurrency int
	Once              bool
}

// DevRemoteFactory is the Project-owned port for binding the dev loop to an
// authenticated target. The application adapter may implement it with
// Deployment APIs without making Project depend on Deployment.
type DevRemoteFactory interface {
	Remote(
		context.Context,
		cliapi.Credentials,
		int,
	) (devloop.Remote, error)
}

// DevCommand synchronizes coherent local project snapshots into one private
// target candidate. It never starts or embeds a LeapView server.
func DevCommand(
	ctx context.Context,
	client cliapi.Client,
	checkpoints *CandidateCheckpointStore,
	remotes DevRemoteFactory,
) *cobra.Command {
	values := DevOptions{
		ProjectPath:       filepath.Join("dashboards", "leapview.yaml"),
		UploadConcurrency: 4,
	}
	command := &cobra.Command{
		Use:   "dev [project]",
		Short: "Synchronize a project into your private target candidate",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 1 {
				if command.Flags().Changed("project") {
					return fmt.Errorf(
						"choose either --project or positional project, not both",
					)
				}
				values.ProjectPath = args[0]
			}
			return RunDev(
				ctx,
				client,
				checkpoints,
				remotes,
				values,
				command.OutOrStdout(),
				command.ErrOrStderr(),
			)
		},
	}
	command.Flags().StringVar(
		&values.ProjectPath,
		"project",
		values.ProjectPath,
		"project manifest path",
	)
	command.Flags().StringVar(
		&values.Credentials.Target,
		"target",
		"",
		"authenticated target profile or LeapView target URL",
	)
	command.Flags().StringVar(
		&values.Credentials.Token,
		"token",
		"",
		"ephemeral API token compatibility path",
	)
	command.Flags().IntVar(
		&values.UploadConcurrency,
		"upload-concurrency",
		values.UploadConcurrency,
		"maximum parallel content-addressed source uploads (1-16)",
	)
	command.Flags().BoolVar(
		&values.Once,
		"once",
		false,
		"synchronize one candidate and exit",
	)
	return command
}

// RunDev executes the Project-owned candidate synchronization lifecycle. It is
// shared by the public command and target bootstrap adapters that must exercise
// the exact same candidate contract.
func RunDev(
	ctx context.Context,
	client cliapi.Client,
	checkpoints *CandidateCheckpointStore,
	remotes DevRemoteFactory,
	options DevOptions,
	out,
	errOut io.Writer,
) error {
	if client == nil {
		return fmt.Errorf("Project CLI API client is required")
	}
	if checkpoints == nil {
		return fmt.Errorf("Project candidate checkpoint store is required")
	}
	if remotes == nil {
		return fmt.Errorf("Project candidate remote factory is required")
	}
	credentials, err := client.Resolve(ctx, options.Credentials)
	if err != nil {
		return err
	}
	remote, err := remotes.Remote(
		ctx,
		credentials,
		options.UploadConcurrency,
	)
	if err != nil {
		return err
	}
	service, err := devloop.New(
		devloop.FilesystemBuilder{ProjectPath: options.ProjectPath},
		remote,
	)
	if err != nil {
		return err
	}
	lastPreviewURL := ""
	report := func(update devloop.Update) error {
		if update.Err != nil {
			for _, diagnostic := range configschema.Diagnostics(update.Err) {
				fmt.Fprintln(errOut, diagnostic.String())
			}
			return update.Err
		}
		if update.Result.Status != devloop.StatusSynchronized {
			return nil
		}
		candidate := update.Result.Candidate
		if err := checkpoints.Save(CandidateCheckpoint{
			ProjectPath: options.ProjectPath, TargetOrigin: credentials.Target,
			TargetID: candidate.TargetID, Environment: candidate.Environment,
			ProjectID: candidate.ProjectID, CandidateID: candidate.ID,
			CandidateRevision: candidate.Revision,
			ArtifactDigest:    candidate.ArtifactDigest,
			ProvenanceDigest:  candidate.ProvenanceDigest,
		}); err != nil {
			return fmt.Errorf("persist publish candidate: %w", err)
		}
		fmt.Fprintf(out, "synchronized %s\n", candidate.ArtifactDigest)
		if candidate.PreviewURL != "" &&
			candidate.PreviewURL != lastPreviewURL {
			fmt.Fprintf(out, "preview %s\n", candidate.PreviewURL)
			lastPreviewURL = candidate.PreviewURL
		}
		return nil
	}
	if options.Once {
		result, reconcileErr := service.Reconcile(ctx)
		reportErr := report(devloop.Update{
			Result: result,
			Err:    reconcileErr,
		})
		if reconcileErr != nil {
			return reconcileErr
		}
		return reportErr
	}
	watcher, err := devloop.NewWatcher(options.ProjectPath, service)
	if err != nil {
		return err
	}
	signalContext, stop := signal.NotifyContext(
		ctx,
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	return watcher.Run(signalContext, func(update devloop.Update) {
		if err := report(update); err != nil && update.Err == nil {
			fmt.Fprintln(errOut, err)
		}
	})
}
