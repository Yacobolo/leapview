package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/flidai/leapview/rtest/internal/buildkit"
	"github.com/flidai/leapview/rtest/internal/cas"
	"github.com/flidai/leapview/rtest/internal/client"
	"github.com/flidai/leapview/rtest/internal/config"
	"github.com/flidai/leapview/rtest/internal/profile"
	"github.com/flidai/leapview/rtest/internal/protocol"
	"github.com/flidai/leapview/rtest/internal/reapi"
	"github.com/flidai/leapview/rtest/internal/snapshot"
	"github.com/flidai/leapview/rtest/internal/swarm"
	"github.com/flidai/leapview/rtest/internal/tunnel"
	"github.com/flidai/leapview/rtest/internal/workspace"
)

const version = "0.5.0"

type IO struct {
	Stdout io.Writer
	Stderr io.Writer
	Dir    string
}

func Run(ctx context.Context, args []string, streams IO) int {
	streams = defaults(streams)
	explicitToken, args, err := globalArgs(args)
	if err != nil {
		return failUsage(streams.Stderr, err.Error())
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		usage(streams.Stdout)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(streams.Stdout, version)
		return 0
	}
	settings, err := config.Load()
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if settings.Backend == config.BackendService {
		return runService(ctx, settings, explicitToken, args, streams)
	}
	if explicitToken != "" {
		settings.Token = explicitToken
	}
	if args[0] == "build" {
		return runBuild(ctx, settings, args[1:], streams)
	}
	if settings.Backend == config.BackendREAPI {
		return runREAPI(ctx, settings, args, streams)
	}
	if settings.Backend == config.BackendSwarm {
		return runSwarm(ctx, settings, args, streams)
	}
	url, sshTunnel, err := tunnel.Open(ctx, settings)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if sshTunnel != nil {
		defer sshTunnel.Close()
	}
	api, err := client.New(url, settings.Token)
	if err != nil {
		return fail(streams.Stderr, err)
	}

	switch args[0] {
	case "run":
		return runJob(ctx, api, args[1:], streams)
	case "status":
		return status(ctx, api, args[1:], streams)
	case "logs":
		return logs(ctx, api, args[1:], streams)
	case "cancel":
		return cancel(ctx, api, args[1:], streams)
	case "list":
		return list(ctx, api, args[1:], streams)
	case "doctor":
		return doctor(ctx, api, settings, streams)
	default:
		fmt.Fprintf(streams.Stderr, "rtest: unknown command %q\n", args[0])
		usage(streams.Stderr)
		return 2
	}
}

