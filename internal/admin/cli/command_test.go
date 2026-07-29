package cli

import (
	"context"
	"io"
	"strings"
	"testing"
)

type fakeOperations struct {
	called  string
	options Options
}

func (operations *fakeOperations) Initialize(context.Context, string, io.Writer) error {
	operations.called = "initialize"
	return nil
}
func (operations *fakeOperations) AcknowledgeInitialCredentials(context.Context) error {
	operations.called = "acknowledge"
	return nil
}
func (operations *fakeOperations) StorageCleanup(_ context.Context, options Options, _ io.Writer) error {
	operations.called, operations.options = "cleanup", options
	return nil
}
func (operations *fakeOperations) Maintenance(_ context.Context, options Options, _ io.Writer) error {
	operations.called, operations.options = "maintenance", options
	return nil
}
func (operations *fakeOperations) Backup(_ context.Context, options Options, _ io.Writer) error {
	operations.called, operations.options = "backup", options
	return nil
}
func (operations *fakeOperations) Restore(_ context.Context, options Options, _ io.Reader, _ io.Writer) error {
	operations.called, operations.options = "restore", options
	return nil
}

func TestCommandOwnsMaintenanceFlags(t *testing.T) {
	operations := &fakeOperations{}
	command := Command(context.Background(), operations)
	command.SetArgs([]string{"maintenance", "--apply", "--audit-days", "10", "--query-days", "11", "--archived-agent-days", "12", "--auth-state-days", "13"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if operations.called != "maintenance" {
		t.Fatalf("called = %q", operations.called)
	}
	if !operations.options.Apply || operations.options.AuditDays != 10 || operations.options.QueryDays != 11 ||
		operations.options.ArchivedAgentDays != 12 || operations.options.AuthStateDays != 13 {
		t.Fatalf("options = %#v", operations.options)
	}
}

func TestCommandRequiresOperations(t *testing.T) {
	command := Command(context.Background(), nil)
	command.SetArgs([]string{"backup", "--out", "-"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "operations are required") {
		t.Fatalf("error = %v", err)
	}
}
