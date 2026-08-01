package cli

import (
	"reflect"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/config"
)

func TestParseExecUsesGenericProjectImageAndArbitraryArgv(t *testing.T) {
	settings := config.Config{Service: &config.Service{Project: "example", Image: "image@sha256:digest", CPUs: "2", Memory: "4g"}}
	got, err := parseExec(settings, []string{"--timeout", "5m", "--workdir", "service", "--env", "CI=true", "--", "task", "test", "--race"})
	if err != nil {
		t.Fatal(err)
	}
	if got.project != "example" || got.image != "image@sha256:digest" || got.timeout != 5*time.Minute || got.workdir != "service" || got.environment["CI"] != "true" {
		t.Fatalf("options = %#v", got)
	}
	if !reflect.DeepEqual(got.command, []string{"task", "test", "--race"}) {
		t.Fatalf("command = %#v", got.command)
	}
}

func TestParseExecRequiresExplicitCommandBoundary(t *testing.T) {
	settings := config.Config{Service: &config.Service{Project: "example", Image: "image", CPUs: "2", Memory: "4g"}}
	if _, err := parseExec(settings, []string{"go", "test", "./..."}); err == nil {
		t.Fatal("exec accepted a command without -- boundary")
	}
}

func TestGlobalTokenIsRemovedBeforeCommandDispatch(t *testing.T) {
	token, args, err := globalArgs([]string{"--token", "secret", "exec", "--", "true"})
	if err != nil || token != "secret" || !reflect.DeepEqual(args, []string{"exec", "--", "true"}) {
		t.Fatalf("token=%q args=%#v err=%v", token, args, err)
	}
}
