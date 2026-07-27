package cli

type apiListResponse[T any] struct {
	Items []T `json:"items"`
	Page  struct {
		NextCursor string `json:"nextCursor"`
	} `json:"page"`
}
