package auth

import "net/http"

// NewAllowlistMiddleware wraps the given handler. Auth is currently enforced
// solely by the ConnectRPC interceptor, so this middleware passes all requests
// through unchanged. It exists as a placeholder: when HTTP-level auth is added,
// requests to allowedPaths will bypass that check (exact match).
func NewAllowlistMiddleware(_ []string, next http.Handler) http.Handler {
	return next
}
