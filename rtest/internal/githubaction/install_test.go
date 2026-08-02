package githubaction_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func installerCommand(environment map[string]string) *exec.Cmd {
	path := filepath.Join("..", "..", "action", "setup-rtest", "install-release.sh")
	command := exec.Command("bash", path)
	command.Env = os.Environ()
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	return command
}

func runInstaller(t *testing.T, environment map[string]string) string {
	t.Helper()
	output, err := installerCommand(environment).CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, output)
	}
	return string(output)
}

func run(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(command.Args, " "), err, output)
	}
	return string(output)
}

func installerOS(value string) string {
	if value == "darwin" {
		return "darwin"
	}
	return "linux"
}

func installerArch(value string) string {
	if value == "arm64" {
		return "arm64"
	}
	return "amd64"
}
