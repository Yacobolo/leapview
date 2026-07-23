package module

import (
	"net/http"
	"testing"

	"github.com/Yacobolo/leapview/internal/access"
	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
)

func testAPIGenAuthorizer(t *testing.T) *APIGenAuthorizer {
	t.Helper()
	resolver := func(*http.Request, string) []ObjectRef { return nil }
	authorizer, err := (&Module{}).APIGenAuthorizer(APIGenObjectResolvers{
		Dashboard: resolver, SemanticModel: resolver, WorkspaceAsset: resolver,
	})
	if err != nil {
		t.Fatal(err)
	}
	return authorizer
}

func TestAPIGenAuthorizationContractCoverage(t *testing.T) {
	authorizer := testAPIGenAuthorizer(t)
	contracts := apigenapi.GetAPIGenOperationContracts()
	if len(contracts) == 0 {
		t.Fatal("no generated operation contracts")
	}
	for operationID, contract := range contracts {
		if !contract.Protected {
			t.Fatalf("%s auth contract is not protected", operationID)
		}
		privilege, ok := apiGenOperationPrivilege(contract)
		if !ok {
			t.Fatalf("%s has invalid authorization metadata", operationID)
		}
		if operationID == "getInstance" {
			if contract.AuthzMode != "authenticated" || privilege != "" {
				t.Fatalf("getInstance authorization = (%q, %q), want authenticated without privilege", contract.AuthzMode, privilege)
			}
		} else if contract.AuthzMode != "privilege" {
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
	contract, ok := apigenapi.GetAPIGenOperationContract("uploadReleaseArtifact")
	if !ok {
		t.Fatal("uploadReleaseArtifact contract is missing")
	}
	if got, _ := apiGenOperationPrivilege(contract); got != access.PrivilegeDeploy {
		t.Fatalf("uploadReleaseArtifact privilege = %q, want %q", got, access.PrivilegeDeploy)
	}
}

func TestAPIGenObjectResolverRejectsInvalidContracts(t *testing.T) {
	authorizer := testAPIGenAuthorizer(t)
	tests := []struct {
		name         string
		contract     apigenapi.GenOperationContract
		wantOK       bool
		wantResolver bool
	}{
		{name: "workspace scoped", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards", Extensions: map[string]any{}}, wantOK: true},
		{name: "supported exact scope", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{apiGenObjectScopeExtension: "dashboard"}}, wantOK: true, wantResolver: true},
		{name: "missing exact scope", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{}}},
		{name: "wrong exact scope", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{apiGenObjectScopeExtension: "semantic-model"}}},
		{name: "unknown exact scope", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}", Extensions: map[string]any{apiGenObjectScopeExtension: "tenant"}}},
		{
			name: "malformed exact scope",
			contract: apigenapi.GenOperationContract{
				Path:       "/api/v1/workspaces/{workspace}/dashboards/{dashboard}",
				Extensions: map[string]any{apiGenObjectScopeExtension: map[string]any{"kind": "dashboard"}},
			},
		},
		{name: "unexpected exact scope", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards", Extensions: map[string]any{apiGenObjectScopeExtension: "dashboard"}}},
		{name: "ambiguous exact scope", contract: apigenapi.GenOperationContract{Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}/semantic-models/{model}", Extensions: map[string]any{apiGenObjectScopeExtension: "dashboard"}}},
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
		"getActiveManagedDataRevision":         access.PrivilegeViewData,
		"listManagedConnections":               access.PrivilegeViewData,
		"getManagedConnection":                 access.PrivilegeViewData,
		"listManagedDataRevisions":             access.PrivilegeViewData,
		"getManagedDataRevision":               access.PrivilegeViewData,
		"createManagedDataUploadSession":       access.PrivilegeIngestData,
		"getManagedDataUploadSession":          access.PrivilegeIngestData,
		"listManagedDataUploadSessions":        access.PrivilegeIngestData,
		"cancelManagedDataUploadSession":       access.PrivilegeIngestData,
		"listManagedDataUploadSessionEvents":   access.PrivilegeIngestData,
		"finalizeManagedDataUploadSession":     access.PrivilegeIngestData,
		"createManagedDataS3MultipartUpload":   access.PrivilegeIngestData,
		"signManagedDataS3MultipartPart":       access.PrivilegeIngestData,
		"completeManagedDataS3MultipartUpload": access.PrivilegeIngestData,
		"abortManagedDataS3MultipartUpload":    access.PrivilegeIngestData,
		"createDeployment":                     access.PrivilegeActivateDeployment,
		"getDeployment":                        access.PrivilegeViewItem,
		"listDeployments":                      access.PrivilegeViewItem,
		"cancelDeployment":                     access.PrivilegeActivateDeployment,
		"rollbackDeployment":                   access.PrivilegeActivateDeployment,
	}
	for operationID, expected := range want {
		contract, ok := apigenapi.GetAPIGenOperationContract(operationID)
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
