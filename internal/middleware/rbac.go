// package middleware
//
// import (
// 	"net/http"
//
// 	"github.com/example/goapi/internal/models"
// 	"github.com/example/goapi/internal/utils"
// )
//
// // RequirePermission allows a request when the authenticated user's role
// // contains the required permission.
// //
// // Must run after Auth middleware.
//
// func RequirePermission(
// 	required ...models.Permission,
// ) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			role, ok := UserRoleFromContext(r.Context())
// 			if !ok {
// 				utils.WriteError(
// 					w,
// 					http.StatusUnauthorized,
// 					"unauthenticated",
// 					"authentication required",
// 				)
// 				return
// 			}
//
// 			permissions, ok := models.RolePermissions[models.Role(role)]
// 			if !ok {
// 				utils.WriteError(
// 					w,
// 					http.StatusForbidden,
// 					"forbidden",
// 					"invalid user role",
// 				)
// 				return
// 			}
//
// 			for _, permission := range required {
// 				if _, ok := permissions[permission]; !ok {
// 					utils.WriteError(
// 						w,
// 						http.StatusForbidden,
// 						"forbidden",
// 						"you do not have permission to perform this action",
// 					)
// 					return
// 				}
// 			}
//
// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }
//
// // For multiple required permissions, use RequireAnyPermission.
// func RequireAnyPermission(
// 	required ...models.Permission,
// ) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			role, ok := UserRoleFromContext(r.Context())
// 			if !ok {
// 				utils.WriteError(
// 					w,
// 					http.StatusUnauthorized,
// 					"unauthenticated",
// 					"authentication required",
// 				)
// 				return
// 			}
//
// 			permissions, ok := models.RolePermissions[models.Role(role)]
// 			if !ok {
// 				utils.WriteError(
// 					w,
// 					http.StatusForbidden,
// 					"forbidden",
// 					"invalid user role",
// 				)
// 				return
// 			}
//
// 			for _, permission := range required {
// 				if _, ok := permissions[permission]; ok {
// 					next.ServeHTTP(w, r)
// 					return
// 				}
// 			}
//
// 			utils.WriteError(
// 				w,
// 				http.StatusForbidden,
// 				"forbidden",
// 				"you do not have permission to perform this action",
// 			)
// 		})
// 	}
// }

// package middleware
//
// import (
// 	"errors"
// 	"net/http"
//
// 	"github.com/example/goapi/internal/service"
// 	"github.com/example/goapi/internal/utils"
// )
//
// func RequirePermission(
// 	rbac *service.RBACService,
// 	permission string,
// ) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			roleID, ok := UserRoleFromContext(r.Context())
// 			if !ok || roleID == "" {
// 				utils.WriteError(
// 					w,
// 					http.StatusUnauthorized,
// 					"unauthenticated",
// 					"authentication required",
// 				)
// 				return
// 			}
//
// 			err := rbac.Authorize(
// 				r.Context(),
// 				roleID,
// 				permission,
// 			)
// 			if err != nil {
// 				if errors.Is(err, service.ErrForbidden) {
// 					utils.WriteError(
// 						w,
// 						http.StatusForbidden,
// 						"forbidden",
// 						"you do not have permission to perform this action",
// 					)
// 					return
// 				}
//
// 				utils.WriteError(
// 					w,
// 					http.StatusInternalServerError,
// 					"authorization_error",
// 					"failed to verify permissions",
// 				)
// 				return
// 			}
//
// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }

package middleware

import (
	"errors"
	"net/http"

	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

// RequirePermission requires the user to have ALL supplied permissions.
func RequirePermission(
	rbac *service.RBACService,
	permissions ...string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleID, ok := UserRoleFromContext(r.Context())
			if !ok || roleID == "" {
				utils.WriteError(
					w,
					http.StatusUnauthorized,
					"unauthenticated",
					"authentication required",
				)
				return
			}

			for _, permission := range permissions {
				if permission == "" {
					continue
				}

				err := rbac.Authorize(
					r.Context(),
					roleID,
					permission,
				)
				if err != nil {
					if errors.Is(err, service.ErrForbidden) {
						utils.WriteError(
							w,
							http.StatusForbidden,
							"forbidden",
							"you do not have permission to perform this action",
						)
						return
					}

					utils.WriteError(
						w,
						http.StatusInternalServerError,
						"authorization_error",
						"failed to verify permissions",
					)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission requires the user to have at least ONE
// of the supplied permissions.
func RequireAnyPermission(
	rbac *service.RBACService,
	permissions ...string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleID, ok := UserRoleFromContext(r.Context())
			if !ok || roleID == "" {
				utils.WriteError(
					w,
					http.StatusUnauthorized,
					"unauthenticated",
					"authentication required",
				)
				return
			}

			for _, permission := range permissions {
				if permission == "" {
					continue
				}

				err := rbac.Authorize(
					r.Context(),
					roleID,
					permission,
				)

				if err == nil {
					next.ServeHTTP(w, r)
					return
				}

				if !errors.Is(err, service.ErrForbidden) {
					utils.WriteError(
						w,
						http.StatusInternalServerError,
						"authorization_error",
						"failed to verify permissions",
					)
					return
				}
			}

			utils.WriteError(
				w,
				http.StatusForbidden,
				"forbidden",
				"you do not have permission to perform this action",
			)
		})
	}
}
