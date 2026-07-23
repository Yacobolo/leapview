package module

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Yacobolo/leapview/internal/access"
	dashboardhttp "github.com/Yacobolo/leapview/internal/dashboard/http"
	"github.com/Yacobolo/leapview/internal/dashboard/publication"
	publicationsqlite "github.com/Yacobolo/leapview/internal/dashboard/publication/sqlite"
	semanticapi "github.com/Yacobolo/leapview/internal/dashboard/semanticapi"
	dashboardstream "github.com/Yacobolo/leapview/internal/dashboard/stream"
	dashboardui "github.com/Yacobolo/leapview/internal/dashboard/ui"
	"github.com/Yacobolo/leapview/internal/queryruntime"
	"github.com/Yacobolo/leapview/internal/ui"
	visualizationir "github.com/Yacobolo/leapview/internal/visualization/ir"
	"github.com/Yacobolo/leapview/internal/workload"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type Module struct {
	handler            dashboardhttp.Handler
	semantic           semanticapi.Handler
	snapshot           func(context.Context, string) (string, error)
	publications       *publicationsqlite.Repository
	publicationService *publication.Service
	publicURL          string
	currentActor       func(*http.Request) string
	streams            publication.StreamRegistry
	publicBroker       dashboardhttp.SignalBroker
	publicTelemetry    PublicTelemetry
	logger             *slog.Logger
	runtimeMetrics     queryruntime.Metrics
	defaultWorkspaceID string
	coordinators       *dashboardstream.Registry
	lifecycleMu        sync.Mutex
	lifecycleCancel    context.CancelFunc
	lifecycleWG        sync.WaitGroup
}

type Config struct {
	Database           *sql.DB
	HTTP               HTTPConfig
	Semantic           SemanticConfig
	ServingSnapshot    func(context.Context, string) (string, error)
	PublicTelemetry    PublicTelemetry
	Logger             *slog.Logger
	Trace              *pagestream.TraceStore
	PublicURL          string
	CurrentActor       func(*http.Request) string
	RuntimeMetrics     queryruntime.Metrics
	DefaultWorkspaceID string
}

type HTTPConfig struct {
	Metrics             queryruntime.Metrics
	MetricsForWorkspace func(string) (queryruntime.Metrics, bool)
	Admission           workload.Admitter
	Broker              SignalBroker
	Logger              *slog.Logger
	Telemetry           DashboardTelemetry
	CurrentPrincipalID  func(*http.Request) string
	AuthorizeListObject func(context.Context, string, access.ObjectRef) (bool, error)
	CSRFToken           func(*http.Request) string
	ChatChromeSignal    func(*http.Request) ui.ChatSignal
	Environment         func(*http.Request) string
	DataRefreshedAt     func(context.Context, string, string, string) string
	AgentBootstrap      func(*http.Request, string) ui.ChatViewState
}

type SemanticConfig struct {
	Metrics             queryruntime.Metrics
	MetricsForWorkspace func(string) (queryruntime.Metrics, bool)
	CurrentPrincipalID  func(*http.Request) string
	AuthorizeListObject func(context.Context, string, access.ObjectRef) (bool, error)
}

type SignalBroker interface {
	Subscribe(string) (<-chan pagestream.SignalPatch, func())
	PublishEnvelope(string, pagestream.Envelope)
	TraceStore() *pagestream.TraceStore
}

type DashboardTelemetry interface {
	DashboardRefreshStarted(string)
	DashboardRefreshFinished(string, string, int, map[string]float64)
	DashboardRefreshEventObserved(string, string)
	VisualizationFrameObserved(kind string, rows, cardinality, encodedBytes int)
	DashboardCacheObserved(string)
}

