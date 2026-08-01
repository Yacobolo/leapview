package swarm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

type commander interface {
	Output(context.Context, ...string) ([]byte, error)
	Run(context.Context, io.Writer, io.Writer, ...string) error
}

type Config struct {
	Binary      string
	Host        string
	SSHIdentity string
}

type Client struct {
	commands commander
}

func New(config Config) *Client {
	binary := config.Binary
	if binary == "" {
		binary = "docker"
	}
	return newClient(&dockerCommander{binary: binary, host: config.Host, sshIdentity: config.SSHIdentity})
}

func newClient(commands commander) *Client {
	return &Client{commands: commands}
}

func (c *Client) Check(ctx context.Context) error {
	data, err := c.commands.Output(ctx, "info", "--format", "{{.Swarm.LocalNodeState}}")
	if err != nil {
		return fmt.Errorf("inspect Docker Swarm: %w", err)
	}
	if state := strings.TrimSpace(string(data)); state != "active" {
		return fmt.Errorf("Docker Swarm is %s, want active", fallback(state, "unavailable"))
	}
	return nil
}

func (c *Client) ValidateImage(ctx context.Context, image string) error {
	var output bytes.Buffer
	if err := c.commands.Run(ctx, &output, &output, "image", "pull", image); err != nil {
		return fmt.Errorf("pull image %s: %w: %s", image, err, strings.TrimSpace(output.String()))
	}
	data, err := c.commands.Output(ctx, "image", "inspect", "--format", "{{.Id}}", image)
	if err != nil {
		return fmt.Errorf("inspect image %s: %w", image, err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "sha256:") {
		return fmt.Errorf("inspect image %s: missing content digest", image)
	}
	return nil
}

func (c *Client) Create(ctx context.Context, spec Spec) (string, error) {
	if spec.ID == "" {
		return "", errors.New("job ID is required")
	}
	var output bytes.Buffer
	if err := c.commands.Run(ctx, &output, &output, CreateArgs(spec)...); err != nil {
		return "", fmt.Errorf("create Swarm job: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return spec.ID, nil
}

func (c *Client) Status(ctx context.Context, id string) (protocol.Job, error) {
	serviceData, err := c.commands.Output(ctx, "service", "inspect", id)
	if err != nil {
		return protocol.Job{}, fmt.Errorf("inspect job %s: %w", id, err)
	}
	var services []serviceInspect
	if err := json.Unmarshal(serviceData, &services); err != nil || len(services) != 1 {
		return protocol.Job{}, fmt.Errorf("decode job %s service: %w", id, decodeError(err, len(services)))
	}
	service := services[0]
	labels := service.Spec.Labels
	job := protocol.Job{
		ID: id, Repository: decodeLabel(labels["rtest.repository"]), Suite: decodeLabel(labels["rtest.suite"]),
		Runner: decodeLabel(labels["rtest.runner"]), SourceDigest: labels["rtest.root_digest"],
		Command: append([]string(nil), service.Spec.TaskTemplate.ContainerSpec.Args...), CreatedAt: service.CreatedAt,
		Status: protocol.StatusQueued, CancelRequested: labels[cancelledLabel] == "true",
	}
	job.TimeoutSeconds, _ = strconv.Atoi(labels["rtest.timeout_seconds"])
	if job.CancelRequested {
		job.Status = protocol.StatusCancelled
		finished := service.UpdatedAt
		job.FinishedAt = &finished
	}
	taskIDs, err := c.commands.Output(ctx, "service", "ps", "-q", "--no-trunc", id)
	if err != nil {
		return protocol.Job{}, fmt.Errorf("list job %s tasks: %w", id, err)
	}
	fields := strings.Fields(string(taskIDs))
	if len(fields) == 0 {
		return job, nil
	}
	taskData, err := c.commands.Output(ctx, "inspect", fields[0])
	if err != nil {
		return protocol.Job{}, fmt.Errorf("inspect job %s task: %w", id, err)
	}
	var tasks []taskInspect
	if err := json.Unmarshal(taskData, &tasks); err != nil || len(tasks) != 1 {
		return protocol.Job{}, fmt.Errorf("decode job %s task: %w", id, decodeError(err, len(tasks)))
	}
	task := tasks[0]
	exitCode := task.Status.ContainerStatus.ExitCode
	job.Status = taskStatus(task.Status.State, exitCode, job.CancelRequested)
	job.WorkerID = task.NodeID
	if !task.CreatedAt.IsZero() {
		started := task.CreatedAt
		job.StartedAt = &started
	}
	if job.Status.Terminal() {
		finished := task.Status.Timestamp
		if finished.IsZero() {
			finished = task.UpdatedAt
		}
		job.FinishedAt = &finished
		job.ExitCode = &exitCode
		job.ErrorMessage = task.Status.Err
	}
	return job, nil
}

func (c *Client) Logs(ctx context.Context, id string, follow bool, output io.Writer) error {
	args := []string{"service", "logs", "--raw"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, id)
	if err := c.commands.Run(ctx, output, output, args...); err != nil && ctx.Err() == nil {
		return fmt.Errorf("read job %s logs: %w", id, err)
	}
	return ctx.Err()
}

func (c *Client) Wait(ctx context.Context, id string, output io.Writer) (protocol.Job, error) {
	logsCtx, stopLogs := context.WithCancel(ctx)
	logsDone := make(chan error, 1)
	go func() { logsDone <- c.Logs(logsCtx, id, true, output) }()
	defer func() {
		stopLogs()
		<-logsDone
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		job, err := c.Status(ctx, id)
		if err != nil {
			return protocol.Job{}, err
		}
		if job.Status.Terminal() {
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return protocol.Job{}, ctx.Err()
			case <-timer.C:
			}
			return job, nil
		}
		select {
		case <-ctx.Done():
			cancelCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			cancelErr := c.Cancel(cancelCtx, id)
			cancel()
			if cancelErr != nil {
				return protocol.Job{}, fmt.Errorf("%w; cancel remote job: %v", ctx.Err(), cancelErr)
			}
			return protocol.Job{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) Cancel(ctx context.Context, id string) error {
	if err := c.commands.Run(ctx, io.Discard, io.Discard, "service", "update", "--detach", "--label-add", cancelledLabel+"=true", id); err != nil {
		return fmt.Errorf("mark job %s cancelled: %w", id, err)
	}
	if err := c.commands.Run(ctx, io.Discard, io.Discard, "service", "scale", "--detach", id+"=0"); err != nil {
		return fmt.Errorf("stop job %s: %w", id, err)
	}
	return nil
}

func (c *Client) List(ctx context.Context, repository string, limit int) ([]protocol.Job, error) {
	data, err := c.commands.Output(ctx, "service", "ls", "--filter", "label="+managedLabel+"=true", "--format", "{{.Name}}")
	if err != nil {
		return nil, fmt.Errorf("list Swarm jobs: %w", err)
	}
	var jobs []protocol.Job
	for _, id := range strings.Fields(string(data)) {
		job, err := c.Status(ctx, id)
		if err != nil {
			return nil, err
		}
		if repository == "" || job.Repository == repository {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

type serviceInspect struct {
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	Spec      struct {
		Name         string            `json:"Name"`
		Labels       map[string]string `json:"Labels"`
		TaskTemplate struct {
			ContainerSpec struct {
				Args []string `json:"Args"`
			} `json:"ContainerSpec"`
		} `json:"TaskTemplate"`
	} `json:"Spec"`
}

type taskInspect struct {
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	NodeID    string    `json:"NodeID"`
	Status    struct {
		State           string    `json:"State"`
		Timestamp       time.Time `json:"Timestamp"`
		Err             string    `json:"Err"`
		ContainerStatus struct {
			ExitCode int `json:"ExitCode"`
		} `json:"ContainerStatus"`
	} `json:"Status"`
}

func decodeError(err error, count int) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("expected one object, got %d", count)
}

type dockerCommander struct {
	binary      string
	host        string
	sshIdentity string
}

func (d *dockerCommander) Output(ctx context.Context, args ...string) ([]byte, error) {
	command := d.command(ctx, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (d *dockerCommander) Run(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	command := d.command(ctx, args...)
	command.Stdout, command.Stderr = stdout, stderr
	return command.Run()
}

func (d *dockerCommander) command(ctx context.Context, args ...string) *exec.Cmd {
	if d.host != "" {
		args = append([]string{"--host", d.host}, args...)
	}
	command := exec.CommandContext(ctx, d.binary, args...)
	command.Env = os.Environ()
	if strings.HasPrefix(d.host, "ssh://") {
		sshCommand := "ssh -o IgnoreUnknown=UseKeychain -o BatchMode=yes -o UseKeychain=yes -o AddKeysToAgent=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
		if d.sshIdentity != "" {
			sshCommand += " -i " + shellQuote(d.sshIdentity)
		}
		command.Env = append(command.Env, "DOCKER_SSH_COMMAND="+sshCommand)
	}
	return command
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
