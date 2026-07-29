package module

import (
	dashboardobservability "github.com/flidai/leapview/internal/dashboard/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func NewTelemetry(registerer prometheus.Registerer) Telemetry {
	return dashboardobservability.New(registerer)
}
