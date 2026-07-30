package composectl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type qualificationProcess struct {
	dir         string
	executable  string
	environment []string
}

type qualificationCommandRequest struct {
	Directory   string
	Executable  string
	Environment []string
	Stdin       io.Reader
	Arguments   []string
}

type qualificationCommandExecutor interface {
	Execute(context.Context, qualificationCommandRequest) ([]byte, error)
}

type osQualificationCommandExecutor struct{}

func (osQualificationCommandExecutor) Execute(
	ctx context.Context,
	request qualificationCommandRequest,
) ([]byte, error) {
	command := exec.CommandContext(ctx, request.Executable, request.Arguments...)
	command.Dir = request.Directory
	command.Env = request.Environment
	if len(command.Env) == 0 {
		command.Env = os.Environ()
	}
	command.Stdin = request.Stdin
	return command.CombinedOutput()
}

func (p qualificationProcess) Run(
	ctx context.Context,
	stdin io.Reader,
	executor qualificationCommandExecutor,
	args ...string,
) ([]byte, error) {
	if executor == nil {
		executor = osQualificationCommandExecutor{}
	}
	output, err := executor.Execute(ctx, qualificationCommandRequest{
		Directory: p.dir, Executable: p.executable,
		Environment: append([]string(nil), p.environment...),
		Stdin:       stdin, Arguments: append([]string(nil), args...),
	})
	if err != nil {
		commandText := string(redactQualificationBytes(
			[]byte(p.executable + " " + strings.Join(args, " ")),
		))
		return output, fmt.Errorf(
			"%s: %w: %s",
			commandText,
			err,
			redactQualificationLog(output, 100),
		)
	}
	return output, nil
}

func (c *Controller) qualificationDocker(
	ctx context.Context,
	stdin io.Reader,
	args ...string,
) ([]byte, error) {
	return qualificationProcess{
		dir:         c.root,
		executable:  c.dockerBin,
		environment: os.Environ(),
	}.Run(ctx, stdin, c.qualificationExecutor, args...)
}

func qualificationComposeArguments(root string, args ...string) ([]string, error) {
	https, err := envFileValue(filepath.Join(root, deploymentEnvName), "COMPOSE_HTTPS")
	if err != nil {
		return nil, err
	}
	result := []string{
		"compose",
		"--project-directory", root,
		"--env-file", filepath.Join(root, deploymentEnvName),
		"--file", filepath.Join(root, "compose.yaml"),
	}
	if https == "1" {
		result = append(result, "--file", filepath.Join(root, "compose.https.yaml"))
	}
	return append(result, args...), nil
}

func (c *Controller) qualificationCompose(
	ctx context.Context,
	root string,
	args ...string,
) ([]byte, error) {
	commandArgs, err := qualificationComposeArguments(root, args...)
	if err != nil {
		return nil, err
	}
	return c.qualificationDocker(ctx, nil, commandArgs...)
}

type qualificationJSONWorker struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	stderr  *boundedQualificationBuffer
	nextID  int
	mu      sync.Mutex
}

