package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flidai/leapview/rtest/internal/client"
	"github.com/flidai/leapview/rtest/internal/protocol"
	"github.com/flidai/leapview/rtest/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	api, err := client.New(env("RTEST_URL", "http://127.0.0.1:8080"), os.Getenv("RTEST_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	hostname, _ := os.Hostname()
	workerID := env("RTEST_WORKER_ID", hostname)
	runner := worker.Runner{
		Docker: env("RTEST_DOCKER", "docker"), WorkRoot: env("RTEST_WORK_ROOT", "/var/lib/rtest/jobs"),
		CacheRoot: env("RTEST_CACHE_ROOT", "/var/lib/rtest/cache"),
		Image:     env("RTEST_RUNNER_IMAGE", "rtest-runner-standard:local"), CPUs: env("RTEST_JOB_CPUS", "1.5"), Memory: env("RTEST_JOB_MEMORY", "2500m"),
	}
	if err := os.MkdirAll(runner.WorkRoot, 0o700); err != nil {
		log.Fatal(err)
	}
	log.Printf("rtest worker %s ready", workerID)
	for ctx.Err() == nil {
		job, ok, err := api.Claim(ctx, workerID)
		if err != nil {
			log.Printf("claim: %v", err)
			wait(ctx, 2*time.Second)
			continue
		}
		if !ok {
			wait(ctx, time.Second)
			continue
		}
		runJob(ctx, api, runner, workerID, job)
	}
}

func runJob(parent context.Context, api *client.Client, runner worker.Runner, workerID string, job protocol.Job) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	source, err := api.DownloadSource(ctx, job.ID)
	if err != nil {
		finish(api, job.ID, protocol.FinishRequest{Status: protocol.StatusLost, ErrorMessage: err.Error()})
		return
	}
	defer source.Close()
	logs := &remoteLogWriter{ctx: ctx, api: api, jobID: job.ID}
	_, _ = fmt.Fprintf(logs, "rtest: worker=%s runner=%s command=%q\n", workerID, job.Runner, job.Command)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := api.Heartbeat(ctx, job.ID, workerID)
				if err != nil || current.CancelRequested {
					cancel()
					return
				}
			}
		}
	}()
	result := runner.Run(ctx, job, source, logs)
	cancel()
	finish(api, job.ID, result)
}

func finish(api *client.Client, id string, result protocol.FinishRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := api.Finish(ctx, id, result); err != nil {
		log.Printf("finish %s: %v", id, err)
	}
}

type remoteLogWriter struct {
	ctx   context.Context
	api   *client.Client
	jobID string
}

func (w *remoteLogWriter) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		size := min(len(data), 256<<10)
		if err := w.api.AppendLog(w.ctx, w.jobID, data[:size]); err != nil {
			return written, err
		}
		written += size
		data = data[size:]
	}
	return written, nil
}

func wait(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

var _ io.Writer = (*remoteLogWriter)(nil)
