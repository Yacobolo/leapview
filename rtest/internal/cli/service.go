package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/flidai/leapview/rtest/internal/authclient"
	"github.com/flidai/leapview/rtest/internal/buildkit"
	"github.com/flidai/leapview/rtest/internal/cas"
	"github.com/flidai/leapview/rtest/internal/config"
	"github.com/flidai/leapview/rtest/internal/control/controlclient"
	"github.com/flidai/leapview/rtest/internal/credentialfiles"
	rtestv1 "github.com/flidai/leapview/rtest/internal/gen/rtest/v1"
	"github.com/flidai/leapview/rtest/internal/gen/rtest/v1/rtestv1connect"
	"github.com/flidai/leapview/rtest/internal/profile"
	"github.com/flidai/leapview/rtest/internal/projectlink"
	"github.com/flidai/leapview/rtest/internal/protocol"
	"github.com/flidai/leapview/rtest/internal/workspace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func runService(ctx context.Context, settings config.Config, explicitToken string, args []string, streams IO) int {
	if len(args) == 0 {
		usage(streams.Stderr)
		return 2
	}
	switch args[0] {
	case "login":
		return serviceLogin(ctx, settings, explicitToken, args[1:], streams)
	case "logout":
		return serviceLogout(settings, streams)
	}
	selectedProject := ""
	if args[0] == "exec" || args[0] == "build" || args[0] == "list" {
		explicitProject, err := explicitProject(args[1:])
		if err != nil {
			return failUsage(streams.Stderr, err.Error())
		}
		selectedProject, err = projectlink.Resolve(ctx, streams.Dir, explicitProject, os.Getenv("RTEST_PROJECT"))
		if err != nil {
			return failUsage(streams.Stderr, err.Error())
		}
	}
	api, source, err := authenticatedServiceClient(ctx, settings, explicitToken, selectedProject)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	_ = source
	switch args[0] {
	case "init":
		return serviceInit(ctx, api, args[1:], streams)
	case "exec":
		return serviceExec(ctx, api, settings, selectedProject, args[1:], streams)
	case "build":
		return serviceBuild(ctx, api, settings, selectedProject, args[1:], streams)
	case "status":
		return serviceStatus(ctx, api, args[1:], streams)
	case "logs":
		return serviceLogs(ctx, api, args[1:], streams)
	case "cancel":
		return serviceCancel(ctx, api, args[1:], streams)
	case "list":
		return serviceList(ctx, api, selectedProject, args[1:], streams)
	case "doctor":
		return serviceDoctor(ctx, api, streams)
	case "token":
		return serviceToken(ctx, api, args[1:], streams)
	case "trust":
		return serviceTrust(ctx, api, args[1:], streams)
	case "admin":
		return serviceAdmin(ctx, api, args[1:], streams)
	default:
		return failUsage(streams.Stderr, "unknown service command "+args[0])
	}
}

func serviceAdmin(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	if len(args) < 2 {
		return failUsage(streams.Stderr, "admin requires user create, project create, or member add")
	}
	resource, command, args := args[0], args[1], args[2:]
	values := map[string]string{}
	admin := false
	for len(args) > 0 {
		if args[0] == "--admin" {
			admin, args = true, args[1:]
			continue
		}
		if len(args) < 2 {
			return failUsage(streams.Stderr, args[0]+" requires a value")
		}
		values[args[0]], args = args[1], args[2:]
	}
	switch resource + " " + command {
	case "user create":
		if values["--name"] == "" {
			return failUsage(streams.Stderr, "admin user create requires --name")
		}
		response, err := api.CreateUser(ctx, connect.NewRequest(&rtestv1.CreateUserRequest{Name: values["--name"], Admin: admin}))
		if err != nil {
			return fail(streams.Stderr, err)
		}
		return encode(streams, response.Msg.User)
	case "project create":
		if values["--slug"] == "" || values["--name"] == "" {
			return failUsage(streams.Stderr, "admin project create requires --slug and --name")
		}
		response, err := api.CreateProject(ctx, connect.NewRequest(&rtestv1.CreateProjectRequest{Slug: values["--slug"], Name: values["--name"]}))
		if err != nil {
			return fail(streams.Stderr, err)
		}
		return encode(streams, response.Msg.Project)
	case "member add":
		if values["--project"] == "" || values["--user"] == "" {
			return failUsage(streams.Stderr, "admin member add requires --project and --user")
		}
		if _, err := api.AddProjectMember(ctx, connect.NewRequest(&rtestv1.AddProjectMemberRequest{Project: values["--project"], UserId: values["--user"]})); err != nil {
			return fail(streams.Stderr, err)
		}
		fmt.Fprintf(streams.Stdout, "Added user %s to project %s\n", values["--user"], values["--project"])
		return 0
	default:
		return failUsage(streams.Stderr, "admin requires user create, project create, or member add")
	}
}

