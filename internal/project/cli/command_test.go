package cli

import (
	"context"
	"strings"
	"testing"
)

func TestValidateCommandOwnsProjectArgumentRules(t *testing.T) {
	command := ValidateCommand(context.Background())
	command.SetArgs([]string{"project.yaml", "--project", "other.yaml"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "choose either --project or positional project") {
		t.Fatalf("error = %v", err)
	}
}

func TestSchemaCommandRejectsUnknownFormats(t *testing.T) {
	command := SchemaCommand()
	command.SetArgs([]string{"export", "--format", "yaml"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), `unsupported schema format "yaml"`) {
		t.Fatalf("error = %v", err)
	}
}
