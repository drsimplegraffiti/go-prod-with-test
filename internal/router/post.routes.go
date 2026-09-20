package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/service"
)

func registerPostRoutes(
	mux *http.ServeMux,
	postHandler *handlers.PostHandler,
	authMW func(http.Handler) http.Handler,
	rbac *service.RBACService,
) {
	const url = "/api/v1/posts"

	// Public reads.
	public := NewGroup(mux, url)

	public.HandleFunc("GET /", postHandler.List)
	public.HandleFunc("GET /{id}", postHandler.Get)

	// Authenticated + permission-protected writes.
	protected := NewGroup(
		mux,
		url,
		authMW,
	)

	protected.HandleFunc(
		"POST /",
		postHandler.Create,
		middleware.RequirePermission(rbac, "post:create"),
	)

	protected.HandleFunc(
		"PATCH /{id}",
		postHandler.Update,
		middleware.RequirePermission(rbac, "post:update"),
	)

	protected.HandleFunc(
		"DELETE /{id}",
		postHandler.Delete,
		middleware.RequirePermission(rbac, "post:delete"),
	)

	// protected.HandleFunc(
	// 	"POST /{id}/publish",
	// 	postHandler.Publish,
	// 	middleware.RequirePermission(
	// 		rbac,
	// 		"post:update",
	// 		"post:publish",
	// 	),
	// )
}

// INSERT INTO permissions (name, description)
// VALUES
//     ('post:create', 'Create posts'),
//     ('post:read', 'Read posts'),
//     ('post:update', 'Update posts'),
//     ('post:delete', 'Delete posts'),
//     ('role:read', 'View roles'),
//     ('role:permission:add', 'Add permissions to roles'),
//     ('role:permission:remove', 'Remove permissions from roles')
// ON CONFLICT (name) DO NOTHING;
//
// INSERT INTO role_permissions (role_id, permission_id)
// SELECT
//     r.id,
//     p.id
// FROM roles r
// CROSS JOIN permissions p
// WHERE r.name = 'admin'
// ON CONFLICT DO NOTHING;
