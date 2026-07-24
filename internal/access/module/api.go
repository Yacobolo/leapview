package module

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
)

func (m *Module) UpdatePrincipal(w http.ResponseWriter, r *http.Request, headers apigenapi.GenUpdatePrincipalHeaders) {
	r.Header.Set("If-Match", headers.IfMatch)
	m.handler.UpdatePrincipal(w, r)
}

func (m *Module) UpdateServicePrincipal(w http.ResponseWriter, r *http.Request, headers apigenapi.GenUpdateServicePrincipalHeaders) {
	r.Header.Set("If-Match", headers.IfMatch)
	m.handler.UpdateServicePrincipal(w, r)
}

func (m *Module) UpdateGrant(w http.ResponseWriter, r *http.Request, headers apigenapi.GenUpdateGrantHeaders) {
	r.Header.Set("If-Match", headers.IfMatch)
	m.handler.UpdateGrant(w, r)
}

func (m *Module) UpdateDataPolicy(w http.ResponseWriter, r *http.Request, headers apigenapi.GenUpdateDataPolicyHeaders) {
	r.Header.Set("If-Match", headers.IfMatch)
	m.handler.UpdateDataPolicy(w, r)
}

func (m *Module) UpdateGroup(w http.ResponseWriter, r *http.Request, headers apigenapi.GenUpdateGroupHeaders) {
	r.Header.Set("If-Match", headers.IfMatch)
	m.handler.UpdateGroup(w, r)
}

func (m *Module) UpdateRoleBinding(w http.ResponseWriter, r *http.Request, headers apigenapi.GenUpdateRoleBindingHeaders) {
	r.Header.Set("If-Match", headers.IfMatch)
	m.handler.UpdateRoleBinding(w, r)
}