func authenticatedServiceClient(ctx context.Context, settings config.Config, explicitToken, project string) (rtestv1connect.ControlServiceClient, authclient.Source, error) {
	github := authclient.GitHubActions{}
	oidc := func(ctx context.Context) (string, error) {
		if !github.Available() {
			return "", errors.New("GitHub Actions OIDC identity is unavailable")
		}
		if project == "" {
			return "", errors.New("repository project selection is required for GitHub OIDC")
		}
		idToken, err := github.IDToken(ctx, settings.Service.OIDCAudience)
		if err != nil {
			return "", err
		}
		unauthenticated, err := controlclient.New(settings.URL, "", settings.Service.CACertFile)
		if err != nil {
			return "", err
		}
		response, err := unauthenticated.ExchangeGitHubOIDC(ctx, connect.NewRequest(&rtestv1.ExchangeGitHubOIDCRequest{Project: project, IdToken: idToken}))
		if err != nil {
			return "", err
		}
		return response.Msg.Token, nil
	}
	if !github.Available() {
		oidc = nil
	}
	token, source, err := authclient.Resolve(ctx, authclient.ResolveOptions{
		ExplicitToken: explicitToken, ServiceURL: settings.URL, Keyring: authclient.SystemKeyring{}, OIDC: oidc,
	})
	if err != nil {
		return nil, "", err
	}
	api, err := controlclient.New(settings.URL, token, settings.Service.CACertFile)
	return api, source, err
}

func serviceLogin(ctx context.Context, settings config.Config, explicitToken string, args []string, streams IO) int {
	token := explicitToken
	if token == "" && len(args) == 2 && args[0] == "--token" {
		token = args[1]
		args = nil
	}
	if token == "" {
		token = os.Getenv("RTEST_TOKEN")
	}
	if len(args) != 0 || token == "" {
		return failUsage(streams.Stderr, "login requires --token <device-token>")
	}
	api, err := controlclient.New(settings.URL, token, settings.Service.CACertFile)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if _, err := api.ListDeviceTokens(ctx, connect.NewRequest(&rtestv1.ListDeviceTokensRequest{})); err != nil {
		return fail(streams.Stderr, fmt.Errorf("validate device token: %w", err))
	}
	if err := authclient.StoreToken(authclient.SystemKeyring{}, settings.URL, token); err != nil {
		return fail(streams.Stderr, fmt.Errorf("store device token: %w", err))
	}
	fmt.Fprintln(streams.Stdout, "Authenticated to "+settings.URL)
	return 0
}

func serviceLogout(settings config.Config, streams IO) int {
	if err := authclient.DeleteToken(authclient.SystemKeyring{}, settings.URL); err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintln(streams.Stdout, "Removed the local rtest credential")
	return 0
}

