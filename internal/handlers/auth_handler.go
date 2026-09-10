package handlers

import (
	"encoding/json"
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

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+jsonErrDetail(err))
		return
	}

	resp, err := h.auth.Register(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, resp)
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+jsonErrDetail(err))
		return
	}

	resp, err := h.auth.Login(r.Context(), req)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, resp)
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+jsonErrDetail(err))
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

	utils.WriteJSON(w, http.StatusOK, resp)
}

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

func jsonErrDetail(err error) string {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return "malformed JSON"
	}
	return err.Error()
}
