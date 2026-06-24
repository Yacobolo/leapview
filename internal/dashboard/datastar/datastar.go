package datastar

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/Yacobolo/libredash/internal/dashboard"
	ds "github.com/starfederation/datastar-go/datastar"
)

const ClientIDCookieName = "ld_client_id"

func ReadSignals(r *http.Request, signals *dashboard.Signals) error {
	return ds.ReadSignals(r, signals)
}

func EnsureClientID(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(ClientIDCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	clientID := newClientID()
	http.SetCookie(w, &http.Cookie{
		Name:     ClientIDCookieName,
		Value:    clientID,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
	return clientID
}

func DashboardID(r *http.Request, signals dashboard.Signals, defaultID string) string {
	if id := r.URL.Query().Get("dashboard"); id != "" {
		return id
	}
	if signals.Runtime.DashboardID != "" {
		return signals.Runtime.DashboardID
	}
	return defaultID
}

func PageID(r *http.Request, signals dashboard.Signals) string {
	if id := r.URL.Query().Get("page"); id != "" {
		return id
	}
	if signals.Runtime.PageID != "" {
		return signals.Runtime.PageID
	}
	return ""
}

func ModelID(r *http.Request, signals dashboard.Signals, dashboardID string, defaultForDashboard func(string) string) string {
	if id := r.URL.Query().Get("model"); id != "" {
		return id
	}
	if signals.Runtime.ModelID != "" {
		return signals.Runtime.ModelID
	}
	return defaultForDashboard(dashboardID)
}

func ClientStreamID(r *http.Request, signals dashboard.Signals, dashboardID, pageID string) string {
	return ClientIDFromRequest(r, signals) + ":" + dashboardID + ":" + pageID
}

func ClientIDFromRequest(r *http.Request, signals dashboard.Signals) string {
	if signals.Runtime.ClientID != "" {
		return signals.Runtime.ClientID
	}
	cookie, err := r.Cookie(ClientIDCookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return "default"
}

func DashboardPatch(patch dashboard.Patch) map[string]any {
	return map[string]any{
		"filters":       patch.Filters,
		"filterOptions": patch.FilterOptions,
		"status":        patch.Status,
		"visuals":       patch.Visuals,
	}
}

func TablePatch(name string, table dashboard.Table) map[string]any {
	return map[string]any{
		"tables": map[string]dashboard.Table{
			name: table,
		},
	}
}

func TablesPatch(tables map[string]dashboard.Table) map[string]any {
	return map[string]any{"tables": tables}
}

func LoadingPatch(dataDir string) map[string]any {
	return map[string]any{
		"status": map[string]any{
			"loading":       true,
			"error":         "",
			"dataDirectory": dataDir,
		},
	}
}

func newClientID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes[:])
}
