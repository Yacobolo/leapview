package swarmscheduler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/rtest/internal/swarm"
)

func TestCompletedLogsPreferDurableJobFile(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "job-one")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "job.log"), []byte("durable output\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scheduler := New(Config{Client: swarm.New(swarm.Config{Binary: "does-not-exist"}), JobsRoot: root})
	var output bytes.Buffer
	if err := scheduler.Logs(context.Background(), "job-one", false, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "durable output\n" {
		t.Fatalf("output = %q", output.String())
	}
}
