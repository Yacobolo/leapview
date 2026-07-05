package app

import (
	"net/http"

	adminhttp "github.com/Yacobolo/libredash/internal/admin/http"
	"github.com/Yacobolo/libredash/internal/dashboard"
	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
)

func (s *Server) adminHTTPHandler() adminhttp.Handler {
	return adminhttp.Handler{
		Catalog: func() dashboard.Catalog {
			return s.metrics.Catalog()
		},
		Data:                s.adminData,
		CurrentRoleLabel:    s.currentAdminRoleLabel,
		ChromeOption:        s.chatChromeOption,
		EnsureClientID:      func(w http.ResponseWriter, r *http.Request) { _ = lddatastar.EnsureClientID(w, r) },
		QueryHistoryUpdates: s.adminQueryHistoryUpdates,
		QueryHistoryCommand: s.adminQueryHistoryCommand,
		StorageUpdates:      s.adminStorageUpdates,
		StorageSelectTable:  s.adminStorageSelectTable,
	}
}
