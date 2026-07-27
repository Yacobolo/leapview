package module

import (
	"net/http"

	accessgen "github.com/Yacobolo/leapview/internal/access/api/gen"
	accesshttp "github.com/Yacobolo/leapview/internal/access/http"
)

func (m *Module) DispatchAPIGenOperation(operationID string, w http.ResponseWriter, r *http.Request) bool {
	return accessgen.DispatchAPIGenOperation(
		operationID,
		accesshttp.NewAPIGenDispatcher(m.handler),
		accesshttp.APIGenTransportErrorResponder{Logger: m.logger},
		w,
		r,
	)
}
