package http

import agentgen "github.com/Yacobolo/leapview/internal/agent/api/gen"

var _ agentgen.GenOperationDispatcher = (*APIGenDispatcher)(nil)
var _ agentgen.GenTransportErrorResponder = APIGenTransportErrorResponder{}
