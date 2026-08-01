package buildkit_test

import (
	"reflect"
	"testing"

	"github.com/flidai/leapview/rtest/internal/buildkit"
)

func TestPlanUsesEphemeralNativeBuildxRemoteBuilder(t *testing.T) {
	got := buildkit.Plan("tcp://127.0.0.1:1234", "rtest-proof", []string{"--push", "-t", "registry.invalid/proof:latest", "."})
	if !reflect.DeepEqual(got.Create, []string{"buildx", "create", "--name", "rtest-proof", "--driver", "remote", "tcp://127.0.0.1:1234"}) {
		t.Fatalf("create = %#v", got.Create)
	}
	if !reflect.DeepEqual(got.Build, []string{"buildx", "build", "--builder", "rtest-proof", "--push", "-t", "registry.invalid/proof:latest", "."}) {
		t.Fatalf("build = %#v", got.Build)
	}
	if !reflect.DeepEqual(got.Remove, []string{"buildx", "rm", "--force", "rtest-proof"}) {
		t.Fatalf("remove = %#v", got.Remove)
	}
}

func TestPlanWithTLSUsesStandardRemoteDriverOptions(t *testing.T) {
	got := buildkit.PlanWithTLS("tcp://builder.example:1234", "rtest-secure", []string{"--push", "."}, buildkit.TLS{
		CA: "/tmp/ca.pem", Certificate: "/tmp/cert.pem", Key: "/tmp/key.pem", ServerName: "builder.example",
	})
	want := []string{"buildx", "create", "--name", "rtest-secure", "--driver", "remote", "--driver-opt", "cacert=/tmp/ca.pem,cert=/tmp/cert.pem,key=/tmp/key.pem,servername=builder.example", "tcp://builder.example:1234"}
	if !reflect.DeepEqual(got.Create, want) {
		t.Fatalf("create = %#v, want %#v", got.Create, want)
	}
}
