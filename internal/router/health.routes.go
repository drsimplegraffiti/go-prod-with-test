package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
)

// registerHealthRoutes wires up the unauthenticated, unversioned
// operational endpoints.
func registerHealthRoutes(mux *http.ServeMux, healthHandler *handlers.HealthHandler) {
	mux.HandleFunc("GET /healthz", healthHandler.Live)
	mux.HandleFunc("GET /readyz", healthHandler.Ready)
}
