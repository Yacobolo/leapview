package apigenruntime

import (
	"fmt"
	"net/http"

	apiprotocol "github.com/Yacobolo/leapview/internal/app/api/protocol"
)

type Authorizer interface {
	Protect(operationID string, next http.Handler) (http.Handler, bool)
}

type Handler struct {
	authorizer Authorizer
	dispatch   Dispatch
}

type Dispatch func(operationID string, w http.ResponseWriter, r *http.Request) bool

func Build(authorizer Authorizer, dispatch Dispatch) (*Handler, error) {
	if authorizer == nil {
		return nil, fmt.Errorf("APIGen authorizer is required")
	}
	if dispatch == nil {
		return nil, fmt.Errorf("APIGen dispatch function is required")
	}
	return &Handler{authorizer: authorizer, dispatch: dispatch}, nil
}

func (h *Handler) HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request) {
	protected, ok := h.authorizer.Protect(operationID, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffered := apiprotocol.NewResponseBuffer(w, r)
		if ok := h.dispatch(operationID, buffered, r); !ok {
			http.NotFound(w, r)
			return
		}
		buffered.Flush()
	}))
	if !ok {
		http.NotFound(w, r)
		return
	}
	protected.ServeHTTP(w, r)
}
