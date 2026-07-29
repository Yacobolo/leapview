package module

import (
	"log/slog"
	"net/http"

	"github.com/flidai/leapview/internal/analytics/queryaudit"
	queryaudithttp "github.com/flidai/leapview/internal/analytics/queryaudit/http"
)

type QueryAuditAPIGenConfig struct {
	Reader      func() (queryaudit.Reader, error)
	WorkspaceID func(string) string
}

func DispatchQueryAuditAPIGenOperation(config QueryAuditAPIGenConfig, operationID string, logger *slog.Logger, w http.ResponseWriter, r *http.Request) bool {
	handler := queryaudithttp.Handler{
		Reader:      queryaudithttp.ReaderProvider(config.Reader),
		WorkspaceID: queryaudithttp.WorkspaceIDNormalizer(config.WorkspaceID),
	}
	return queryaudithttp.DispatchAPIGenOperation(operationID, handler, logger, w, r)
}
