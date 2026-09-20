package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/service"
)

func registerRBACRoutes(
	mux *http.ServeMux,
	handler *handlers.RBACHandler,
	authMW func(http.Handler) http.Handler,
	rbac *service.RBACService,
) {
	admin := NewGroup(
		mux,
		"/api/v1/admin/roles",
		authMW,
	)

	admin.HandleFunc(
		"GET /{id}/permissions",
		handler.GetRolePermissions,
		middleware.RequirePermission(
			rbac,
			"role:read",
		),
	)

	admin.HandleFunc(
		"POST /{id}/permissions/{permission_id}",
		handler.AddPermission,
		middleware.RequirePermission(
			rbac,
			"role:permission:add",
		),
	)

	admin.HandleFunc(
		"DELETE /{id}/permissions/{permission_id}",
		handler.RemovePermission,
		middleware.RequirePermission(
			rbac,
			"role:permission:remove",
		),
	)
}
