package proof

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

const redisImage = "redis:7.4.2-alpine@sha256:02419de7eddf55aa5bcf49efb74e88fa8d931b4d77c07eff8a6b2144472b6952"

func TestDirtyWorktreeRunsWithRedis(t *testing.T) {
	dirty, err := os.ReadFile("proof.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(dirty)) != "dirty worktree reached remote worker" {
		t.Fatalf("remote snapshot did not contain dirty bytes: %q", dirty)
	}
	if _, err := os.Stat("untracked.txt"); err != nil {
		t.Fatalf("untracked file did not reach remote worker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	untrackedPath, err := filepath.Abs("untracked.txt")
	if err != nil {
		t.Fatal(err)
	}
	redis, err := testcontainers.Run(ctx, redisImage,
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(wait.ForLog("Ready to accept connections").WithStartupTimeout(60*time.Second)),
		testcontainers.WithHostConfigModifier(func(config *containertypes.HostConfig) {
			config.Mounts = append(config.Mounts, mount.Mount{
				Type: mount.TypeBind, Source: untrackedPath, Target: "/rtest-proof/untracked.txt", ReadOnly: true,
			})
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(redis); err != nil {
			t.Errorf("terminate Redis: %v", err)
		}
	})
	exitCode, mounted, err := redis.Exec(ctx, []string{"cat", "/rtest-proof/untracked.txt"}, tcexec.Multiplexed())
	if err != nil {
		t.Fatal(err)
	}
	mountedBytes, err := io.ReadAll(mounted)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 || strings.TrimSpace(string(mountedBytes)) != "untracked worktree bytes" {
		t.Fatalf("sibling bind mount: exit=%d contents=%q", exitCode, mountedBytes)
	}
	host, err := redis.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := redis.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, port.Port()), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := fmt.Fprint(connection, "*1\r\n$4\r\nPING\r\n"); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "+PONG\r\n" {
		t.Fatalf("Redis PING response = %q", line)
	}
	t.Logf("REMOTE_E2E_PROOF job=%s redis=%s response=PONG bind_mount=ok", os.Getenv("RTEST_JOB_ID"), redisImage)
}
