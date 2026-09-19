// package router
//
// import (
// 	"net/http"
//
// 	"github.com/example/goapi/internal/handlers"
// )
//
// // registerPostRoutes wires up the post endpoints — public reads, and writes
// // guarded by authMW.
// func registerPostRoutes(
// 	mux *http.ServeMux,
// 	postHandler *handlers.PostHandler,
// 	authMW func(http.Handler) http.Handler,
// ) {
//
//
// 	// Public reads
// 	mux.HandleFunc("GET /api/v1/posts", postHandler.List)
// 	mux.HandleFunc("GET /api/v1/posts/{id}", postHandler.Get)
//
// 	// Authenticated writes
// 	mux.Handle("POST /api/v1/posts", authMW(http.HandlerFunc(postHandler.Create)))
// 	mux.Handle("PATCH /api/v1/posts/{id}", authMW(http.HandlerFunc(postHandler.Update)))
// 	mux.Handle("DELETE /api/v1/posts/{id}", authMW(http.HandlerFunc(postHandler.Delete)))
// }

package router

import (
	"net/http"

	"github.com/example/goapi/internal/handlers"
)

// registerPostRoutes wires up the post endpoints — public reads, and writes
// guarded by authMW.
func registerPostRoutes(
	mux *http.ServeMux,
	postHandler *handlers.PostHandler,
	authMW func(http.Handler) http.Handler,
) {
	// Public reads
	// mux.HandleFunc("GET /api/v1/posts", postHandler.List)
	// mux.HandleFunc("GET /api/v1/posts/{id}", postHandler.Get)

	url := "/api/v1/posts"
	public := NewGroup(mux, url)

	public.HandleFunc("GET /", postHandler.List)
	public.HandleFunc("GET /{id}", postHandler.Get)

	// Authenticated writes
	protected := NewGroup(
		mux,
		url,
		authMW,
	)

	protected.HandleFunc("POST /", postHandler.Create)
	protected.HandleFunc("PATCH /{id}", postHandler.Update)
	protected.HandleFunc("DELETE /{id}", postHandler.Delete)
}
