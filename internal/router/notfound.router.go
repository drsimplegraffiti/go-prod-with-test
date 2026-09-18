package router

import "net/http"

// registerNotFoundRoute wires up a catch-all handler for any request that
// doesn't match one of the explicitly registered routes.
//
// The "/" pattern matches every path not matched by a more specific
// registered pattern (per net/http.ServeMux's routing rules), so this acts
// as the mux's fallback 404 — it takes no method prefix, so it catches
// every HTTP method on any unmatched path.
func registerNotFoundRoute(mux *http.ServeMux) {
	mux.HandleFunc("/", notFoundHandler)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":"Not Found","message":"the requested resource was not found"}`))
}