func serviceInit(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	project := os.Getenv("RTEST_PROJECT")
	if len(args) == 2 && args[0] == "--project" && args[1] != "" {
		project = args[1]
	} else if len(args) != 0 {
		return failUsage(streams.Stderr, "init accepts only --project <project>")
	}
	response, err := api.ListProjects(ctx, connect.NewRequest(&rtestv1.ListProjectsRequest{}))
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("list authorized projects: %w", err))
	}
	if project == "" {
		if len(response.Msg.Projects) != 1 {
			available := make([]string, 0, len(response.Msg.Projects))
			for _, item := range response.Msg.Projects {
				available = append(available, item.Slug)
			}
			return failUsage(streams.Stderr, fmt.Sprintf("init requires --project when %d projects are available: %s", len(available), strings.Join(available, ", ")))
		}
		project = response.Msg.Projects[0].Slug
	}
	selected, err := selectAuthorizedProject(response.Msg.Projects, project)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	if _, err := profile.Root(ctx, streams.Dir); err != nil {
		return fail(streams.Stderr, err)
	}
	path, err := projectlink.Write(streams.Dir, selected.Slug)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintf(streams.Stdout, "Linked %s to rtest project %s\n", path, selected.Slug)
	return 0
}

func explicitProject(args []string) (string, error) {
	project := ""
	for index := 0; index < len(args); index++ {
		if args[index] == "--" {
			break
		}
		if args[index] != "--project" {
			continue
		}
		if index+1 >= len(args) || args[index+1] == "" || args[index+1] == "--" {
			return "", errors.New("--project requires a value")
		}
		candidate := args[index+1]
		if project != "" && project != candidate {
			return "", errors.New("conflicting --project values")
		}
		project = candidate
		index++
	}
	return project, nil
}

func authorizedProject(ctx context.Context, api rtestv1connect.ControlServiceClient, selector string) (*rtestv1.Project, error) {
	response, err := api.ListProjects(ctx, connect.NewRequest(&rtestv1.ListProjectsRequest{}))
	if err != nil {
		return nil, fmt.Errorf("authorize project %q: %w", selector, err)
	}
	return selectAuthorizedProject(response.Msg.Projects, selector)
}

func selectAuthorizedProject(projects []*rtestv1.Project, selector string) (*rtestv1.Project, error) {
	for _, project := range projects {
		if project.Id == selector || project.Slug == selector {
			return project, nil
		}
	}
	return nil, fmt.Errorf("not authorized for rtest project %q", selector)
}

type execOptions struct {
	project, image, cpus, memory, workdir string
	timeout                               time.Duration
	detach                                bool
	environment                           map[string]string
	command                               []string
}

func serviceExec(ctx context.Context, api rtestv1connect.ControlServiceClient, settings config.Config, project string, args []string, streams IO) int {
	options, err := parseExec(settings, project, args)
	if err != nil {
		return failUsage(streams.Stderr, err.Error())
	}
	if _, err := authorizedProject(ctx, api, project); err != nil {
		return fail(streams.Stderr, err)
	}
	root, err := profile.Root(ctx, streams.Dir)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	files, err := workspace.Files(ctx, root)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	idempotencyKey, err := jobID()
	if err != nil {
		return fail(streams.Stderr, err)
	}
	prepared, err := api.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		Project: options.project, Image: options.image, Command: options.command, WorkingDirectory: options.workdir,
		Environment: options.environment, Timeout: durationpb.New(options.timeout), Cpus: options.cpus, Memory: options.memory,
		IdempotencyKey: idempotencyKey,
	}))
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("prepare remote job: %w", err))
	}
	credentials, err := credentialfiles.Write(prepared.Msg.Cas.CaPem, prepared.Msg.Cas.CertificatePem, prepared.Msg.Cas.PrivateKeyPem)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	defer credentials.Cleanup()
	upload, err := cas.UploadConnection(ctx, cas.Connection{
		Service: grpcAddress(prepared.Msg.Cas.Endpoint), Instance: prepared.Msg.Cas.InstanceName,
		CACertFile: credentials.CA, ClientCertFile: credentials.Certificate, ClientKeyFile: credentials.Key,
		ServerName: prepared.Msg.Cas.ServerName,
	}, root, files)
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("upload inputs: %w", err))
	}
	started, err := api.StartJob(ctx, connect.NewRequest(&rtestv1.StartJobRequest{Id: prepared.Msg.Job.Id, RootDigest: upload.RootDigest}))
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("start remote job: %w", err))
	}
	fmt.Fprintf(streams.Stderr, "Backend: rtest shared service\nJob: %s\nInputs: %d files, %s\nTransfer: %s uploaded\n",
		started.Msg.Job.Id, upload.InputFiles, humanBytes(upload.TotalInputBytes), humanBytes(upload.TransferredBytes))
	if options.detach {
		return 0
	}
	return waitServiceJob(ctx, api, started.Msg.Job.Id, streams)
}