func startQualificationJSONWorker(
	ctx context.Context,
	dir string,
	environment []string,
	executable string,
	arguments ...string,
) (*qualificationJSONWorker, error) {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = dir
	command.Env = environment
	if len(command.Env) == 0 {
		command.Env = os.Environ()
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr := &boundedQualificationBuffer{maxBytes: 256 << 10}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &qualificationJSONWorker{
		command: command,
		stdin:   stdin,
		stdout:  bufio.NewScanner(stdout),
		stderr:  stderr,
	}, nil
}

func (w *qualificationJSONWorker) Call(
	method string,
	params any,
	result any,
	onEvent func(string, json.RawMessage) error,
) error {
	return w.CallContext(context.Background(), method, params, result, onEvent)
}

func (w *qualificationJSONWorker) CallContext(
	ctx context.Context,
	method string,
	params any,
	result any,
	onEvent func(string, json.RawMessage) error,
) error {
	if ctx == nil {
		return fmt.Errorf("%s worker context is required", method)
	}
	done := make(chan error, 1)
	go func() {
		done <- w.call(method, params, result, onEvent)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = w.Kill()
		<-done
		return fmt.Errorf("%s worker call: %w", method, ctx.Err())
	}
}

func (w *qualificationJSONWorker) call(
	method string,
	params any,
	result any,
	onEvent func(string, json.RawMessage) error,
) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nextID++
	request := struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{ID: w.nextID, Method: method, Params: params}
	if err := json.NewEncoder(w.stdin).Encode(request); err != nil {
		return fmt.Errorf("write %s worker request: %w", method, err)
	}
	for w.stdout.Scan() {
		var response struct {
			ID     int             `json:"id"`
			Event  string          `json:"event,omitempty"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  string          `json:"error,omitempty"`
		}
		if err := json.Unmarshal(w.stdout.Bytes(), &response); err != nil {
			return fmt.Errorf("decode %s worker response: %w", method, err)
		}
		if response.ID != request.ID {
			return fmt.Errorf("%s worker response id %d does not match request %d", method, response.ID, request.ID)
		}
		if response.Event != "" {
			if onEvent == nil {
				return fmt.Errorf("%s worker emitted unexpected event %q", method, response.Event)
			}
			if err := onEvent(response.Event, response.Result); err != nil {
				return err
			}
			continue
		}
		if response.Error != "" {
			return fmt.Errorf("%s worker: %s", method, response.Error)
		}
		if result != nil && len(response.Result) > 0 {
			if err := json.Unmarshal(response.Result, result); err != nil {
				return fmt.Errorf("decode %s worker result: %w", method, err)
			}
		}
		return nil
	}
	if err := w.stdout.Err(); err != nil {
		return fmt.Errorf("read %s worker response: %w", method, err)
	}
	return fmt.Errorf("%s worker exited: %s", method, w.stderr.String())
}

func (w *qualificationJSONWorker) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.stdin.Close()
	waitErr := w.command.Wait()
	if waitErr != nil {
		return fmt.Errorf("qualification worker exit: %w: %s", waitErr, w.stderr.String())
	}
	return nil
}

func (w *qualificationJSONWorker) Kill() error {
	if w == nil || w.command == nil || w.command.Process == nil {
		return nil
	}
	_ = w.stdin.Close()
	if err := w.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	_ = w.command.Wait()
	return nil
}

type boundedQualificationBuffer struct {
	maxBytes int
	bytes.Buffer
}

func (b *boundedQualificationBuffer) Write(contents []byte) (int, error) {
	written := len(contents)
	_, _ = b.Buffer.Write(contents)
	if b.maxBytes > 0 && b.Buffer.Len() > b.maxBytes {
		value := append([]byte(nil), b.Buffer.Bytes()[b.Buffer.Len()-b.maxBytes:]...)
		b.Buffer.Reset()
		_, _ = b.Buffer.Write(value)
	}
	return written, nil
}

func copyQualificationFile(source, destination string, mode fs.FileMode) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	return os.WriteFile(destination, contents, mode)
}

func copyQualificationTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyQualificationFile(path, target, info.Mode().Perm())
	})
}

func writeQualificationJSON(path string, value any) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return writePrivateAtomic(path, contents)
}

func readQualificationJSON(path string, value any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(contents, value); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func appendOrReplaceQualificationEnv(path, key, value string) error {
	if err := validateEnvLineValue(key, value); err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(contents), "\n")
	found := false
	for index, line := range lines {
		name, _, present := strings.Cut(line, "=")
		if present && name == key {
			lines[index] = key + "=" + value
			found = true
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, key+"=", "")
		lines[len(lines)-2] = key + "=" + value
	}
	return writePrivateAtomic(path, []byte(strings.Join(lines, "\n")))
}

func qualificationWait(
	ctx context.Context,
	interval time.Duration,
	operation func(context.Context) (bool, error),
) error {
	for {
		complete, err := operation(ctx)
		if err != nil {
			return err
		}
		if complete {
			return nil
		}
		if err := sleepContext(ctx, interval); err != nil {
			return err
		}
	}
}

func parseQualificationInteger(value, label string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", label, err)
	}
	return parsed, nil
}

func joinQualificationError(primary error, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	if primary == nil {
		return cleanup
	}
	return errors.Join(primary, fmt.Errorf("qualification cleanup: %w", cleanup))
}
