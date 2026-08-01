package buildkit

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

type Commands struct {
	Create []string
	Build  []string
	Remove []string
}

type TLS struct {
	CA          string
	Certificate string
	Key         string
	ServerName  string
}

func Plan(address, name string, arguments []string) Commands {
	return PlanWithTLS(address, name, arguments, TLS{})
}

func PlanWithTLS(address, name string, arguments []string, credentials TLS) Commands {
	create := []string{"buildx", "create", "--name", name, "--driver", "remote"}
	if credentials.CA != "" || credentials.Certificate != "" || credentials.Key != "" || credentials.ServerName != "" {
		create = append(create, "--driver-opt", "cacert="+credentials.CA+",cert="+credentials.Certificate+",key="+credentials.Key+",servername="+credentials.ServerName)
	}
	create = append(create, address)
	return Commands{
		Create: create,
		Build:  append([]string{"buildx", "build", "--builder", name}, arguments...),
		Remove: []string{"buildx", "rm", "--force", name},
	}
}

func Run(ctx context.Context, docker, address, name, directory string, arguments []string, stdout, stderr io.Writer) (int, error) {
	return RunWithTLS(ctx, docker, address, name, directory, arguments, TLS{}, stdout, stderr)
}

func RunWithTLS(ctx context.Context, docker, address, name, directory string, arguments []string, credentials TLS, stdout, stderr io.Writer) (int, error) {
	if docker == "" {
		docker = "docker"
	}
	commands := PlanWithTLS(address, name, arguments, credentials)
	create := exec.CommandContext(ctx, docker, commands.Create...)
	create.Dir, create.Stdout, create.Stderr = directory, io.Discard, stderr
	if err := create.Run(); err != nil {
		return 1, err
	}
	defer func() {
		remove := exec.Command(docker, commands.Remove...)
		remove.Dir, remove.Stdout, remove.Stderr = directory, io.Discard, io.Discard
		_ = remove.Run()
	}()
	build := exec.CommandContext(ctx, docker, commands.Build...)
	build.Dir, build.Stdout, build.Stderr = directory, stdout, stderr
	if err := build.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