func parseExec(settings config.Config, project string, args []string) (execOptions, error) {
	options := execOptions{
		project: project, image: settings.Service.Image, cpus: settings.Service.CPUs,
		memory: settings.Service.Memory, workdir: ".", timeout: 30 * time.Minute, environment: map[string]string{},
	}
	for len(args) > 0 && args[0] != "--" {
		switch args[0] {
		case "--detach":
			options.detach, args = true, args[1:]
		case "--project", "--image", "--cpus", "--memory", "--workdir", "--timeout", "--env":
			if len(args) < 2 {
				return execOptions{}, errors.New(args[0] + " requires a value")
			}
			flag, value := args[0], args[1]
			args = args[2:]
			switch flag {
			case "--project":
				options.project = value
			case "--image":
				options.image = value
			case "--cpus":
				options.cpus = value
			case "--memory":
				options.memory = value
			case "--workdir":
				options.workdir = value
			case "--timeout":
				parsed, err := time.ParseDuration(value)
				if err != nil {
					return execOptions{}, errors.New("invalid --timeout")
				}
				options.timeout = parsed
			case "--env":
				key, item, ok := strings.Cut(value, "=")
				if !ok || key == "" {
					return execOptions{}, errors.New("--env requires KEY=VALUE")
				}
				options.environment[key] = item
			}
		default:
			return execOptions{}, errors.New("unknown exec option " + args[0])
		}
	}
	if len(args) == 0 || args[0] != "--" || len(args) == 1 {
		return execOptions{}, errors.New("exec requires -- <command> [arguments...]")
	}
	options.command = append([]string(nil), args[1:]...)
	if options.project == "" || options.image == "" {
		return execOptions{}, errors.New("project selection and image are required")
	}
	return options, nil
}

func waitServiceJob(ctx context.Context, api rtestv1connect.ControlServiceClient, id string, streams IO) int {
	var terminal *rtestv1.Job
	var offset int64
	retryDelay := 250 * time.Millisecond
	for terminal == nil {
		stream, err := api.StreamJobLogs(ctx, connect.NewRequest(&rtestv1.StreamJobLogsRequest{Id: id, Offset: offset}))
		if err == nil {
			for stream.Receive() {
				message := stream.Msg()
				if len(message.Data) > 0 {
					if _, err := streams.Stdout.Write(message.Data); err != nil {
						return fail(streams.Stderr, err)
					}
				}
				if message.NextOffset >= offset {
					offset = message.NextOffset
				}
				if message.TerminalJob != nil {
					terminal = message.TerminalJob
				}
			}
			err = stream.Err()
		}
		if terminal != nil {
			break
		}
		if ctx.Err() != nil {
			cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = api.CancelJob(cancelCtx, connect.NewRequest(&rtestv1.CancelJobRequest{Id: id}))
			cancel()
			return fail(streams.Stderr, fmt.Errorf("stream job logs: %w", ctx.Err()))
		}
		if err != nil && connect.CodeOf(err) != connect.CodeUnavailable {
			return fail(streams.Stderr, fmt.Errorf("stream job logs: %w", err))
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			continue
		case <-timer.C:
		}
		if retryDelay < 5*time.Second {
			retryDelay *= 2
			if retryDelay > 5*time.Second {
				retryDelay = 5 * time.Second
			}
		}
	}
	job := protocolJob(terminal)
	printCompletion(streams.Stderr, job)
	return clientExitCode(job)
}

