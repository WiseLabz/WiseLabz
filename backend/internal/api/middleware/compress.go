package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// Compress wraps chi's built-in Compress middleware, skipping compression for
// WebSocket upgrades and for responses that are already compressed.
func Compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip compression for WebSocket upgrades
		if isWebSocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}
		middleware.Compress(5)(next).ServeHTTP(w, r)
	})
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}
