package module

import (
	"context"
	"testing"

	"github.com/Yacobolo/leapview/internal/workload"
)

func TestBuildOwnsAdmissionLifecycle(t *testing.T) {
	module, err := Build(t.Context(), Config{Policy: workload.DefaultConfig()})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := module.Acquire(t.Context(), workload.Request{Class: workload.Control, Operation: "test"})
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	module.Close()
	module.Close()
	if _, err := module.Acquire(context.Background(), workload.Request{Class: workload.Control, Operation: "after-close"}); err == nil {
		t.Fatal("closed admission module accepted work")
	}
}

func TestBuildRejectsInvalidPolicy(t *testing.T) {
	policy := workload.DefaultConfig()
	policy.MaxRunning = 0
	if _, err := Build(t.Context(), Config{Policy: policy}); err == nil {
		t.Fatal("invalid workload policy was accepted")
	}
}
