package model

type ErrorResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"requestId"`
}

type ProblemFieldError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Field  string `json:"field"`
}

type ProblemDetails struct {
	Code      string              `json:"code"`
	Detail    string              `json:"detail"`
	Errors    []ProblemFieldError `json:"errors"`
	Instance  string              `json:"instance"`
	RequestID string              `json:"requestId"`
	Status    int32               `json:"status"`
	Title     string              `json:"title"`
	Type      string              `json:"type"`
}

type PageInfo struct {
	NextCursor string `json:"nextCursor"`
}

type ListResponse[T any] struct {
	Items []T      `json:"items"`
	Page  PageInfo `json:"page"`
}