func runSwarm(ctx context.Context, settings config.Config, args []string, streams IO) int {
	casService := settings.CAS.Service
	var casTunnel *tunnel.Tunnel
	if casService == "" {
		address, opened, err := tunnel.Forward(ctx, settings.SSH, settings.CAS.RemoteAddress)
		if err != nil {
			return fail(streams.Stderr, err)
		}
		casService, casTunnel = address, opened
		defer casTunnel.Close()
	}
	dockerHost := settings.Swarm.DockerHost
	identity := ""
	if dockerHost == "" && settings.SSH != nil {
		dockerHost = "ssh://" + settings.SSH.User + "@" + settings.SSH.Host
		identity = settings.SSH.IdentityFile
	}
	docker := swarm.New(swarm.Config{Binary: os.Getenv("RTEST_DOCKER"), Host: dockerHost, SSHIdentity: identity})
	switch args[0] {
	case "run":
		return runSwarmJob(ctx, docker, casService, settings, args[1:], streams)
	case "doctor":
		if err := cas.Check(ctx, casService, settings.CAS.Instance); err != nil {
			return fail(streams.Stderr, err)
		}
		if err := docker.Check(ctx); err != nil {
			return fail(streams.Stderr, err)
		}
		transport := "local Docker and direct CAS"
		if settings.SSH != nil && settings.Swarm.DockerHost == "" {
			transport = "Docker SSH and CAS tunnel to " + settings.SSH.Host
		}
		fmt.Fprintf(streams.Stdout, "rtest %s\nconnection: ok (%s)\ninstance: %s\n", version, transport, settings.CAS.Instance)
		return 0
	case "status":
		jsonOutput, id, err := jobArgs(args[1:])
		if err != nil {
			return failUsage(streams.Stderr, "status "+err.Error())
		}
		job, err := docker.Status(ctx, id)
		if err != nil {
			return fail(streams.Stderr, err)
		}
		if jsonOutput {
			return encode(streams, job)
		}
		printJob(streams.Stdout, job)
		return 0
	case "logs":
		if len(args) != 2 {
			return failUsage(streams.Stderr, "logs requires exactly one job ID")
		}
		job, err := docker.Status(ctx, args[1])
		if err != nil {
			return fail(streams.Stderr, err)
		}
		if err := docker.Logs(ctx, args[1], !job.Status.Terminal(), streams.Stdout); err != nil {
			return fail(streams.Stderr, err)
		}
		job, err = docker.Status(ctx, args[1])
		if err != nil {
			return fail(streams.Stderr, err)
		}
		printCompletion(streams.Stderr, job)
		return client.ExitCode(job)
	case "cancel":
		if len(args) != 2 {
			return failUsage(streams.Stderr, "cancel requires exactly one job ID")
		}
		if err := docker.Cancel(ctx, args[1]); err != nil {
			return fail(streams.Stderr, err)
		}
		fmt.Fprintf(streams.Stdout, "Job %s: cancelled\n", args[1])
		return 0
	case "list":
		return listSwarm(ctx, docker, args[1:], streams)
	default:
		fmt.Fprintf(streams.Stderr, "rtest: unknown command %q\n", args[0])
		usage(streams.Stderr)
		return 2
	}
}

func runSwarmJob(ctx context.Context, docker *swarm.Client, casService string, settings config.Config, args []string, streams IO) int {
	detach, selected, root, err := resolveRun(ctx, args, streams.Dir)
	if err != nil {
		return failUsage(streams.Stderr, err.Error())
	}
	files, err := workspace.Files(ctx, root)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	upload, err := cas.Upload(ctx, casService, settings.CAS.Instance, root, files)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	id, err := jobID()
	if err != nil {
		return fail(streams.Stderr, err)
	}
	_, err = docker.Create(ctx, swarm.Spec{
		ID: id, Repository: selected.Repository, Suite: selected.Suite, Runner: selected.Runner,
		Image: settings.Swarm.Image, CASAddress: settings.CAS.JobAddress, CASInstance: settings.CAS.Instance,
		RootDigest: upload.RootDigest, JobsRoot: settings.Swarm.JobsRoot, Command: selected.Command,
		Timeout: time.Duration(selected.TimeoutSeconds) * time.Second, CPUs: settings.Swarm.CPUs, Memory: settings.Swarm.Memory,
	})
	if err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintf(streams.Stderr, "Backend: Docker Swarm job\nJob: %s\nInputs: %d files, %s\nTransfer: %s uploaded\n",
		id, upload.InputFiles, humanBytes(upload.TotalInputBytes), humanBytes(upload.TransferredBytes))
	if detach {
		return 0
	}
	job, err := docker.Wait(ctx, id, streams.Stdout)
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("wait for remote job: %w", err))
	}
	printCompletion(streams.Stderr, job)
	return client.ExitCode(job)
}

func resolveRun(ctx context.Context, args []string, directory string) (bool, profile.Resolved, string, error) {
	detach := false
	timeout := 30 * time.Minute
	for len(args) > 0 && args[0] != "--" && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--detach":
			detach, args = true, args[1:]
		case "--timeout":
			if len(args) < 2 {
				return false, profile.Resolved{}, "", errors.New("--timeout requires a duration")
			}
			value, err := time.ParseDuration(args[1])
			if err != nil || value < time.Second || value > time.Hour {
				return false, profile.Resolved{}, "", errors.New("--timeout must be between 1s and 1h")
			}
			timeout, args = value, args[2:]
		default:
			return false, profile.Resolved{}, "", errors.New("unknown run option " + args[0])
		}
	}
	if len(args) == 0 {
		return false, profile.Resolved{}, "", errors.New("run requires a suite or -- <command>")
	}
	root, err := profile.Root(ctx, directory)
	if err != nil {
		return false, profile.Resolved{}, "", err
	}
	var selected profile.Resolved
	if args[0] == "--" {
		selected, err = profile.Command(root, args[1:], int(timeout.Seconds()))
	} else {
		extra := args[1:]
		if len(extra) > 0 && extra[0] == "--" {
			extra = extra[1:]
		} else if len(extra) > 0 {
			return false, profile.Resolved{}, "", errors.New("suite arguments must follow --")
		}
		selected, err = profile.Load(root, args[0], extra)
		if err == nil && timeout != 30*time.Minute {
			selected.TimeoutSeconds = int(timeout.Seconds())
		}
	}
	return detach, selected, root, err
}

