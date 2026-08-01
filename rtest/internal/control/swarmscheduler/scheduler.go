package swarmscheduler

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

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
	CacheRoot          string
	HostUID            string
	HostGID            string
}

type Scheduler struct {
	config Config
}

func New(config Config) *Scheduler { return &Scheduler{config: config} }

func (s *Scheduler) Check(ctx context.Context) error { return s.config.Client.Check(ctx) }

func (s *Scheduler) ValidateImage(ctx context.Context, image string) error {
	return s.config.Client.ValidateImage(ctx, image)
}

func (s *Scheduler) Create(ctx context.Context, job control.Job) error {
	if err := prepareCacheDirectories(s.config.CacheRoot, job.ProjectID, job.Caches); err != nil {
		return err
	}
	_, err := s.config.Client.Create(ctx, specForJob(s.config, job))
	return err
}

func specForJob(config Config, job control.Job) swarm.Spec {
	caches := make([]swarm.CacheMount, 0, len(job.Caches))
	for _, cache := range job.Caches {
		caches = append(caches, swarm.CacheMount{Name: cache.Name, Target: cache.Target})
	}
	return swarm.Spec{
		ID: job.ID, Repository: job.ProjectID, Suite: "exec", Runner: "oci",
		Image: job.Image, CASAddress: config.CASAddress, CASInstance: config.CASInstance,
		RootDigest: job.RootDigest, JobsRoot: config.JobsRoot, Command: job.Command,
		WorkingDirectory: job.WorkingDirectory, Environment: job.Environment,
		EntrypointHostPath: config.EntrypointHostPath,
		Timeout:            job.Timeout, CPUs: job.CPUs, Memory: job.Memory,
		CacheRoot: config.CacheRoot, ProjectID: job.ProjectID, Caches: caches,
		HostUID: config.HostUID, HostGID: config.HostGID,
	}
}

func prepareCacheDirectories(root, projectID string, caches []control.CacheMount) error {
	if len(caches) == 0 {
		return nil
	}
	if root == "" || !safeComponent(projectID) {
		return errors.New("cache root and safe project ID are required")
	}
	for _, cache := range caches {
		if !safeComponent(cache.Name) {
			return errors.New("cache name is not a safe path component")
		}
		directory := filepath.Join(root, projectID, cache.Name)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func safeComponent(value string) bool {
	return value != "" && value != "." && value != ".." && filepath.Base(value) == value
}

func (s *Scheduler) Status(ctx context.Context, id string) (protocol.Job, error) {
	return s.config.Client.Status(ctx, id)
}

func (s *Scheduler) Logs(ctx context.Context, id string, follow bool, output io.Writer) error {
	if !follow {
		file, err := os.Open(filepath.Join(s.config.JobsRoot, id, "job.log"))
		if err == nil {
			defer file.Close()
			_, err = io.Copy(output, file)
			return err
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return s.config.Client.Logs(ctx, id, follow, output)
}

func (s *Scheduler) Cancel(ctx context.Context, id string) error {
	return s.config.Client.Cancel(ctx, id)
}

func (s *Scheduler) ManagedJobs(ctx context.Context) ([]protocol.Job, error) {
	return s.config.Client.List(ctx, "", 0)
}

func (s *Scheduler) Remove(ctx context.Context, id string) error {
	return s.config.Client.Remove(ctx, id)
}

var _ control.Scheduler = (*Scheduler)(nil)
