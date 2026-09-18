package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
)

// registerAuthRoutes wires up the public authentication endpoints.
func registerAuthRoutes(mux *http.ServeMux, authHandler *handlers.AuthHandler) {
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)
}
