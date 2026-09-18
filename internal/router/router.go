// no third-party routing framework.
package router

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/example/goapi/internal/config"
	"github.com/example/goapi/internal/email"
	"github.com/example/goapi/internal/file"
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
	requestLogRepo := repository.NewRequestLogRepository(db)

	emailService := email.NewService(email.Config{
		Host:     cfg.EmailHost,
		Port:     cfg.EmailPort,
		Username: cfg.EmailUsername,
		Password: cfg.EmailPassword,
		From:     cfg.EmailFrom,
	})

	profileRepo := repository.NewProfileRepository(db)
	cloudinaryService, _ := file.NewCloudinaryService(file.Config{
		CloudName: cfg.CloudinaryCloudName,
		APIKey:    cfg.CloudinaryAPIKey,
		APISecret: cfg.CloudinaryAPISecret,
	})

	profileService := service.NewProfileService(profileRepo, cloudinaryService)

	authService := service.NewAuthService(userRepo, jwtManager, emailService, cfg.BcryptCost)
	postService := service.NewPostService(postRepo)
	auditService := service.NewAuditService(requestLogRepo)

	authHandler := handlers.NewAuthHandler(authService)
	postHandler := handlers.NewPostHandler(postService)
	healthHandler := handlers.NewHealthHandler(db)
	profileHandler := handlers.NewProfileHandler(profileService)

	mux := http.NewServeMux()

	authMW := middleware.Auth(jwtManager)

	registerHealthRoutes(mux, healthHandler)
	registerAuthRoutes(mux, authHandler)
	registerPostRoutes(mux, postHandler, authMW)
	registerProfileRoutes(mux, profileHandler, authMW)
	registerNotFoundRoute(mux)

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS)

	// Global middleware chain applied to every route, outermost first.
	chain := middleware.Chain(
		middleware.BodyDump(func(r *http.Request, status int, duration time.Duration, reqBody, resBody []byte) {
			requestID := middleware.RequestIDFromContext(r.Context())
			auditService.LogRequest(context.WithoutCancel(r.Context()), r, status, duration, requestID, reqBody, resBody)
		}),
		middleware.RequestID,
		middleware.Recover,
		middleware.Logging,
		middleware.SecurityHeaders,
		middleware.CORS([]string{"*"}),
		rateLimiter.Middleware,
	)

	// return chain(mux)
	return chain(MethodNotAllowedJSON(mux))
}
