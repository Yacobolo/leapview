package module

import (
	"net/http"
	"testing"

	"github.com/flidai/leapview/internal/access"
	apiaggregate "github.com/flidai/leapview/internal/app/api/aggregate"
)

func testAPIGenAuthorizer(t *testing.T) *APIGenAuthorizer {
	t.Helper()
	resolver := func(*http.Request, string) []ObjectRef { return nil }
	authorizer, err := (&Module{}).APIGenAuthorizer(testAPIGenContracts(), APIGenObjectResolvers{
		Dashboard: resolver, SemanticModel: resolver, WorkspaceAsset: resolver,
	})
	if err != nil {
		t.Fatal(err)
	}
	return authorizer
}

func testAPIGenContracts() map[string]APIGenOperationContract {
	generated := apiaggregate.GetAPIGenOperationContracts()
	contracts := make(map[string]APIGenOperationContract, len(generated))
	for operationID, contract := range generated {
		contracts[operationID] = APIGenOperationContract{
			OperationID: contract.OperationID, Path: contract.Path, Protected: contract.Protected,
			AuthzMode: contract.AuthzMode, Extensions: contract.Extensions,
		}
	}
	return contracts
}

func TestAPIGenAuthorizationContractCoverage(t *testing.T) {
	authorizer := testAPIGenAuthorizer(t)
	contracts := authorizer.operations
	if len(contracts) == 0 {
		t.Fatal("no generated operation contracts")
	}
	publicAuthoringAuth := map[string]bool{
		"beginDeviceAuthorization":    true,
		"exchangeDeviceAuthorization": true,
		"refreshAuthoringToken":       true,
		"revokeAuthoringToken":        true,
		"exchangeWorkloadIdentity":    true,
		"getInstance":                 true,
	}
	for operationID, contract := range contracts {
		if publicAuthoringAuth[operationID] {
			if contract.Protected || contract.AuthzMode != "none" {
				t.Fatalf("%s authorization = protected:%t mode:%q, want public credential exchange", operationID, contract.Protected, contract.AuthzMode)
			}
			continue
		}
		if !contract.Protected {
			t.Fatalf("%s auth contract is not protected", operationID)
		}
		_, ok := apiGenOperationPrivilege(contract)
		if !ok {
			t.Fatalf("%s has invalid authorization metadata", operationID)
		}
		if operationID == "decideDeviceAuthorization" {
			if contract.AuthzMode != "authenticated" {
				t.Fatalf("%s auth mode = %q, want authenticated", operationID, contract.AuthzMode)
			}
			continue
		}
		if contract.AuthzMode != "privilege" {
			t.Fatalf("%s auth mode = %q, want privilege", operationID, contract.AuthzMode)
		}
		if isGlobalAgentOperation(operationID) {
			if _, hasScope := contract.Extensions[apiGenObjectScopeExtension]; hasScope {
				t.Fatalf("%s global operation retains object-scope metadata", operationID)
			}
			continue
		}
		if _, ok := authorizer.objectResolverForContract(contract); !ok {
			t.Fatalf("%s has invalid object scope for %q", operationID, contract.Path)
		}
	}
	contract, ok := contracts["uploadReleaseArtifact"]
	if !ok {
		t.Fatal("uploadReleaseArtifact contract is missing")
	}
	if got, _ := apiGenOperationPrivilege(contract); got != access.PrivilegePublishRelease {
		t.Fatalf("uploadReleaseArtifact privilege = %q, want %q", got, access.PrivilegePublishRelease)
	}
}

func TestDeviceAuthorizationApprovalRequiresCSRF(t *testing.T) {
	for operationID := range testAPIGenContracts() {
		got := apiGenRequiresCSRF(operationID)
		if got != (operationID == "decideDeviceAuthorization") {
			t.Errorf("apiGenRequiresCSRF(%q) = %t", operationID, got)
		}
	}
}

