package swarmscheduler

import (
	"context"
	"io"

	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/protocol"
	"github.com/flidai/leapview/rtest/internal/swarm"
)

type Config struct {
	Client             *swarm.Client
	CASAddress         string
	CASInstance        string
	JobsRoot           string
	EntrypointHostPath string
}

type Scheduler struct {
	config Config
}

func New(config Config) *Scheduler { return &Scheduler{config: config} }

func (s *Scheduler) Check(ctx context.Context) error { return s.config.Client.Check(ctx) }

func (s *Scheduler) Create(ctx context.Context, job control.Job) error {
	_, err := s.config.Client.Create(ctx, swarm.Spec{
		ID: job.ID, Repository: job.ProjectID, Suite: "exec", Runner: "oci",
		Image: job.Image, CASAddress: s.config.CASAddress, CASInstance: s.config.CASInstance,
		RootDigest: job.RootDigest, JobsRoot: s.config.JobsRoot, Command: job.Command,
		WorkingDirectory: job.WorkingDirectory, Environment: job.Environment,
		EntrypointHostPath: s.config.EntrypointHostPath,
		Timeout:            job.Timeout, CPUs: job.CPUs, Memory: job.Memory,
	})
	return err
}

func (s *Scheduler) Status(ctx context.Context, id string) (protocol.Job, error) {
	return s.config.Client.Status(ctx, id)
}

func (s *Scheduler) Logs(ctx context.Context, id string, follow bool, output io.Writer) error {
	return s.config.Client.Logs(ctx, id, follow, output)
}

func (s *Scheduler) Cancel(ctx context.Context, id string) error {
	return s.config.Client.Cancel(ctx, id)
}

var _ control.Scheduler = (*Scheduler)(nil)
