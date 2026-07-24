package model

type ErrorResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"requestId"`
}

type PageInfo struct {
	NextCursor string `json:"nextCursor"`
}

type ListResponse[T any] struct {
	Items []T      `json:"items"`
	Page  PageInfo `json:"page"`
}
