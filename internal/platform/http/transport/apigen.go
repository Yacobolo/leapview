package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// APIGenFailure is the generated-transport-neutral error shape shared by
// independently generated capability packages.
type APIGenFailure struct {
	OperationID  string
	Kind         string
	StatusCode   int
	Code         string
	PublicDetail string
	Cause        error
}

func WriteAPIGenFailure(ctx context.Context, w http.ResponseWriter, r *http.Request, logger *slog.Logger, failure APIGenFailure) {
	if logger != nil && failure.Cause != nil {
		log := logger.DebugContext
		if failure.StatusCode >= http.StatusInternalServerError {
			log = logger.ErrorContext
		}
		log(ctx, "APIGen transport error", "operation", failure.OperationID, "kind", failure.Kind, "status", failure.StatusCode, "error", failure.Cause)
	}
	requestID := ""
	instance := ""
	if r != nil {
		requestID = r.Header.Get("X-Request-ID")
		instance = r.URL.Path
	}
	if requestID == "" {
		requestID = NewRequestID()
		if r != nil {
			r.Header.Set("X-Request-ID", requestID)
		}
	}
	w.Header().Set("X-Request-ID", requestID)
	problem := ProblemDetails{
		Type:  "https://leapview.dev/problems/" + strings.ToLower(strings.ReplaceAll(failure.Code, "_", "-")),
		Title: http.StatusText(failure.StatusCode), Status: int32(failure.StatusCode),
		Detail: failure.PublicDetail, Instance: instance, Code: failure.Code,
		RequestID: requestID, Errors: []ProblemFieldError{},
	}
	if field := apigenFailureField(failure); field != "" {
		problem.Detail = strings.TrimSuffix(failure.PublicDetail, ".") + " \"" + field + "\"."
		problem.Errors = append(problem.Errors, ProblemFieldError{
			Code: failure.Code, Detail: failure.PublicDetail, Field: field,
		})
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(failure.StatusCode)
	_ = writeJSONBody(w, problem)
}

func apigenFailureField(failure APIGenFailure) string {
	if failure.Cause == nil {
		return ""
	}
	switch failure.Kind {
	case "path_parameter", "query_parameter", "header_parameter":
	default:
		return ""
	}
	message := failure.Cause.Error()
	const marker = "parameter \""
	start := strings.Index(message, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.IndexByte(message[start:], '"')
	if end < 0 {
		return ""
	}
	return message[start : start+end]
}

func writeJSONBody(w http.ResponseWriter, value any) error {
	return json.NewEncoder(w).Encode(value)
}
