package app

import (
	"log/slog"
	"net/http"

	apihttpmiddleware "github.com/Yacobolo/leapview/internal/platform/http/middleware"
)

type RateLimitConfig = apihttpmiddleware.RateLimitConfig
type SecurityHeadersConfig = apihttpmiddleware.SecurityHeadersConfig
type RequestBodyLimitConfig = apihttpmiddleware.RequestBodyLimitConfig

func ProductionRateLimitConfig() RateLimitConfig {
	return apihttpmiddleware.ProductionRateLimitConfig()
}

func SecurityHeaders(hsts bool) SecurityHeadersConfig {
	return apihttpmiddleware.SecurityHeaders(hsts)
}

func requestBodyLimit(config RequestBodyLimitConfig) func(http.Handler) http.Handler {
	return apihttpmiddleware.RequestBodyLimit(config)
}

func panicRecovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return apihttpmiddleware.PanicRecovery(logger)
}
