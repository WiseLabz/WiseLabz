package middleware

import (
	"net/http"
	"strings"
)

// CacheHeaders adds appropriate Cache-Control headers to responses:
// - /assets/* gets public, immutable cache with 1-year max-age
// - /index.html gets no-cache
// - Other paths get no caching
func CacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/assets/") {
			// Hashed Vite bundle assets: immutable, cache for 1 year
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "/" || path == "/index.html" {
			// index.html: must revalidate on every load
			w.Header().Set("Cache-Control", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}