func jobID() (string, error) {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return fmt.Sprintf("rtest-%x", value), nil
}

func listSwarm(ctx context.Context, docker *swarm.Client, args []string, streams IO) int {
	repository, limit, jsonOutput := "", 20, false
	for len(args) > 0 {
		switch args[0] {
		case "--repository":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--repository requires a value")
			}
			repository, args = args[1], args[2:]
		case "--limit":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--limit requires a value")
			}
			value, err := strconv.Atoi(args[1])
			if err != nil || value < 1 || value > 100 {
				return failUsage(streams.Stderr, "--limit must be between 1 and 100")
			}
			limit, args = value, args[2:]
		case "--json":
			jsonOutput, args = true, args[1:]
		default:
			return failUsage(streams.Stderr, "unknown list option "+args[0])
		}
	}
	jobs, err := docker.List(ctx, repository, limit)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if jsonOutput {
		return encode(streams, jobs)
	}
	writer := tabwriter.NewWriter(streams.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "JOB\tSTATUS\tREPOSITORY\tSUITE\tAGE\tDURATION")
	for _, job := range jobs {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", job.ID, job.Status, job.Repository, job.Suite,
			time.Since(job.CreatedAt).Round(time.Second), duration(job))
	}
	_ = writer.Flush()
	return 0
}

func runBuild(ctx context.Context, settings config.Config, args []string, streams IO) int {
	if settings.BuildKit == nil {
		return fail(streams.Stderr, errors.New("buildkit configuration is required for rtest build"))
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		args = []string{"."}
	}
	root, err := profile.Root(ctx, streams.Dir)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	address := settings.BuildKit.Address
	var sshTunnel *tunnel.Tunnel
	if address == "" {
		local, opened, err := tunnel.Forward(ctx, settings.SSH, settings.BuildKit.RemoteAddress)
		if err != nil {
			return fail(streams.Stderr, err)
		}
		address, sshTunnel = "tcp://"+local, opened
		defer sshTunnel.Close()
	}
	if !strings.Contains(address, "://") {
		address = "tcp://" + address
	}
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return fail(streams.Stderr, err)
	}
	name := fmt.Sprintf("rtest-%x", random)
	fmt.Fprintf(streams.Stderr, "Backend: BuildKit via native Docker Buildx\nBuilder: %s\n", address)
	code, err := buildkit.Run(ctx, os.Getenv("RTEST_DOCKER"), address, name, root, args, streams.Stdout, streams.Stderr)
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("remote build: %w", err))
	}
	return code
}

func runREAPI(ctx context.Context, settings config.Config, args []string, streams IO) int {
	service := settings.REAPI.Service
	var sshTunnel *tunnel.Tunnel
	if service == "" {
		address, opened, err := tunnel.Forward(ctx, settings.SSH, settings.REAPI.RemoteAddress)
		if err != nil {
			return fail(streams.Stderr, err)
		}
		service, sshTunnel = address, opened
		defer sshTunnel.Close()
	}
	switch args[0] {
	case "run":
		return runREAPIJob(ctx, service, settings.REAPI.Instance, args[1:], streams)
	case "doctor":
		if err := reapi.Check(ctx, service, settings.REAPI.Instance); err != nil {
			return fail(streams.Stderr, err)
		}
		transport := "direct gRPC"
		if sshTunnel != nil {
			transport = "REAPI over SSH to " + settings.SSH.Host
		}
		fmt.Fprintf(streams.Stdout, "rtest %s\nconnection: ok (%s)\ninstance: %s\n", version, transport, settings.REAPI.Instance)
		return 0
	case "status", "logs", "cancel", "list":
		return fail(streams.Stderr, fmt.Errorf("%s is only available with the legacy detached-job backend; REAPI run cancellation follows the client context", args[0]))
	default:
		fmt.Fprintf(streams.Stderr, "rtest: unknown command %q\n", args[0])
		usage(streams.Stderr)
		return 2
	}
}

