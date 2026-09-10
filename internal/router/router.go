// Package router assembles the full HTTP route table using only the
// standard library's http.ServeMux (Go 1.22+ method-aware patterns), with
// no third-party routing framework.
package router

import (
	"database/sql"
	"net/http"

	"github.com/example/goapi/internal/config"
	"github.com/example/goapi/internal/handlers"
	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/repository"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

// New builds the fully-wired HTTP handler: repositories -> services ->
// handlers -> routes -> middleware chain.
func New(db *sql.DB, cfg *config.Config) http.Handler {
	jwtManager := utils.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	userRepo := repository.NewUserRepository(db)
	postRepo := repository.NewPostRepository(db)

	authService := service.NewAuthService(userRepo, jwtManager, cfg.BcryptCost)
	postService := service.NewPostService(postRepo)

	authHandler := handlers.NewAuthHandler(authService)
	postHandler := handlers.NewPostHandler(postService)
	healthHandler := handlers.NewHealthHandler(db)

	mux := http.NewServeMux()

	// --- Health / operational endpoints (unauthenticated, unversioned) ---
	mux.HandleFunc("GET /healthz", healthHandler.Live)
	mux.HandleFunc("GET /readyz", healthHandler.Ready)

	// --- Public auth endpoints ---
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)

	// --- Public read endpoints for posts ---
	mux.HandleFunc("GET /api/v1/posts", postHandler.List)
	mux.HandleFunc("GET /api/v1/posts/{id}", postHandler.Get)

	// --- Authenticated write endpoints for posts ---
	authMW := middleware.Auth(jwtManager)
	mux.Handle("POST /api/v1/posts", authMW(http.HandlerFunc(postHandler.Create)))
	mux.Handle("PATCH /api/v1/posts/{id}", authMW(http.HandlerFunc(postHandler.Update)))
	mux.Handle("DELETE /api/v1/posts/{id}", authMW(http.HandlerFunc(postHandler.Delete)))

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS)

	// Global middleware chain applied to every route, outermost first.
	chain := middleware.Chain(
		middleware.RequestID,
		middleware.Recover,
		middleware.Logging,
		middleware.SecurityHeaders,
		middleware.CORS([]string{"*"}),
		rateLimiter.Middleware,
	)

	return chain(mux)
}
