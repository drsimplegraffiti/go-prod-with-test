package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

type ProfileHandler struct {
	profile *service.ProfileService
}

func NewProfileHandler(profile *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profile: profile,
	}
}

func (h *ProfileHandler) GetProfile(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	profile, err := h.profile.GetProfile(
		r.Context(),
		userID,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "internal_error",
			err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) UpdateProfile(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	slog.Info("creating profile", "user_id", userID)

	var req models.UpdateProfileRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+err.Error())
		return
	}

	profile, err := h.profile.UpdateProfile(
		r.Context(),
		userID,
		req,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "internal_error",
			err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) UpdateAvatar(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+err.Error())
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+err.Error())
		return
	}

	defer file.Close()

	header := r.MultipartForm.File["avatar"][0]

	profile, err := h.profile.UpdateAvatar(
		r.Context(),
		userID,
		header,
	)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "internal_error",
			err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, profile)
}

func parseUserID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
