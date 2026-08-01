package reapi

import (
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
)

type Request struct {
	Root       string
	Files      []string
	Command    []string
	Timeout    time.Duration
	Repository string
	Runner     string
}

// Action converts the rtest project contract to the standard REAPI command
// model. Tests deliberately bypass the action cache because Testcontainers and
// other external services make their results non-hermetic; CAS uploads are
// still content-addressed and incremental.
func Action(request Request) (*command.Command, *command.ExecutionOptions) {
	cmd := &command.Command{
		Args:     append([]string(nil), request.Command...),
		ExecRoot: request.Root,
		InputSpec: &command.InputSpec{
			Inputs:          append([]string(nil), request.Files...),
			SymlinkBehavior: command.PreserveSymlink,
		},
		Timeout: request.Timeout,
		Platform: map[string]string{
			"OSFamily":         "linux",
			"rtest.repository": request.Repository,
			"rtest.runner":     request.Runner,
		},
		Identifiers: &command.Identifiers{
			ToolName:    "rtest",
			ToolVersion: "0.2.0-poc",
		},
	}
	options := &command.ExecutionOptions{
		AcceptCached:    false,
		DoNotCache:      true,
		DownloadOutputs: false,
		DownloadOutErr:  true,
		StreamOutErr:    true,
	}
	return cmd, options
}
