package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
)

// registerProfileRoutes wires up authenticated profile endpoints.
func registerProfileRoutes(
	mux *http.ServeMux,
	profileHandler *handlers.ProfileHandler,
	authMW func(http.Handler) http.Handler,
) {
	// Authenticated profile reads
	mux.Handle("GET /api/v1/profile", authMW(http.HandlerFunc(profileHandler.GetProfile)))

	// Authenticated profile update
	mux.Handle("PATCH /api/v1/profile/update", authMW(http.HandlerFunc(profileHandler.UpdateProfile)))

	// Authenticated avatar upload
	mux.Handle("POST /api/v1/profile/avatar", authMW(http.HandlerFunc(profileHandler.UpdateAvatar)))
}
