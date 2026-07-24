package module

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/agent"
	agentapi "github.com/Yacobolo/leapview/internal/agent/api"
	agenthttp "github.com/Yacobolo/leapview/internal/agent/http"
	agentopenai "github.com/Yacobolo/leapview/internal/agent/openai"
	agenttools "github.com/Yacobolo/leapview/internal/agent/tools"
	"github.com/Yacobolo/leapview/internal/dashboard/queryruntime"
	productsearch "github.com/Yacobolo/leapview/internal/workspace/search"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
	agentcore "github.com/Yacobolo/leapview/pkg/agent"
	"github.com/Yacobolo/leapview/pkg/pagestream"
	"github.com/Yacobolo/toolbelt/apigen/runtime/agenttool"
)

type Module struct {
	handler                  *agenthttp.Handler
	service                  *agent.Service
	jobs                     JobStore
	defaultWorkspaceID       string
	runWorkloadClass         string
	globalWorkspaceID        string
	search                   SearchPort
	environment              func(*http.Request) string
	dashboardMetrics         func(string) (queryruntime.Metrics, bool)
	authorizeAnyObject       func(context.Context, string, access.Privilege, []access.ObjectRef) (bool, error)
	skipContextAuthorization bool
	recordAudit              func(context.Context, access.AuditEventInput) error
	dispatchAPIGen           func(agent.Scope, string, http.ResponseWriter, *http.Request) bool
	enableSystemPrompt       bool
	broker                   *pagestream.Broker
	logger                   *slog.Logger
	chatTitleMu              sync.Mutex
	pendingChatTitles        map[string]struct{}
	mcpScope                 func(*http.Request) (agent.Scope, bool)
	mcpProtect               func(http.Handler) http.Handler
	productName              string
	apiOperations            []agenttools.APIGenOperation
}

type Service = agent.Service
type AdminAgentResponse = agentapi.AdminAgentResponse
type APIGenOperation = agenttools.APIGenOperation
type APIGenOperationContract = agenttools.OperationContract

func BuildAPIGenOperations(operationContracts map[string]APIGenOperationContract, toolContracts map[string]agenttool.Contract) []APIGenOperation {
	return agenttools.BuildAPIGenOperations(operationContracts, toolContracts)
}

type Config struct {
	Database                 *sql.DB
	Model                    ModelConfig
	Service                  *agent.Service
	Jobs                     JobStore
	DefaultWorkspaceID       string
	RunWorkloadClass         string
	GlobalWorkspaceID        string
	Search                   SearchPort
	Environment              func(*http.Request) string
	DashboardMetrics         func(string) (queryruntime.Metrics, bool)
	AuthorizeAnyObject       func(context.Context, string, access.Privilege, []access.ObjectRef) (bool, error)
	SkipContextAuthorization bool
	RecordAudit              func(context.Context, access.AuditEventInput) error
	DispatchAPIGen           func(Scope, string, http.ResponseWriter, *http.Request) bool
	EnableSystemPrompt       bool
	Logger                   *slog.Logger
	MCPScope                 func(*http.Request) (Scope, bool)
	MCPProtect               func(http.Handler) http.Handler
	ProductName              string
	APIGenOperations         []agenttools.APIGenOperation
	HTTP                     HTTPConfig
}

type SearchPort interface {
	SearchSubject(*http.Request) (productsearch.Subject, bool)
	Search(context.Context, productsearch.Subject, productsearch.Query) (productsearch.Page, error)
	ResolveSearchReferences(context.Context, productsearch.Subject, string, []productsearch.Reference) ([]productsearch.Result, error)
}

type Principal struct {
	ID            string
	DevAuthBypass bool
}

type ModelConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type Scope struct {
	WorkspaceID   string
	PrincipalID   string
	Credential    CredentialScope
	DevAuthBypass bool
}

type CredentialScope struct {
	WorkspaceID string
	Privileges  []string
	Restricted  bool
}

type Settings interface {
	GetSetting(context.Context, string) (string, error)
	UpsertSetting(context.Context, string, string) error
}

type HTTPConfig struct {
	Settings           Settings
	CurrentPrincipal   func(*http.Request) (Principal, bool)
	CurrentCredential  func(*http.Request) (access.APICredential, bool)
	Broker             *pagestream.Broker
	CSRFToken          func(*http.Request) string
	CurrentRoleLabel   func(*http.Request) string
	SearchReferences   func(*http.Request, agent.TurnContext, string, int) ([]ui.AgentReferenceSignal, error)
	ResolveTurnContext func(*http.Request, agent.Scope, agent.TurnContext) (agent.TurnContext, error)
}