func runREAPIJob(ctx context.Context, service, instance string, args []string, streams IO) int {
	timeout := 30 * time.Minute
	for len(args) > 0 && args[0] != "--" && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--timeout":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--timeout requires a duration")
			}
			value, err := time.ParseDuration(args[1])
			if err != nil || value < time.Second || value > time.Hour {
				return failUsage(streams.Stderr, "--timeout must be between 1s and 1h")
			}
			timeout, args = value, args[2:]
		case "--detach":
			return failUsage(streams.Stderr, "--detach is not supported by the REAPI backend")
		default:
			return failUsage(streams.Stderr, "unknown run option "+args[0])
		}
	}
	if len(args) == 0 {
		return failUsage(streams.Stderr, "run requires a suite or -- <command>")
	}
	root, err := profile.Root(ctx, streams.Dir)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	var selected profile.Resolved
	if args[0] == "--" {
		selected, err = profile.Command(root, args[1:], int(timeout.Seconds()))
	} else {
		suite := args[0]
		extra := args[1:]
		if len(extra) > 0 && extra[0] == "--" {
			extra = extra[1:]
		} else if len(extra) > 0 {
			return failUsage(streams.Stderr, "suite arguments must follow --")
		}
		selected, err = profile.Load(root, suite, extra)
		if err == nil && timeout != 30*time.Minute {
			selected.TimeoutSeconds = int(timeout.Seconds())
		}
	}
	if err != nil {
		return fail(streams.Stderr, err)
	}
	files, err := workspace.Files(ctx, root)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintf(streams.Stderr, "Backend: REAPI v2 (%s)\nInputs: %d worktree files\n", instance, len(files))
	outcome, err := reapi.Execute(ctx, service, instance, reapi.Request{
		Root: root, Files: files, Command: selected.Command,
		Timeout:    time.Duration(selected.TimeoutSeconds) * time.Second,
		Repository: selected.Repository, Runner: selected.Runner,
	}, streams.Stdout, streams.Stderr)
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("remote execution: %w", err))
	}
	fmt.Fprintf(streams.Stderr, "Action: %s\nTransfer: %s uploaded / %s input\nStatus: %s\n",
		outcome.ActionDigest, humanBytes(outcome.RealBytesUploaded), humanBytes(outcome.TotalInputBytes), outcome.Status)
	return outcome.ExitCode
}

