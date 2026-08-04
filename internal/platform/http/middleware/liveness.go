package httpmiddleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ResponseLiveness applies write deadlines at request scope. A server's
// WriteTimeout cannot be used for this because it is absolute for the whole
// connection, whereas Datastar streams may remain open indefinitely while
// continuing to publish events.
func ResponseLiveness(next http.Handler, ordinaryTimeout, streamIdleTimeout time.Duration) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestContext, cancel := context.WithCancel(r.Context())
		defer cancel()
		var ordinaryDeadline time.Time
		if ordinaryTimeout > 0 {
			ordinaryDeadline = time.Now().Add(ordinaryTimeout)
		}
		writer := &responseLivenessWriter{
			ResponseWriter:   w,
			ordinaryDeadline: ordinaryDeadline,
			streamIdle:       streamIdleTimeout,
			cancel:           cancel,
		}
		if ordinaryTimeout > 0 {
			writer.ordinaryTimer = time.AfterFunc(ordinaryTimeout, cancel)
		}
		defer writer.stopTimers()
		writer.setDeadline(writer.ordinaryDeadline)
		next.ServeHTTP(writer, r.WithContext(requestContext))
	})
}

type responseLivenessWriter struct {
	http.ResponseWriter
	ordinaryDeadline time.Time
	streamIdle       time.Duration
	stream           bool
	cancel           context.CancelFunc
	mu               sync.Mutex
	ordinaryTimer    *time.Timer
	streamTimer      *time.Timer
}

func (w *responseLivenessWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseLivenessWriter) WriteHeader(status int) {
	w.detectStream()
	if w.stream {
		w.refreshStreamDeadline()
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseLivenessWriter) Write(p []byte) (int, error) {
	w.detectStream()
	if w.stream {
		w.refreshStreamDeadline()
	}
	return w.ResponseWriter.Write(p)
}

func (w *responseLivenessWriter) Flush() {
	// A Flush with a text/event-stream content type is the first observable
	// indication that this response is a stream. Set an idle deadline before
	// flushing so a blocked write to a stalled client is eventually released.
	w.detectStream()
	if w.stream {
		w.refreshStreamDeadline()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseLivenessWriter) detectStream() {
	if w.stream {
		return
	}
	contentType := strings.TrimSpace(strings.ToLower(strings.SplitN(w.Header().Get("Content-Type"), ";", 2)[0]))
	if contentType == "text/event-stream" {
		// A handler must identify the stream before the ordinary response
		// budget expires. Do not clear an already-fired write deadline merely
		// because non-cooperative code attempts its first write late.
		if !w.ordinaryDeadline.IsZero() && !time.Now().Before(w.ordinaryDeadline) {
			return
		}
		w.stream = true
		w.mu.Lock()
		if w.ordinaryTimer != nil {
			w.ordinaryTimer.Stop()
		}
		w.mu.Unlock()
		// Remove the ordinary absolute deadline before the first stream write.
		w.setDeadline(time.Time{})
	}
}

func (w *responseLivenessWriter) refreshStreamDeadline() {
	w.refreshStreamIdleTimer()
	if w.streamIdle <= 0 {
		w.setDeadline(time.Time{})
		return
	}
	w.setDeadline(time.Now().Add(w.streamIdle))
}

func (w *responseLivenessWriter) refreshStreamIdleTimer() {
	if w.streamIdle <= 0 || w.cancel == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.streamTimer == nil {
		w.streamTimer = time.AfterFunc(w.streamIdle, w.cancel)
		return
	}
	w.streamTimer.Reset(w.streamIdle)
}

func (w *responseLivenessWriter) stopTimers() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ordinaryTimer != nil {
		w.ordinaryTimer.Stop()
	}
	if w.streamTimer != nil {
		w.streamTimer.Stop()
	}
}

func (w *responseLivenessWriter) setDeadline(deadline time.Time) {
	_ = http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
}
