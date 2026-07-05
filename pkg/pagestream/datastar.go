package pagestream

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	ds "github.com/starfederation/datastar-go/datastar"
)

const DefaultClientIDCookieName = "ld_client_id"

func ReadSignals(r *http.Request, target any) error {
	return ds.ReadSignals(r, target)
}

type SignalWriter struct {
	sse *ds.ServerSentEventGenerator
}

func NewSignalWriter(w http.ResponseWriter, r *http.Request) SignalWriter {
	return SignalWriter{sse: ds.NewSSE(w, r)}
}

func (w SignalWriter) Patch(patch Patch) error {
	return w.sse.MarshalAndPatchSignals(patch)
}

func EnsureClientID(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(DefaultClientIDCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	clientID := newClientID()
	http.SetCookie(w, &http.Cookie{
		Name:     DefaultClientIDCookieName,
		Value:    clientID,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
	return clientID
}

func ClientIDFromRequest(r *http.Request, signalClientID string) string {
	if signalClientID != "" {
		return signalClientID
	}
	cookie, err := r.Cookie(DefaultClientIDCookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return "default"
}

func newClientID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes[:])
}
