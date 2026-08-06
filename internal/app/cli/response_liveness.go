package cli

import (
	"net/http"
	"time"

	httpmiddleware "github.com/flidai/leapview/internal/platform/http/middleware"
)

// withResponseLiveness applies write deadlines at request scope.  A server's
// WriteTimeout cannot be used here because it is absolute for the whole
// connection, whereas Datastar streams may remain open indefinitely while
// continuing to publish events.
func withResponseLiveness(next http.Handler, ordinaryTimeout, streamIdleTimeout time.Duration) http.Handler {
	return httpmiddleware.ResponseLiveness(next, ordinaryTimeout, streamIdleTimeout)
}
