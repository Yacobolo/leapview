package module

import (
	"net/http"
)

func (m *Module) UpdatePrincipal(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdatePrincipal(w, r)
}

func (m *Module) UpdateServicePrincipal(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdateServicePrincipal(w, r)
}

func (m *Module) UpdateGrant(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdateGrant(w, r)
}

func (m *Module) UpdateDataPolicy(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdateDataPolicy(w, r)
}

func (m *Module) UpdateGroup(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdateGroup(w, r)
}

func (m *Module) UpdateRoleBinding(w http.ResponseWriter, r *http.Request, ifMatch string) {
	r.Header.Set("If-Match", ifMatch)
	m.handler.UpdateRoleBinding(w, r)
}
