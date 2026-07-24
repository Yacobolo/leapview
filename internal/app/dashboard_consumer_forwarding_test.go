package app

import (
	"context"
	"testing"

	"github.com/Yacobolo/leapview/internal/analytics/dataquery"
	"github.com/Yacobolo/leapview/internal/dashboard/command"
	"github.com/Yacobolo/leapview/internal/dashboard/consumer"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	queryauthz "github.com/Yacobolo/leapview/internal/dashboard/queryauthz"
	dashboardstream "github.com/Yacobolo/leapview/internal/dashboard/stream"
	visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"
	"github.com/Yacobolo/leapview/internal/workload"
)

type consumerForwardingMetrics struct {
	fakeMetrics
	calls    int
	governed bool
	admitter bool
}

func (m *consumerForwardingMetrics) ExecuteConsumersPage(ctx context.Context, request consumer.Request, publish consumer.Publisher) error {
	m.calls++
	_, m.governed = dataquery.GovernorFromContext(ctx)
	_, m.admitter = workload.FromContext(ctx)
	dataquery.ObservePhysicalQuery(ctx, dataquery.PhysicalQueryObservation{Count: 1})
	for _, target := range request.Targets {
		publish(consumer.Result{Target: target, Envelope: visualizationir.VisualizationEnvelope{VisualID: target.ID}, Queries: 1})
	}
	return nil
}

func TestProductionDashboardWrappersForwardGovernedConsumerPlan(t *testing.T) {
	underlying := &consumerForwardingMetrics{}
	controller, err := workload.New(workload.DefaultConfig())
	if err != nil {
		t.Fatalf("new workload controller: %v", err)
	}
	t.Cleanup(controller.Close)
	metrics := dashboardmodule.WithQueryAudit(
		dashboardmodule.WithAdmission(queryauthz.New(underlying, queryauthz.Options{}), controller, ""),
		nil, "", nil,
	)

	visuals := 0
	dashboardstream.TargetWork(metrics, dashboardstream.WorkRequest{
		DashboardID: "sales-dashboard",
		PageID:      "overview",
		Plan: command.RefreshPlan{Targets: []command.Target{
			{Kind: command.TargetVisual, ID: "orders"},
			{Kind: command.TargetVisual, ID: "revenue"},
		}},
	})(context.Background(), func(event dashboardstream.RefreshEvent) bool {
		if event.Type == dashboardstream.RefreshEventVisual {
			visuals++
		}
		return true
	})

	if underlying.calls != 1 || !underlying.governed || !underlying.admitter || visuals != 2 {
		t.Fatalf("calls=%d governed=%v admitter=%v visuals=%d", underlying.calls, underlying.governed, underlying.admitter, visuals)
	}
}
