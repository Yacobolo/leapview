package client

import (
	"testing"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestExitCodeDoesNotTreatFailedZeroAsSuccess(t *testing.T) {
	zero := 0
	if got := ExitCode(protocol.Job{Status: protocol.StatusFailed, ExitCode: &zero}); got != 1 {
		t.Fatalf("ExitCode() = %d, want 1", got)
	}
}
