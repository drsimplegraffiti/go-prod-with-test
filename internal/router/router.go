// // no third-party routing framework.
// package router
//
// import (
// 	"context"
// 	"database/sql"
// 	"net/http"
// 	"time"
//
// 	"github.com/example/goapi/internal/config"
// 	"github.com/example/goapi/internal/email"
// 	"github.com/example/goapi/internal/file"
// 	"github.com/example/goapi/internal/handlers"
// 	"github.com/example/goapi/internal/middleware"
// 	"github.com/example/goapi/internal/repository"
// 	"github.com/example/goapi/internal/service"
// 	"github.com/example/goapi/internal/utils"
// )
//
// // New builds the fully-wired HTTP handler: repositories -> services ->
// // handlers -> routes -> middleware chain.
// func New(db *sql.DB, cfg *config.Config) http.Handler {
// 	jwtManager := utils.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
//
// 	userRepo := repository.NewUserRepository(db)
// 	postRepo := repository.NewPostRepository(db)
// 	requestLogRepo := repository.NewRequestLogRepository(db)
//
// 	emailService := email.NewService(email.Config{
// 		Host:     cfg.EmailHost,
// 		Port:     cfg.EmailPort,
// 		Username: cfg.EmailUsername,
// 		Password: cfg.EmailPassword,
// 		From:     cfg.EmailFrom,
// 	})
//
// 	profileRepo := repository.NewProfileRepository(db)
// 	cloudinaryService, _ := file.NewCloudinaryService(file.Config{
// 		CloudName: cfg.CloudinaryCloudName,
// 		APIKey:    cfg.CloudinaryAPIKey,
// 		APISecret: cfg.CloudinaryAPISecret,
// 	})
//
// 	profileService := service.NewProfileService(profileRepo, cloudinaryService)
//
// 	authService := service.NewAuthService(userRepo, jwtManager, emailService, cfg.BcryptCost)
// 	postService := service.NewPostService(postRepo)
// 	auditService := service.NewAuditService(requestLogRepo)
//
// 	authHandler := handlers.NewAuthHandler(authService)
// 	postHandler := handlers.NewPostHandler(postService)
// 	healthHandler := handlers.NewHealthHandler(db)
// 	profileHandler := handlers.NewProfileHandler(profileService)
//
// 	mux := http.NewServeMux()
//
// 	authMW := middleware.Auth(jwtManager)
//
// 	registerHealthRoutes(mux, healthHandler)
// 	registerAuthRoutes(mux, authHandler)
// 	registerPostRoutes(mux, postHandler, authMW)
// 	registerProfileRoutes(mux, profileHandler, authMW)
// 	registerNotFoundRoute(mux)
//
// 	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS)
//
// 	// Global middleware chain applied to every route, outermost first.
// 	chain := middleware.Chain(
// 		middleware.BodyDump(func(r *http.Request, status int, duration time.Duration, reqBody, resBody []byte) {
// 			requestID := middleware.RequestIDFromContext(r.Context())
// 			auditService.LogRequest(context.WithoutCancel(r.Context()), r, status, duration, requestID, reqBody, resBody)
// 		}),
// 		middleware.RequestID,
// 		middleware.Recover,
// 		middleware.Logging,
// 		middleware.SecurityHeaders,
// 		middleware.CORS([]string{"*"}),
// 		rateLimiter.Middleware,
// 	)
//
// 	// return chain(mux)
// 	return chain(MethodNotAllowedJSON(mux))
// }

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
func New(db *sql.DB, cfg *config.Config) (http.Handler, error) {
	jwtManager := utils.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	userRepo := repository.NewUserRepository(db)
	postRepo := repository.NewPostRepository(db)
	requestLogRepo := repository.NewRequestLogRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)

	emailService := email.NewService(email.Config{
		Host:     cfg.EmailHost,
		Port:     cfg.EmailPort,
		Username: cfg.EmailUsername,
		Password: cfg.EmailPassword,
		From:     cfg.EmailFrom,
	})

	profileRepo := repository.NewProfileRepository(db)
	cloudinaryService, err := file.NewCloudinaryService(file.Config{
		CloudName: cfg.CloudinaryCloudName,
		APIKey:    cfg.CloudinaryAPIKey,
		APISecret: cfg.CloudinaryAPISecret,
	})
	if err != nil {
		// Fail startup rather than silently running without file storage.
		return nil, err
	}

	profileService := service.NewProfileService(profileRepo, cloudinaryService)
	authService := service.NewAuthService(userRepo, refreshTokenRepo, jwtManager, emailService, cfg.BcryptCost)
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

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.TrustXFF)

	// Audit callback. Uses the request-scoped logger so request_id is
	// attached automatically. Bodies arrive already redacted by BodyDump.
	auditFn := func(r *http.Request, status int, duration time.Duration, reqBody, resBody []byte) {
		requestID := middleware.RequestIDFromContext(r.Context())
		auditService.LogRequest(context.WithoutCancel(r.Context()), r, status, duration, requestID, reqBody, resBody)
	}

	// Global middleware chain applied to every route, outermost first:
	//
	//	RequestID   -> correlation ID + request-scoped logger for everything below
	//	Recover     -> converts panics into 500s (with stack traces) — outermost
	//	               so it also covers middleware panics below it
	//	Logging     -> one access log per request, with final status
	//	SecurityHeaders -> baseline defensive headers (HSTS on in prod)
	//	CORS        -> preflight + origin handling
	//	RateLimiter -> per-IP token bucket (429s short-circuit cheaply here)
	//	BodyDump    -> audit capture; skips file routes; redacts secrets;
	//	               truncates bodies at 64 KiB
	chain := middleware.Chain(
		middleware.RequestID,
		middleware.Recover,
		middleware.Logging,
		middleware.SecurityHeaders(cfg.Env == "production"),
		middleware.CORS(cfg.AllowedOrigins),
		rateLimiter.Middleware,
		middleware.BodyDump(middleware.BodyDumpOptions{
			MaxBytes:  64 << 10,
			SkipPaths: []string{"/api/profiles/avatar"}, // TODO: match your real upload route
		}, auditFn),
	)

	return chain(MethodNotAllowedJSON(mux)), nil
}
