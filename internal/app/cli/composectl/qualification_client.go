package composectl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type QualificationClientWorkerOptions struct {
	Target          string
	Project         string
	SourceRevision  string
	KeyringPassword string
}

type qualificationWorkerRequest struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type qualificationWorkerResponse struct {
	ID     int    `json:"id"`
	Event  string `json:"event,omitempty"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type qualificationLoginChallenge struct {
	VerificationURL string `json:"verificationUrl"`
	UserCode        string `json:"userCode"`
}

func parseQualificationCandidate(output, sourceRevision string) (QualificationCandidate, error) {
	var wire struct {
		SchemaVersion    int    `json:"schemaVersion"`
		CandidateID      string `json:"candidateId"`
		Revision         int64  `json:"revision"`
		TargetID         string `json:"targetId"`
		PrincipalID      string `json:"principalId"`
		ArtifactDigest   string `json:"artifactDigest"`
		ProvenanceDigest string `json:"provenanceDigest"`
		PreviewURL       string `json:"previewUrl"`
	}
	if err := json.Unmarshal([]byte(output), &wire); err != nil {
		return QualificationCandidate{}, fmt.Errorf("decode dev result: %w", err)
	}
	if wire.SchemaVersion != 1 {
		return QualificationCandidate{}, fmt.Errorf("unsupported dev result schema %d", wire.SchemaVersion)
	}
	result := QualificationCandidate{
		ID: wire.CandidateID, Revision: wire.Revision, TargetID: wire.TargetID,
		PrincipalID: wire.PrincipalID, ArtifactDigest: wire.ArtifactDigest,
		ProvenanceDigest: wire.ProvenanceDigest, PreviewURL: wire.PreviewURL,
		SourceRevision: strings.TrimSpace(sourceRevision),
	}
	if result.ID == "" || result.Revision <= 0 || result.TargetID == "" ||
		result.PrincipalID == "" || result.PreviewURL == "" {
		return QualificationCandidate{}, fmt.Errorf("incomplete candidate output")
	}
	for name, value := range map[string]string{
		"artifact digest":   result.ArtifactDigest,
		"provenance digest": result.ProvenanceDigest,
	} {
		if !strings.HasPrefix(value, "sha256:") || len(value) != 71 {
			return QualificationCandidate{}, fmt.Errorf("invalid %s %q", name, value)
		}
	}
	return result, nil
}

func parseQualificationPublication(output string) (QualificationPublication, error) {
	var wire struct {
		SchemaVersion int `json:"schemaVersion"`
		QualificationPublication
	}
	if err := json.Unmarshal([]byte(output), &wire); err != nil {
		return QualificationPublication{}, fmt.Errorf("decode publish result: %w", err)
	}
	if wire.SchemaVersion != 1 {
		return QualificationPublication{}, fmt.Errorf("unsupported publish result schema %d", wire.SchemaVersion)
	}
	result := wire.QualificationPublication
	if result.DeploymentID == "" || result.Status == "" || result.CandidateID == "" ||
		result.CandidateRevision <= 0 || result.TargetID == "" || result.PrincipalID == "" ||
		result.ArtifactDigest == "" || result.ReleaseDigest == "" {
		return QualificationPublication{}, fmt.Errorf("incomplete publication output")
	}
	return result, nil
}

func (c *Controller) RunQualificationClientWorker(
	ctx context.Context,
	options QualificationClientWorkerOptions,
) error {
	options.Target = strings.TrimSpace(options.Target)
	options.Project = strings.TrimSpace(options.Project)
	options.SourceRevision = strings.TrimSpace(options.SourceRevision)
	options.KeyringPassword = strings.TrimSpace(options.KeyringPassword)
	if options.Target == "" || options.Project == "" || options.KeyringPassword == "" {
		return fmt.Errorf("qualification client worker requires target, project, and keyring password")
	}
	runtimeDir, err := os.MkdirTemp("", "leapview-qualification-keyring-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(runtimeDir)
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		return err
	}
	environment := append(os.Environ(), "XDG_RUNTIME_DIR="+runtimeDir)
	environment, err = startQualificationKeyring(ctx, environment, options.KeyringPassword)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(c.stdin)
	encoder := json.NewEncoder(c.stdout)
	for {
		var request qualificationWorkerRequest
		if err := decoder.Decode(&request); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode qualification client request: %w", err)
		}
		var result any
		var runErr error
		switch request.Method {
		case "login":
			runErr = runQualificationLogin(
				ctx,
				environment,
				options,
				request.ID,
				encoder,
			)
			result = map[string]bool{"authenticated": runErr == nil}
		case "dev":
			var output string
			output, runErr = runQualificationCLI(
				ctx,
				environment,
				"dev",
				qualificationDevArguments(options)...,
			)
			if runErr == nil {
				result, runErr = parseQualificationCandidate(output, options.SourceRevision)
			}
		case "publish":
			var output string
			output, runErr = runQualificationCLI(
				ctx,
				environment,
				"publish",
				"--project", options.Project,
				"--target", options.Target,
				"--format", "json",
			)
			if runErr == nil {
				result, runErr = parseQualificationPublication(output)
			}
		default:
			runErr = fmt.Errorf("unsupported qualification client method %q", request.Method)
		}
		response := qualificationWorkerResponse{ID: request.ID, Result: result}
		if runErr != nil {
			response.Result = nil
			response.Error = runErr.Error()
		}
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode qualification client response: %w", err)
		}
	}
}

func qualificationDevArguments(options QualificationClientWorkerOptions) []string {
	arguments := []string{
		"--once",
		"--no-browser",
		"--project", options.Project,
		"--target", options.Target,
		"--format", "json",
	}
	if options.SourceRevision != "" {
		arguments = append(arguments, "--source-revision", options.SourceRevision)
	}
	return arguments
}

func startQualificationKeyring(
	ctx context.Context,
	environment []string,
	password string,
) ([]string, error) {
	unlock := exec.CommandContext(ctx, "gnome-keyring-daemon", "--unlock")
	unlock.Env = environment
	unlock.Stdin = strings.NewReader(password)
	unlockOutput, err := unlock.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unlock qualification keyring: %w: %s", err, unlockOutput)
	}
	environment = mergeQualificationEnvironment(environment, string(unlockOutput))

	start := exec.CommandContext(ctx, "gnome-keyring-daemon", "--start", "--components=secrets")
	start.Env = environment
	startOutput, err := start.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("start qualification keyring: %w: %s", err, startOutput)
	}
	return mergeQualificationEnvironment(environment, string(startOutput)), nil
}

func mergeQualificationEnvironment(environment []string, output string) []string {
	values := make(map[string]string, len(environment))
	order := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if _, exists := values[name]; !exists {
			order = append(order, name)
		}
		values[name] = value
	}
	for _, line := range strings.Split(output, "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		value = strings.Trim(value, "'\"")
		if _, exists := values[name]; !exists {
			order = append(order, name)
		}
		values[name] = value
	}
	result := make([]string, 0, len(order))
	for _, name := range order {
		result = append(result, name+"="+values[name])
	}
	return result
}

func runQualificationLogin(
	ctx context.Context,
	environment []string,
	options QualificationClientWorkerOptions,
	requestID int,
	encoder *json.Encoder,
) error {
	command := exec.CommandContext(
		ctx,
		"leapview",
		"login",
		options.Target,
		"--project", options.Project,
		"--no-browser",
		"--format", "json",
	)
	command.Env = environment
	var output bytes.Buffer
	reader, writer := io.Pipe()
	command.Stdout = io.MultiWriter(&output, writer)
	command.Stderr = io.MultiWriter(&output, writer)
	if err := command.Start(); err != nil {
		_ = writer.Close()
		return fmt.Errorf("start leapview login: %w", err)
	}
	scanned := make(chan error, 1)
	go func() {
		defer close(scanned)
		scanner := bufio.NewScanner(reader)
		challengeSent := false
		authenticated := false
		for scanner.Scan() {
			var event struct {
				SchemaVersion   int    `json:"schemaVersion"`
				Type            string `json:"type"`
				VerificationURL string `json:"verificationUrl"`
				UserCode        string `json:"userCode"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				scanned <- fmt.Errorf("decode login event: %w", err)
				return
			}
			if event.SchemaVersion != 1 {
				scanned <- fmt.Errorf("unsupported login event schema %d", event.SchemaVersion)
				return
			}
			switch event.Type {
			case "deviceChallenge":
				if challengeSent || event.VerificationURL == "" || event.UserCode == "" {
					scanned <- fmt.Errorf("invalid device challenge event")
					return
				}
				challengeSent = true
				if err := encoder.Encode(qualificationWorkerResponse{
					ID:    requestID,
					Event: "device_challenge",
					Result: qualificationLoginChallenge{
						VerificationURL: event.VerificationURL,
						UserCode:        event.UserCode,
					},
				}); err != nil {
					scanned <- err
					return
				}
			case "authenticated":
				authenticated = true
			default:
				scanned <- fmt.Errorf("unexpected login event %q", event.Type)
				return
			}
		}
		if !challengeSent || !authenticated {
			scanned <- fmt.Errorf("login event stream is incomplete")
			return
		}
		scanned <- scanner.Err()
	}()
	waitErr := command.Wait()
	_ = writer.Close()
	scanErr := <-scanned
	_ = reader.Close()
	if scanErr != nil {
		return fmt.Errorf("read leapview login: %w", scanErr)
	}
	if waitErr != nil {
		return fmt.Errorf("leapview login: %w: %s", waitErr, redactQualificationLog(output.Bytes(), 100))
	}
	return nil
}

func runQualificationCLI(
	ctx context.Context,
	environment []string,
	commandName string,
	arguments ...string,
) (string, error) {
	command := exec.CommandContext(ctx, "leapview", append([]string{commandName}, arguments...)...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"leapview %s: %w: %s",
			commandName,
			err,
			redactQualificationLog(output, 100),
		)
	}
	return string(output), nil
}
