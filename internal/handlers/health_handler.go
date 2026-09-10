package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/example/goapi/internal/utils"
)

// HealthHandler exposes liveness and readiness endpoints.
type HealthHandler struct {
	db *sql.DB
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live handles GET /healthz — always returns 200 once the process is up.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready handles GET /readyz — returns 200 only if the database is reachable.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "db_unreachable", "database is not reachable")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
