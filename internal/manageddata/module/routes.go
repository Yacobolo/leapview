package module

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (m *Module) MountTus(r chi.Router, handler http.Handler, protect func(http.Handler) http.Handler) {
	if m == nil || handler == nil || protect == nil {
		return
	}
	protected := protect(handler)
	r.Handle("/upload-protocols/tus", protected)
	r.Handle("/upload-protocols/tus/*", protected)
}
