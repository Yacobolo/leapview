package reapi

import (
	"testing"

	longrunning "google.golang.org/genproto/googleapis/longrunning"
)

func TestOperationTrackerCapturesLatestExecutionName(t *testing.T) {
	tracker := &operationTracker{}
	tracker.observe(&longrunning.Operation{Name: "operations/action-1"})
	tracker.observe(&longrunning.Operation{Name: "operations/action-2"})
	if got := tracker.name(); got != "operations/action-2" {
		t.Fatalf("operation name = %q", got)
	}
}
