package module

import (
	"github.com/Yacobolo/leapview/internal/observability"
	"github.com/Yacobolo/leapview/internal/workload"
)

// TelemetryObserver translates workload-owned events into the neutral process
// telemetry vocabulary. Keeping this adapter with workload avoids making the
// observability package depend on a product capability.
type TelemetryObserver struct {
	telemetry *observability.Telemetry
}

func NewTelemetryObserver(telemetry *observability.Telemetry) *TelemetryObserver {
	return &TelemetryObserver{telemetry: telemetry}
}

func (o *TelemetryObserver) ObserveWorkload(stats workload.Stats) {
	if o == nil || o.telemetry == nil {
		return
	}
	workspaces := make([]observability.WorkloadWorkspace, 0)
	borrowed := make(map[string]int, len(stats.Classes))
	for class, classStats := range stats.Classes {
		className := string(class)
		borrowed[className] = classStats.Borrowed
		for workspace, workspaceStats := range classStats.Workspaces {
			workspaces = append(workspaces, observability.WorkloadWorkspace{
				Class:     className,
				Workspace: workspace,
				Running:   workspaceStats.Running,
				Queued:    workspaceStats.Queued,
			})
		}
	}
	o.telemetry.ObserveWorkload(workspaces, borrowed)
}

func (o *TelemetryObserver) ObserveAdmission(event workload.AdmissionEvent) {
	if o == nil || o.telemetry == nil {
		return
	}
	o.telemetry.ObserveAdmission(
		string(event.Class),
		event.Outcome,
		string(event.Reason),
		event.QueueWait,
		event.Execution,
	)
}