func serviceBuild(ctx context.Context, api rtestv1connect.ControlServiceClient, settings config.Config, project string, args []string, streams IO) int {
	if len(args) >= 2 && args[0] == "--project" {
		project, args = args[1], args[2:]
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if project == "" {
		return failUsage(streams.Stderr, "build requires repository project selection or --project")
	}
	if _, err := authorizedProject(ctx, api, project); err != nil {
		return fail(streams.Stderr, err)
	}
	if len(args) == 0 {
		args = []string{"."}
	}
	if _, err := profile.Root(ctx, streams.Dir); err != nil {
		return fail(streams.Stderr, err)
	}
	random, err := jobID()
	if err != nil {
		return fail(streams.Stderr, err)
	}
	prepared, err := api.PrepareBuild(ctx, connect.NewRequest(&rtestv1.PrepareBuildRequest{Project: project, IdempotencyKey: random}))
	if err != nil {
		return fail(streams.Stderr, fmt.Errorf("prepare remote build: %w", err))
	}
	connection := prepared.Msg.Buildkit
	credentials, err := credentialfiles.Write(connection.CaPem, connection.CertificatePem, connection.PrivateKeyPem)
	if err != nil {
		return fail(streams.Stderr, err)
	}
	defer credentials.Cleanup()
	builderName := strings.Replace(random, "rtest-", "rtest-build-", 1)
	address := connection.Endpoint
	if !strings.Contains(address, "://") {
		address = "tcp://" + address
	}
	fmt.Fprintf(streams.Stderr, "Backend: native Buildx via rtest mTLS\nBuild: %s\nBuilder: %s\n", prepared.Msg.Build.Id, address)
	code, runErr := buildkit.RunWithTLS(ctx, os.Getenv("RTEST_DOCKER"), address, builderName, streams.Dir, args, buildkit.TLS{
		CA: credentials.CA, Certificate: credentials.Certificate, Key: credentials.Key, ServerName: connection.ServerName,
	}, streams.Stdout, streams.Stderr)
	cancelled := ctx.Err() != nil
	finishCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	_, finishErr := api.FinishBuild(finishCtx, connect.NewRequest(&rtestv1.FinishBuildRequest{Id: prepared.Msg.Build.Id, ExitCode: int32(code), Cancelled: cancelled}))
	cancel()
	if runErr != nil {
		return fail(streams.Stderr, fmt.Errorf("remote build: %w", runErr))
	}
	if finishErr != nil {
		return fail(streams.Stderr, fmt.Errorf("finish build record: %w", finishErr))
	}
	return code
}

func serviceStatus(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	jsonOutput, id, err := jobArgs(args)
	if err != nil {
		return failUsage(streams.Stderr, "status "+err.Error())
	}
	response, err := api.GetJob(ctx, connect.NewRequest(&rtestv1.GetJobRequest{Id: id}))
	if err != nil {
		return fail(streams.Stderr, err)
	}
	job := protocolJob(response.Msg.Job)
	if jsonOutput {
		return encode(streams, job)
	}
	printJob(streams.Stdout, job)
	return 0
}

func serviceLogs(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	if len(args) != 1 {
		return failUsage(streams.Stderr, "logs requires exactly one job ID")
	}
	return waitServiceJob(ctx, api, args[0], streams)
}

func serviceCancel(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	if len(args) != 1 {
		return failUsage(streams.Stderr, "cancel requires exactly one job ID")
	}
	response, err := api.CancelJob(ctx, connect.NewRequest(&rtestv1.CancelJobRequest{Id: args[0]}))
	if err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintf(streams.Stdout, "Job %s: %s\n", args[0], strings.ToLower(strings.TrimPrefix(response.Msg.Job.Status.String(), "JOB_STATUS_")))
	return 0
}

func serviceList(ctx context.Context, api rtestv1connect.ControlServiceClient, project string, args []string, streams IO) int {
	limit, jsonOutput := 20, false
	for len(args) > 0 {
		switch args[0] {
		case "--project":
			if len(args) < 2 {
				return failUsage(streams.Stderr, "--project requires a value")
			}
			project, args = args[1], args[2:]
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
	if project == "" {
		return failUsage(streams.Stderr, "list requires a project")
	}
	response, err := api.ListJobs(ctx, connect.NewRequest(&rtestv1.ListJobsRequest{Project: project, PageSize: int32(limit)}))
	if err != nil {
		return fail(streams.Stderr, err)
	}
	jobs := make([]protocol.Job, 0, len(response.Msg.Jobs))
	for _, item := range response.Msg.Jobs {
		jobs = append(jobs, protocolJob(item))
	}
	if jsonOutput {
		return encode(streams, jobs)
	}
	writer := tabwriter.NewWriter(streams.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "JOB\tSTATUS\tPROJECT\tAGE\tDURATION")
	for _, job := range jobs {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", job.ID, job.Status, job.Repository, time.Since(job.CreatedAt).Round(time.Second), duration(job))
	}
	_ = writer.Flush()
	return 0
}

func serviceDoctor(ctx context.Context, api rtestv1connect.ControlServiceClient, streams IO) int {
	info, err := api.GetServiceInfo(ctx, connect.NewRequest(&rtestv1.GetServiceInfoRequest{}))
	if err != nil {
		return fail(streams.Stderr, err)
	}
	fmt.Fprintf(streams.Stdout, "rtest %s\nconnection: ok (Connect over HTTPS)\nserver: %s\n", version, info.Msg.Version)
	return 0
}

func serviceToken(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	if len(args) == 0 {
		return failUsage(streams.Stderr, "token requires create, list, or revoke")
	}
	switch args[0] {
	case "create":
		name, userID := "", ""
		var expires *timestamppb.Timestamp
		for args = args[1:]; len(args) > 0; {
			if len(args) < 2 {
				return failUsage(streams.Stderr, args[0]+" requires a value")
			}
			switch args[0] {
			case "--name":
				name = args[1]
			case "--user":
				userID = args[1]
			case "--expires":
				duration, err := time.ParseDuration(args[1])
				if err != nil || duration < time.Minute {
					return failUsage(streams.Stderr, "invalid token expiry")
				}
				expires = timestamppb.New(time.Now().Add(duration))
			default:
				return failUsage(streams.Stderr, "unknown token create option "+args[0])
			}
			args = args[2:]
		}
		if name == "" {
			return failUsage(streams.Stderr, "token create requires --name")
		}
		response, err := api.CreateDeviceToken(ctx, connect.NewRequest(&rtestv1.CreateDeviceTokenRequest{Name: name, UserId: userID, ExpiresAt: expires}))
		if err != nil {
			return fail(streams.Stderr, err)
		}
		fmt.Fprintf(streams.Stdout, "%s\n", response.Msg.Token)
		fmt.Fprintf(streams.Stderr, "Created device token %s; its secret will not be shown again.\n", response.Msg.TokenMetadata.Id)
		return 0
	case "list":
		if len(args) != 1 {
			return failUsage(streams.Stderr, "token list accepts no arguments")
		}
		response, err := api.ListDeviceTokens(ctx, connect.NewRequest(&rtestv1.ListDeviceTokensRequest{}))
		if err != nil {
			return fail(streams.Stderr, err)
		}
		return encode(streams, response.Msg.Tokens)
	case "revoke":
		if len(args) != 2 {
			return failUsage(streams.Stderr, "token revoke requires one token ID")
		}
		if _, err := api.RevokeDeviceToken(ctx, connect.NewRequest(&rtestv1.RevokeDeviceTokenRequest{Id: args[1]})); err != nil {
			return fail(streams.Stderr, err)
		}
		fmt.Fprintln(streams.Stdout, "Revoked device token "+args[1])
		return 0
	default:
		return failUsage(streams.Stderr, "unknown token command "+args[0])
	}
}

func serviceTrust(ctx context.Context, api rtestv1connect.ControlServiceClient, args []string, streams IO) int {
	if len(args) < 2 || args[0] != "github" {
		return failUsage(streams.Stderr, "trust requires github create, list, or revoke")
	}
	command, args := args[1], args[2:]
	switch command {
	case "create":
		values := map[string]string{}
		var events []string
		for len(args) > 0 {
			if len(args) < 2 {
				return failUsage(streams.Stderr, args[0]+" requires a value")
			}
			if args[0] == "--event" {
				events = append(events, args[1])
			} else {
				values[args[0]] = args[1]
			}
			args = args[2:]
		}
		for _, required := range []string{"--project", "--owner-id", "--repository-id", "--workflow-ref", "--ref"} {
			if values[required] == "" {
				return failUsage(streams.Stderr, "trust github create requires "+required)
			}
		}
		if len(events) == 0 {
			return failUsage(streams.Stderr, "trust github create requires at least one --event")
		}
		response, err := api.CreateGitHubTrust(ctx, connect.NewRequest(&rtestv1.CreateGitHubTrustRequest{
			Project: values["--project"], RepositoryOwnerId: values["--owner-id"], RepositoryId: values["--repository-id"],
			WorkflowRef: values["--workflow-ref"], Ref: values["--ref"], Environment: values["--environment"], Events: events,
		}))
		if err != nil {
			return fail(streams.Stderr, err)
		}
		return encode(streams, response.Msg.Trust)
	case "list":
		project := ""
		if len(args) == 2 && args[0] == "--project" {
			project = args[1]
		} else if len(args) != 0 {
			return failUsage(streams.Stderr, "trust github list accepts only --project")
		}
		response, err := api.ListGitHubTrusts(ctx, connect.NewRequest(&rtestv1.ListGitHubTrustsRequest{Project: project}))
		if err != nil {
			return fail(streams.Stderr, err)
		}
		return encode(streams, response.Msg.Trusts)
	case "revoke":
		if len(args) != 1 {
			return failUsage(streams.Stderr, "trust github revoke requires one trust ID")
		}
		if _, err := api.RevokeGitHubTrust(ctx, connect.NewRequest(&rtestv1.RevokeGitHubTrustRequest{Id: args[0]})); err != nil {
			return fail(streams.Stderr, err)
		}
		fmt.Fprintln(streams.Stdout, "Revoked GitHub trust "+args[0])
		return 0
	default:
		return failUsage(streams.Stderr, "unknown GitHub trust command "+command)
	}
}

func grpcAddress(endpoint string) string {
	return strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "grpcs://")
}

func protocolJob(input *rtestv1.Job) protocol.Job {
	job := protocol.Job{
		ID: input.Id, Repository: input.ProjectId, Suite: "exec", Runner: "oci", Command: append([]string(nil), input.Command...),
		SourceDigest: input.RootDigest, Status: protocolStatus(input.Status),
		CancelRequested: input.CancelRequested, ErrorMessage: input.ErrorMessage, WorkerID: input.WorkerId,
	}
	if input.Timeout != nil {
		job.TimeoutSeconds = int(input.Timeout.AsDuration().Seconds())
	}
	if input.CreatedAt != nil {
		job.CreatedAt = input.CreatedAt.AsTime()
	}
	job.StartedAt, job.FinishedAt = protoTime(input.StartedAt), protoTime(input.FinishedAt)
	if input.ExitCode != nil {
		value := int(*input.ExitCode)
		job.ExitCode = &value
	}
	return job
}

func protocolStatus(status rtestv1.JobStatus) protocol.Status {
	switch status {
	case rtestv1.JobStatus_JOB_STATUS_PREPARING:
		return protocol.StatusPreparing
	case rtestv1.JobStatus_JOB_STATUS_QUEUED:
		return protocol.StatusQueued
	case rtestv1.JobStatus_JOB_STATUS_RUNNING:
		return protocol.StatusRunning
	case rtestv1.JobStatus_JOB_STATUS_SUCCEEDED:
		return protocol.StatusSucceeded
	case rtestv1.JobStatus_JOB_STATUS_FAILED:
		return protocol.StatusFailed
	case rtestv1.JobStatus_JOB_STATUS_CANCELLED:
		return protocol.StatusCancelled
	case rtestv1.JobStatus_JOB_STATUS_TIMED_OUT:
		return protocol.StatusTimedOut
	case rtestv1.JobStatus_JOB_STATUS_LOST:
		return protocol.StatusLost
	default:
		return protocol.StatusLost
	}
}

func protoTime(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	result := value.AsTime()
	return &result
}

func clientExitCode(job protocol.Job) int {
	if job.ExitCode != nil {
		return *job.ExitCode
	}
	if job.Status == protocol.StatusSucceeded {
		return 0
	}
	return 1
}
