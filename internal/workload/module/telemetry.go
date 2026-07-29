package module

import (
	workloadobservability "github.com/flidai/leapview/internal/workload/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func NewTelemetryObserver(registerer prometheus.Registerer) Observer {
	return workloadobservability.New(registerer)
}
