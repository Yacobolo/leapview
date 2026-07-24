package apigenruntime

import (
	"fmt"
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	apiprotocol "github.com/Yacobolo/leapview/internal/app/api/protocol"
)

type Authorizer interface {
	Protect(operationID string, next http.Handler) (http.Handler, bool)
}

type Handler struct {
	authorizer Authorizer
	dispatcher apigenapi.GenOperationDispatcher
	responder  apigenapi.GenTransportErrorResponder
}

func Build(authorizer Authorizer, dispatcher apigenapi.GenOperationDispatcher, responder apigenapi.GenTransportErrorResponder) (*Handler, error) {
	if authorizer == nil {
		return nil, fmt.Errorf("APIGen authorizer is required")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("APIGen dispatcher is required")
	}
	return &Handler{authorizer: authorizer, dispatcher: dispatcher, responder: responder}, nil
}

func (h *Handler) HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request) {
	protected, ok := h.authorizer.Protect(operationID, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffered := apiprotocol.NewResponseBuffer(w, r)
		if ok := apigenapi.DispatchAPIGenOperation(operationID, h.dispatcher, h.responder, buffered, r); !ok {
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