func TestAPIGenObjectResolverRejectsInvalidContracts(t *testing.T) {
	authorizer := testAPIGenAuthorizer(t)
	tests := []struct {
		name         string
		contract     APIGenOperationContract
		wantOK       bool
		wantResolver bool
	}{
		{name: "workspace scoped", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards", Extensions: map[string]any{}}, wantOK: true},
		{name: "supported exact scope", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{apiGenObjectScopeExtension: "dashboard"}}, wantOK: true, wantResolver: true},
		{name: "missing exact scope", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{}}},
		{name: "wrong exact scope", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{apiGenObjectScopeExtension: "semantic-model"}}},
		{name: "unknown exact scope", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{apiGenObjectScopeExtension: "tenant"}}},
		{
			name: "malformed exact scope",
			contract: APIGenOperationContract{
				Path:       "/api/v1/workspaces/{workspace}/dashboards/{dashboard}",
				Extensions: map[string]any{apiGenObjectScopeExtension: map[string]any{"kind": "dashboard"}},
			},
		},
		{name: "unexpected exact scope", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards", Extensions: map[string]any{apiGenObjectScopeExtension: "dashboard"}}},
		{name: "ambiguous exact scope", contract: APIGenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}/semantic-models/{model}", Extensions: map[string]any{apiGenObjectScopeExtension: "dashboard"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver, ok := authorizer.objectResolverForContract(test.contract)
			if ok != test.wantOK {
				t.Fatalf("ok = %t, want %t", ok, test.wantOK)
			}
			if got := resolver != nil; got != test.wantResolver {
				t.Fatalf("has resolver = %t, want %t", got, test.wantResolver)
			}
		})
	}
}

func TestManagedDataAndDeploymentAPIGenPrivilegesArePlatformGlobal(t *testing.T) {
	authorizer := testAPIGenAuthorizer(t)
	want := map[string]access.Privilege{
		"getActiveManagedDataRevision":          access.PrivilegeViewData,
		"listManagedConnections":                access.PrivilegeViewData,
		"getManagedConnection":                  access.PrivilegeViewData,
		"listManagedDataRevisions":              access.PrivilegeViewData,
		"getManagedDataRevision":                access.PrivilegeViewData,
		"createManagedDataUploadSession":        access.PrivilegeIngestData,
		"getManagedDataUploadSession":           access.PrivilegeIngestData,
		"listManagedDataUploadSessions":         access.PrivilegeIngestData,
		"cancelManagedDataUploadSession":        access.PrivilegeIngestData,
		"listManagedDataUploadSessionEvents":    access.PrivilegeIngestData,
		"finalizeManagedDataUploadSession":      access.PrivilegeIngestData,
		"createManagedDataS3MultipartUpload":    access.PrivilegeIngestData,
		"signManagedDataS3MultipartPart":        access.PrivilegeIngestData,
		"completeManagedDataS3MultipartUpload":  access.PrivilegeIngestData,
		"abortManagedDataS3MultipartUpload":     access.PrivilegeIngestData,
		"startProjectCandidate":                 access.PrivilegeAuthorProject,
		"getProjectCandidate":                   access.PrivilegeAuthorProject,
		"reviewProjectCandidate":                access.PrivilegeReviewCandidate,
		"replaceProjectCandidateArtifact":       access.PrivilegeAuthorProject,
		"retryProjectCandidate":                 access.PrivilegeAuthorProject,
		"cancelProjectCandidate":                access.PrivilegeAuthorProject,
		"planProjectCandidateSynchronization":   access.PrivilegeAuthorProject,
		"uploadProjectCandidateSourceBlob":      access.PrivilegeAuthorProject,
		"commitProjectCandidateSynchronization": access.PrivilegeAuthorProject,
		"createDeployment":                      access.PrivilegeRequestDeployment,
		"getDeployment":                         access.PrivilegeViewItem,
		"listDeployments":                       access.PrivilegeViewItem,
		"cancelDeployment":                      access.PrivilegeRequestDeployment,
		"rollbackDeployment":                    access.PrivilegeRollbackDeployment,
		"requestDeploymentApproval":             access.PrivilegeRequestDeployment,
		"approveDeployment":                     access.PrivilegeApproveDeployment,
		"revokeDeploymentApproval":              access.PrivilegeApproveDeployment,
		"activateDeployment":                    access.PrivilegeActivateDeployment,
	}
	for operationID, expected := range want {
		contract, ok := authorizer.operations[operationID]
		if !ok {
			t.Errorf("%s contract is missing", operationID)
			continue
		}
		if got, ok := apiGenOperationPrivilege(contract); !ok || got != expected {
			t.Errorf("%s privilege = %q, want %q", operationID, got, expected)
		}
		if resolver, ok := authorizer.objectResolverForContract(contract); !ok || resolver != nil {
			t.Errorf("%s must remain workspace-scoped without an exact-object resolver", operationID)
		}
	}
}
