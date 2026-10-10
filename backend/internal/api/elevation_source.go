package api

import "net/http"

// elevationSource is implemented by route handlers whose middleware or handler
// performs an elevation check. It records the check's source for contract
// tests; it does not describe authorization policy.
type elevationSource interface {
	http.Handler
	ElevationSource() string
}

var _ elevationSource = sourcedElevationHandler{}

type sourcedElevationHandler struct {
	handler http.Handler
	source  string
}

func (h sourcedElevationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func (h sourcedElevationHandler) ElevationSource() string { return h.source }

func withElevationSource(source string, handler http.HandlerFunc) http.Handler {
	return sourcedElevationHandler{handler: handler, source: source}
}
