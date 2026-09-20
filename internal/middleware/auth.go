package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/example/goapi/internal/utils"
)

type contextKey string

const (
	userIDKey   contextKey = "user_id"
	userRoleKey contextKey = "user_role"
)

// Auth returns middleware that requires a valid Bearer access token,
// injecting the authenticated user's ID and role into the request context.
func Auth(jwtManager *utils.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				utils.WriteError(w, http.StatusUnauthorized, "missing_token", "Authorization header is required")
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				utils.WriteError(w, http.StatusUnauthorized, "malformed_token", "Authorization header must be 'Bearer <token>'")
				return
			}

			claims, err := jwtManager.Verify(parts[1], utils.AccessToken)
			if err != nil {
				utils.WriteError(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
				return
			}

			// ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			// ctx = context.WithValue(ctx, userRoleKey, claims.Role)

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userRoleKey, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user's ID set by Auth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// UserRoleFromContext extracts the authenticated user's role set by Auth.
func UserRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(userRoleKey).(string)
	return role, ok
}

// RequireRole returns middleware that rejects requests from users whose role
// (already set by Auth) is not in the allowed list. Must be chained after Auth.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := UserRoleFromContext(r.Context())
			if !ok {
				utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
				return
			}
			if _, allowed := allowedSet[role]; !allowed {
				utils.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to perform this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
