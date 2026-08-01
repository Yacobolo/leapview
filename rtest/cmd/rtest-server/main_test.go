package main

import (
	"context"
	"testing"
	"time"
)

func TestEndpointUsesThePublicServerNameAndListenerPort(t *testing.T) {
	for _, test := range []struct {
		name, listen, want string
	}{
		{"rtest.example.com", ":50052", "rtest.example.com:50052"},
		{"127.0.0.1", "127.0.0.1:1235", "127.0.0.1:1235"},
		{"2001:db8::1", "[::]:50052", "[2001:db8::1]:50052"},
	} {
		if got := endpoint(test.name, test.listen); got != test.want {
			t.Errorf("endpoint(%q, %q) = %q, want %q", test.name, test.listen, got, test.want)
		}
	}
}

func TestReconcilerRunsImmediatelyWithoutBlockingStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := &recordingReconciler{called: make(chan struct{}, 1)}
	go runReconciler(ctx, runner, time.Hour)
	select {
	case <-runner.called:
	case <-time.After(time.Second):
		t.Fatal("reconciler did not run immediately")
	}
}

type recordingReconciler struct{ called chan struct{} }

func (r *recordingReconciler) RunOnce(context.Context) error {
	r.called <- struct{}{}
	return nil
}