func Build(_ context.Context, config Config) (*Module, error) {
	coordinators := dashboardstream.NewRegistry()
	metricsForHTTP := func(workspaceID string) (dashboardhttp.Metrics, bool) {
		if config.HTTP.MetricsForWorkspace == nil {
			return nil, false
		}
		metrics, ok := config.HTTP.MetricsForWorkspace(workspaceID)
		return metrics, ok
	}
	metricsForSemantic := func(workspaceID string) (semanticapi.Metrics, bool) {
		if config.Semantic.MetricsForWorkspace == nil {
			return nil, false
		}
		metrics, ok := config.Semantic.MetricsForWorkspace(workspaceID)
		return metrics, ok
	}
	chromeDecorators := func(r *http.Request) []dashboardui.ChromeDecorator {
		if config.HTTP.ChatChromeSignal == nil {
			return nil
		}
		return ChatChromeDecorators(config.HTTP.ChatChromeSignal(r))
	}
	telemetry := config.HTTP.Telemetry
	handler := dashboardhttp.Handler{
		Metrics: config.HTTP.Metrics, MetricsForWorkspace: metricsForHTTP,
		AnalyticalContext: func(ctx context.Context) context.Context {
			return workload.WithAdmitter(ctx, config.HTTP.Admission)
		},
		Broker: config.HTTP.Broker, Coordinators: coordinators, Logger: config.HTTP.Logger,
		RefreshStarted: func(refresh dashboardstream.Refresh) {
			if telemetry != nil {
				telemetry.DashboardRefreshStarted(refresh.Command)
			}
		},
		RefreshFinished: func(summary dashboardstream.RefreshSummary) {
			if telemetry != nil {
				telemetry.DashboardRefreshFinished(summary.Command, summary.Outcome, summary.CancellationCount, summary.StageTimingsMs)
			}
		},
		RefreshEventObserved: func(event dashboardstream.RefreshEvent) {
			if telemetry != nil {
				telemetry.DashboardRefreshEventObserved(string(event.Type), event.Target)
				observeVisualizationFrame(telemetry, event)
			}
		},
		CacheObserved: func(outcome string) {
			if telemetry != nil {
				telemetry.DashboardCacheObserved(outcome)
			}
		},
		CurrentPrincipalID: config.HTTP.CurrentPrincipalID, AuthorizeListObject: config.HTTP.AuthorizeListObject,
		CSRFToken: config.HTTP.CSRFToken, ChromeDecorators: chromeDecorators,
		Environment: config.HTTP.Environment, DataRefreshedAt: config.HTTP.DataRefreshedAt,
		AgentBootstrap: config.HTTP.AgentBootstrap,
	}
	module := &Module{
		handler: handler,
		semantic: semanticapi.Handler{
			Metrics: config.Semantic.Metrics, MetricsForWorkspace: metricsForSemantic,
			CurrentPrincipalID:  config.Semantic.CurrentPrincipalID,
			AuthorizeListObject: config.Semantic.AuthorizeListObject,
		},
		snapshot:  config.ServingSnapshot,
		publicURL: config.PublicURL, currentActor: config.CurrentActor,
		streams: publication.NewMemoryStreamRegistry(), publicBroker: config.HTTP.Broker,
		publicTelemetry: config.PublicTelemetry, logger: config.Logger,
		runtimeMetrics: config.RuntimeMetrics, defaultWorkspaceID: config.DefaultWorkspaceID,
		coordinators: coordinators,
	}
	if config.Database != nil {
		module.publications = publicationsqlite.NewRepository(config.Database)
		module.streams = publicationsqlite.NewStreamRegistry(config.Database)
		module.publicBroker = publicationsqlite.NewBroker(config.Database, config.Trace, config.Logger)
		module.publicationService = publication.NewService(module.publications, module.streams.ClosePublication)
	}
	return module, nil
}

func observeVisualizationFrame(telemetry DashboardTelemetry, event dashboardstream.RefreshEvent) {
	if event.Type != dashboardstream.RefreshEventVisual && event.Type != dashboardstream.RefreshEventVisualMetadata {
		return
	}
	envelope, ok := event.Value.(visualizationir.VisualizationEnvelope)
	if !ok {
		return
	}
	rows, cardinality, kind := 0, 0, ""
	switch state := envelope.DataState.Value.(type) {
	case *visualizationir.InlineVisualizationDataState:
		kind = "inline"
		for _, dataset := range state.Datasets {
			rows += len(dataset.Rows)
		}
		cardinality = rows
	case *visualizationir.WindowedVisualizationDataState:
		kind = "windowed"
		for _, block := range state.Blocks {
			rows += len(block.Rows)
		}
		cardinality = int(state.AvailableRows)
		if state.Cardinality.Count != nil {
			cardinality = int(*state.Cardinality.Count)
		}
	case *visualizationir.SpatialWindowedVisualizationDataState:
		kind = "spatial_windowed"
		if state.Window != nil {
			rows = len(state.Window.Rows)
		}
		cardinality = rows
		if state.Cardinality.Count != nil {
			cardinality = int(*state.Cardinality.Count)
		}
	default:
		return
	}
	encoded, _ := json.Marshal(envelope)
	telemetry.VisualizationFrameObserved(kind, rows, cardinality, len(encoded))
}

func (m *Module) HTTP() dashboardhttp.Handler      { return m.handler }
func (m *Module) SemanticAPI() semanticapi.Handler { return m.semantic }
