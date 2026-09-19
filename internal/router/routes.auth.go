package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
)

// registerAuthRoutes wires up the public authentication endpoints.
func registerAuthRoutes(mux *http.ServeMux, authHandler *handlers.AuthHandler) {
	auth := NewGroup(mux, "/api/v1/auth")

	auth.HandleFunc("POST /register", authHandler.Register)
	auth.HandleFunc("POST /login", authHandler.Login)
	auth.HandleFunc("POST /refresh", authHandler.Refresh)

	// mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	// mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	// mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)
}