func Build(_ context.Context, config Config) (*Module, error) {
	if config.RunWorkloadClass == "" {
		config.RunWorkloadClass = "background"
	}
	if config.GlobalWorkspaceID == "" {
		config.GlobalWorkspaceID = "_global"
	}
	service := config.Service
	if service == nil && config.Database != nil {
		service = agent.NewService(newRepository(config.Database), agent.Config{
			APIKey: config.Model.APIKey, BaseURL: config.Model.BaseURL, Model: config.Model.Model,
		})
	}
	if service != nil {
		service.ConfigureDefaultModel(func(modelConfig agent.Config) agentcore.Model {
			return agentopenai.NewModel(modelConfig, nil)
		})
	}
	var dispatchAPIGen func(agent.Scope, string, http.ResponseWriter, *http.Request) bool
	if config.DispatchAPIGen != nil {
		dispatchAPIGen = func(scope agent.Scope, operationID string, writer http.ResponseWriter, request *http.Request) bool {
			return config.DispatchAPIGen(scopeFromAgent(scope), operationID, writer, request)
		}
	}
	var mcpScope func(*http.Request) (agent.Scope, bool)
	if config.MCPScope != nil {
		mcpScope = func(r *http.Request) (agent.Scope, bool) {
			scope, ok := config.MCPScope(r)
			return scopeToAgent(scope), ok
		}
	}
	m := &Module{
		service: service, jobs: config.Jobs,
		defaultWorkspaceID: config.DefaultWorkspaceID, runWorkloadClass: config.RunWorkloadClass,
		globalWorkspaceID: config.GlobalWorkspaceID, search: config.Search, environment: config.Environment,
		dashboardMetrics: config.DashboardMetrics, authorizeAnyObject: config.AuthorizeAnyObject,
		skipContextAuthorization: config.SkipContextAuthorization,
		recordAudit:              config.RecordAudit, dispatchAPIGen: dispatchAPIGen,
		enableSystemPrompt: config.EnableSystemPrompt, broker: config.HTTP.Broker, logger: config.Logger,
		pendingChatTitles: map[string]struct{}{},
		mcpScope:          mcpScope, mcpProtect: config.MCPProtect,
		productName:   config.ProductName,
		apiOperations: append([]agenttools.APIGenOperation(nil), config.APIGenOperations...),
	}
	searchReferences := config.HTTP.SearchReferences
	if searchReferences == nil {
		searchReferences = m.SearchReferences
	}
	resolveTurnContext := config.HTTP.ResolveTurnContext
	if resolveTurnContext == nil {
		resolveTurnContext = m.ResolveTurnContext
	}
	currentPrincipal := func(r *http.Request) (agenthttp.Principal, bool) {
		if config.HTTP.CurrentPrincipal == nil {
			return agenthttp.Principal{}, false
		}
		principal, ok := config.HTTP.CurrentPrincipal(r)
		return agenthttp.Principal{ID: principal.ID, DevAuthBypass: principal.DevAuthBypass}, ok
	}
	m.handler = agenthttp.NewHandler(agenthttp.Options{
		Service: service, Settings: config.HTTP.Settings,
		CurrentPrincipal: currentPrincipal, CurrentCredential: config.HTTP.CurrentCredential,
		Broker: config.HTTP.Broker, CSRFToken: config.HTTP.CSRFToken,
		CurrentRoleLabel: config.HTTP.CurrentRoleLabel, ChatSignal: m.chatSignal,
		ChatSignalWith: m.ChatSignalWith, SearchReferences: searchReferences,
		ResolveTurnContext: resolveTurnContext, QueueMissingTitle: m.queueMissingChatTitle,
		ExecuteStartedChatTurn: m.executeStartedChatTurn,
		EnqueueRun:             m.EnqueueRun, CancelQueuedRun: m.CancelQueuedRun,
		APIGenToolContracts: apiGenToolContracts(m.apiOperations),
	})
	m.configureTools()
	return m, nil
}

func scopeFromAgent(scope agent.Scope) Scope {
	return Scope{
		WorkspaceID: scope.WorkspaceID, PrincipalID: scope.PrincipalID,
		DevAuthBypass: scope.DevAuthBypass,
		Credential: CredentialScope{
			WorkspaceID: scope.Credential.WorkspaceID,
			Privileges:  append([]string(nil), scope.Credential.Privileges...),
			Restricted:  scope.Credential.Restricted,
		},
	}
}

func scopeToAgent(scope Scope) agent.Scope {
	return agent.Scope{
		WorkspaceID: scope.WorkspaceID, PrincipalID: scope.PrincipalID,
		DevAuthBypass: scope.DevAuthBypass,
		Credential: agent.CredentialScope{
			WorkspaceID: scope.Credential.WorkspaceID,
			Privileges:  append([]string(nil), scope.Credential.Privileges...),
			Restricted:  scope.Credential.Restricted,
		},
	}
}

func (m *Module) HTTP() *agenthttp.Handler { return m.handler }

func (m *Module) UpdateConversation(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdateConversation(w, r)
}
