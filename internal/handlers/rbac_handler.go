package handlers

import (
	"errors"
	"net/http"

	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

type RBACHandler struct {
	rbac *service.RBACService
}

func NewRBACHandler(rbac *service.RBACService) *RBACHandler {
	return &RBACHandler{rbac: rbac}
}

func (h *RBACHandler) GetRolePermissions(
	w http.ResponseWriter,
	r *http.Request,
) {
	roleID := r.PathValue("id")

	permissions, err := h.rbac.GetRolePermissions(
		r.Context(),
		roleID,
	)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			utils.WriteError(w, http.StatusForbidden, "forbidden", "forbidden")
			return
		}

		utils.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get permissions")
		return
	}

	utils.WriteJSON(w, http.StatusOK, permissions)
}

func (h *RBACHandler) AddPermission(
	w http.ResponseWriter,
	r *http.Request,
) {
	roleID := r.PathValue("id")
	permissionID := r.PathValue("permission_id")

	err := h.rbac.AddPermission(
		r.Context(),
		roleID,
		permissionID,
	)
	if err != nil {
		utils.WriteError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			err.Error(),
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) RemovePermission(
	w http.ResponseWriter,
	r *http.Request,
) {
	roleID := r.PathValue("id")
	permissionID := r.PathValue("permission_id")

	err := h.rbac.RemovePermission(
		r.Context(),
		roleID,
		permissionID,
	)
	if err != nil {
		utils.WriteError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"failed to remove permission",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Keep middleware imported here only if you use UserRole checks
// directly later.
var _ = middleware.UserRoleFromContext
