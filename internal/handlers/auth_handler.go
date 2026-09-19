package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

// AuthHandler exposes HTTP endpoints for registration, login and refresh.
type AuthHandler struct {
	auth *service.AuthService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if !utils.ReadJSON(w, r, &req) {
		return
	}

	resp, err := h.auth.Register(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setRefreshCookie(w, resp.RefreshToken)
	utils.WriteJSON(w, http.StatusCreated, resp)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if !utils.ReadJSON(w, r, &req) {
		return
	}

	resp, err := h.auth.Login(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setRefreshCookie(w, resp.RefreshToken)
	utils.WriteJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshRequest
	if !utils.ReadJSON(w, r, &req) {
		return
	}
	if req.RefreshToken == "" {
		utils.WriteError(w, http.StatusBadRequest, "missing_field", "refresh_token is required")
		return
	}

	resp, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	setRefreshCookie(w, resp.RefreshToken)
	utils.WriteJSON(w, http.StatusOK, resp)
}

// Logout handles POST /api/v1/auth/logout. Since refresh tokens are
// httpOnly cookies, clearing the cookie is what ends the session on this
// device. (Revoking server-side too is a nice addition — call
// authService.Revoke with the cookie value if you add that method.)
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/", // must match the path used when setting it
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})
	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// setRefreshCookie centralizes the cookie policy so every endpoint that
// issues tokens sets it identically. Change the policy once, applies
// everywhere.
func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",                  // without Path, cookie defaults to the request path and refresh breaks
		HttpOnly: true,                 // not readable from JS — mitigates XSS token theft
		SameSite: http.SameSiteLaxMode, // CSRF mitigation for cookie-based auth
		Secure:   true,                 // HTTPS only
		MaxAge:   60 * 60 * 24 * 30,    // 30 days; align with cfg.JWTRefreshTTL
	})
}

// writeAuthError maps service-layer errors to HTTP responses in one place.
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		utils.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, service.ErrEmailTaken):
		utils.WriteError(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		utils.WriteError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, utils.ErrInvalidToken):
		utils.WriteError(w, http.StatusUnauthorized, "invalid_token", err.Error())
	default:
		slog.Error("auth handler internal error", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}
