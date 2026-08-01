package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/flidai/leapview/rtest/internal/config"
)

type Tunnel struct {
	command *exec.Cmd
	done    <-chan error
}

func Open(ctx context.Context, settings config.Config) (string, *Tunnel, error) {
	if settings.URL != "" {
		return settings.URL, nil, nil
	}
	if settings.SSH == nil {
		return "", nil, errors.New("SSH configuration is required")
	}
	address, sshTunnel, err := Forward(ctx, settings.SSH, settings.SSH.RemoteAddress)
	return "http://" + address, sshTunnel, err
}

// Forward opens an SSH local port forward and returns its local host:port.
// Keeping the transport separate lets HTTP, REAPI gRPC, and BuildKit use the
// same authenticated SSH boundary without inventing another network protocol.
func Forward(ctx context.Context, ssh *config.SSH, remoteAddress string) (string, *Tunnel, error) {
	if ssh == nil {
		return "", nil, errors.New("SSH configuration is required")
	}
	if remoteAddress == "" {
		return "", nil, errors.New("SSH remote address is required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("reserve tunnel port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return "", nil, err
	}
	forward := "127.0.0.1:" + strconv.Itoa(port) + ":" + remoteAddress
	args := []string{
		"-o", "IgnoreUnknown=UseKeychain",
		"-o", "BatchMode=yes",
		"-o", "UseKeychain=yes",
		"-o", "AddKeysToAgent=yes",
		"-o", "IdentitiesOnly=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "StrictHostKeyChecking=accept-new",
		"-N", "-L", forward,
	}
	if ssh.IdentityFile != "" {
		args = append(args, "-i", ssh.IdentityFile)
	}
	args = append(args, ssh.User+"@"+ssh.Host)
	// The forwarding process must outlive cancellation of an individual remote
	// action long enough for the caller to send REAPI CancelOperation. Close is
	// the owner of the established tunnel lifecycle.
	command := exec.Command("ssh", args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return "", nil, fmt.Errorf("start SSH tunnel: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = command.Process.Kill()
			return "", nil, ctx.Err()
		case err := <-done:
			if err == nil {
				return "", nil, errors.New("SSH tunnel exited before becoming ready")
			}
			return "", nil, fmt.Errorf("SSH tunnel exited before becoming ready: %w: %s", err, stderr.String())
		case <-deadline.C:
			_ = command.Process.Kill()
			return "", nil, errors.New("timed out waiting for SSH tunnel")
		case <-ticker.C:
			connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
			if err == nil {
				_ = connection.Close()
				return address, &Tunnel{command: command, done: done}, nil
			}
		}
	}
}

func (t *Tunnel) Close() error {
	if t == nil || t.command == nil || t.command.Process == nil {
		return nil
	}
	_ = t.command.Process.Signal(os.Interrupt)
	select {
	case err := <-t.done:
		if err == nil {
			return nil
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil
		}
		return err
	case <-time.After(2 * time.Second):
		return t.command.Process.Kill()
	}
}