func runJob(ctx context.Context, api *client.Client, args []string, streams IO) int {
	detach := false
	timeout := 30 * time.Minute
	for len(args) > 0 && args[0] != "--" && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--detach":
			detach = true
			args = args[1:]
		case "--timeout":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--timeout requires a duration")
			}
			value, err := time.ParseDuration(args[1])
			if err != nil || value < time.Second || value > time.Hour {
				return failUsage(streams.Stderr, "--timeout must be between 1s and 1h")
			}
			timeout = value
			args = args[2:]
		default:
			return failUsage(streams.Stderr, "unknown run option "+args[0])
		}
	}
	if len(args) == 0 {
		return failUsage(streams.Stderr, "run requires a suite or -- <command>")
	}
	root, err := profile.Root(ctx, streams.Dir)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	var selected profile.Resolved
	if args[0] == "--" {
		selected, err = profile.Command(root, args[1:], int(timeout.Seconds()))
	} else {
		suite := args[0]
		extra := args[1:]
		if len(extra) > 0 && extra[0] == "--" {
			extra = extra[1:]
		} else if len(extra) > 0 {
			return failUsage(streams.Stderr, "suite arguments must follow --")
		}
		selected, err = profile.Load(root, suite, extra)
		if err == nil && timeout != 30*time.Minute {
			selected.TimeoutSeconds = int(timeout.Seconds())
		}
	}
	if err != nil {
		return fail(streams.Stderr, err)
	}
	temporary, err := os.CreateTemp("", "rtest-source-*.tar.zst")
	if err != nil {
		return fail(streams.Stderr, err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	snapshotStarted := time.Now()
	result, err := snapshot.Create(ctx, root, temporary)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("snapshot: %w", err))
	}
	source, err := os.Open(path)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	defer source.Close()
	job, err := api.Submit(ctx, protocol.SubmitManifest{
		Repository: selected.Repository, Suite: selected.Suite, Runner: selected.Runner,
		Command: selected.Command, SourceDigest: result.Digest, SourceSize: result.Size,
		TimeoutSeconds: selected.TimeoutSeconds,
	}, source)
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("submit: %w", err))
	}
	fmt.Fprintf(streams.Stderr, "Job: %s\nStatus: %s\nSnapshot: %d files, %s in %s\n", job.ID, job.Status,
		result.Files, humanBytes(result.Size), time.Since(snapshotStarted).Round(time.Millisecond))
	if detach {
		return 0
	}
	finished, err := api.Stream(ctx, job.ID, streams.Stdout)
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("logs: %w (reconnect with: rtest logs %s)", err, job.ID))
	}
	printCompletion(streams.Stderr, finished)
	return client.ExitCode(finished)
}

func status(ctx context.Context, api *client.Client, args []string, streams IO) int {
	jsonOutput, id, err := jobArgs(args)
	if err != nil {
		return failUsage(streams.Stderr, "status "+err.Error())
	}
	job, err := api.Job(ctx, id)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if jsonOutput {
		return encode(streams, job)
	}
	printJob(streams.Stdout, job)
	return 0
}

func logs(ctx context.Context, api *client.Client, args []string, streams IO) int {
	if len(args) != 1 {
		return failUsage(streams.Stderr, "logs requires exactly one job ID")
	}
	job, err := api.Stream(ctx, args[0], streams.Stdout)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	printCompletion(streams.Stderr, job)
	return client.ExitCode(job)
}

func cancel(ctx context.Context, api *client.Client, args []string, streams IO) int {
	if len(args) != 1 {
		return failUsage(streams.Stderr, "cancel requires exactly one job ID")
	}
	if err := api.Cancel(ctx, args[0]); err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintf(streams.Stdout, "Job %s: cancellation requested\n", args[0])
	return 0
}

func list(ctx context.Context, api *client.Client, args []string, streams IO) int {
	repository := ""
	limit := 20
	jsonOutput := false
	for len(args) > 0 {
		switch args[0] {
		case "--repository":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--repository requires a value")
			}
			repository, args = args[1], args[2:]
		case "--limit":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--limit requires a value")
			}
			value, err := strconv.Atoi(args[1])
			if err != nil || value < 1 || value > 100 {
				return failUsage(streams.Stderr, "--limit must be between 1 and 100")
			}
			limit, args = value, args[2:]
		case "--json":
			jsonOutput, args = true, args[1:]
		default:
			return failUsage(streams.Stderr, "unknown list option "+args[0])
		}
	}
	jobs, err := api.List(ctx, repository, limit)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if jsonOutput {
		return encode(streams, jobs)
	}
	writer := tabwriter.NewWriter(streams.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "JOB\tSTATUS\tREPOSITORY\tSUITE\tAGE\tDURATION")
	for _, job := range jobs {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", job.ID, job.Status, job.Repository, job.Suite,
			time.Since(job.CreatedAt).Round(time.Second), duration(job))
	}
	_ = writer.Flush()
	return 0
}

func doctor(ctx context.Context, api *client.Client, settings config.Config, streams IO) int {
	if _, err := api.List(ctx, "", 1); err != nil {
		return fail(streams.Stderr, err)
	}
	transport := "direct HTTP"
	if settings.URL == "" {
		transport = "SSH tunnel to " + settings.SSH.Host
	}
	fmt.Fprintf(streams.Stdout, "rtest %s\nconnection: ok (%s)\n", version, transport)
	return 0
}

