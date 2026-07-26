package module

import (
	workloadobservability "github.com/Yacobolo/leapview/internal/workload/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func NewTelemetryObserver(registerer prometheus.Registerer) Observer {
	return workloadobservability.New(registerer)
}
