package pkg

import (
	"net/http"

	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/utils"
)

// requireUserID extracts the authenticated user's ID from the request
// context. If the request is not authenticated it writes a 401 and returns
// false — the handler should simply `return` in that case:
//
//	userID, ok := requireUserID(w, r)
//	if !ok {
//	    return
//	}
//
// Note: middleware.Auth already rejects unauthenticated requests, so this
// guard is defense-in-depth for handlers that are (or might become)
// reachable without the middleware.
func requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return "", false
	}
	return userID, true
}