func printJob(output io.Writer, job protocol.Job) {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintf(writer, "Job:\t%s\nStatus:\t%s\nRepository:\t%s\nSuite:\t%s\nRunner:\t%s\nWorker:\t%s\nCommand:\t%s\nCreated:\t%s\nDuration:\t%s\n",
		job.ID, job.Status, job.Repository, job.Suite, job.Runner, value(job.WorkerID), strings.Join(job.Command, " "),
		job.CreatedAt.Format(time.RFC3339), duration(job))
	if job.ErrorMessage != "" {
		fmt.Fprintf(writer, "Error:\t%s\n", job.ErrorMessage)
	}
	_ = writer.Flush()
}

func printCompletion(output io.Writer, job protocol.Job) {
	fmt.Fprintf(output, "Completed: %s in %s", job.Status, duration(job))
	if job.ExitCode != nil {
		fmt.Fprintf(output, " (exit %d)", *job.ExitCode)
	}
	fmt.Fprintln(output)
}

func duration(job protocol.Job) string {
	if job.StartedAt == nil {
		return "-"
	}
	end := time.Now()
	if job.FinishedAt != nil {
		end = *job.FinishedAt
	}
	return end.Sub(*job.StartedAt).Round(time.Millisecond).String()
}

func jobArgs(args []string) (bool, string, error) {
	jsonOutput := false
	if len(args) > 0 && args[0] == "--json" {
		jsonOutput, args = true, args[1:]
	}
	if len(args) != 1 {
		return false, "", errors.New("requires exactly one job ID")
	}
	return jsonOutput, args[0], nil
}

func encode(streams IO, value any) int {
	encoder := json.NewEncoder(streams.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fail(streams.Stderr, err)
	}
	return 0
}

func defaults(streams IO) IO {
	if streams.Stdout == nil {
		streams.Stdout = os.Stdout
	}
	if streams.Stderr == nil {
		streams.Stderr = os.Stderr
	}
	if streams.Dir == "" {
		streams.Dir = "."
	}
	return streams
}

func usage(output io.Writer) {
	fmt.Fprintln(output, `Usage:
  rtest init [--project <project>]
  rtest [--token <token>] exec [--project <project>] [--image <digest>] [--detach] [--timeout 15m] [--cpus 2] [--memory 4g] [--workdir <path>] [--env KEY=VALUE] -- <command> [arguments...]
  rtest [--token <token>] build [--project <project>] [-- <buildx arguments...>]
  rtest login --token <device-token>
  rtest logout
  rtest token create --name <device> [--user <id>] [--expires 720h]
  rtest token list
  rtest token revoke <token-id>
  rtest trust github create --project <project> --owner-id <id> --repository-id <id> --workflow-ref <glob> --ref <glob> --event <event> [--environment <name>]
  rtest trust github list --project <project>
  rtest trust github revoke <trust-id>
  rtest admin user create --name <name> [--admin]
  rtest admin project create --slug <slug> --name <name>
  rtest admin member add --project <project> --user <user-id>
  rtest run [--detach] [--timeout 15m] <suite> [-- <arguments...>]
  rtest run [--detach] [--timeout 15m] -- <command> [arguments...]
  rtest status [--json] <job-id>
  rtest logs <job-id>
  rtest cancel <job-id>
  rtest list [--repository <name>] [--limit 20] [--json]
  rtest doctor
  rtest version`)
}

func globalArgs(args []string) (string, []string, error) {
	token := ""
	for len(args) > 0 && args[0] == "--token" {
		if len(args) < 2 || args[1] == "" {
			return "", nil, errors.New("--token requires a value")
		}
		token, args = args[1], args[2:]
	}
	return token, args, nil
}

func fail(output io.Writer, err error) int {
	fmt.Fprintln(output, "rtest:", err)
	return 1
}

func failUsage(output io.Writer, message string) int {
	fmt.Fprintln(output, "rtest:", message)
	return 2
}

func value(input string) string {
	if input == "" {
		return "-"
	}
	return input
}

func humanBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/(1024*1024))
}
