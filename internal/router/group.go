package router

import (
	"net/http"
	"slices"
)

// Middleware is standard net/http middleware.
type Middleware func(http.Handler) http.Handler

// Group provides a prefix + middleware grouping layer
// on top of the standard library http.ServeMux.
type Group struct {
	mux        *http.ServeMux
	prefix     string
	middleware []Middleware
}

// NewGroup creates a route group.
//
// Example:
//
//	auth := NewGroup(mux, "/api/v1/auth")
//	auth.HandleFunc("POST /register", handler)
//
// Registers:
//
//	POST /api/v1/auth/register
func NewGroup(
	mux *http.ServeMux,
	prefix string,
	middleware ...Middleware,
) *Group {
	return &Group{
		mux:        mux,
		prefix:     normalizePath(prefix),
		middleware: slices.Clone(middleware),
	}
}

// Use adds middleware to this group.
func (g *Group) Use(middleware ...Middleware) {
	g.middleware = append(g.middleware, middleware...)
}

// Group creates a nested group.
//
// Example:
//
//	api := NewGroup(mux, "/api/v1")
//	auth := api.Group("/auth")
//
//	auth.HandleFunc("POST /login", handler)
//
// Registers:
//
//	POST /api/v1/auth/login
func (g *Group) Group(prefix string, middleware ...Middleware) *Group {
	return &Group{
		mux:    g.mux,
		prefix: joinPath(g.prefix, prefix),
		middleware: append(
			slices.Clone(g.middleware),
			middleware...,
		),
	}
}

// Handle registers a pattern with the group's prefix
// and applies the group's middleware.
func (g *Group) Handle(
	pattern string,
	handler http.Handler,
) {
	g.mux.Handle(
		g.pattern(pattern),
		g.wrap(handler),
	)
}

// HandleFunc registers a handler function with the group.
func (g *Group) HandleFunc(
	pattern string,
	handler http.HandlerFunc,
) {
	g.Handle(pattern, handler)
}

// pattern combines the group prefix with a ServeMux pattern.
//
// Supports:
//
//	POST /register
//	GET /users/{id}
//	GET /health
func (g *Group) pattern(pattern string) string {
	method, path := splitPattern(pattern)

	path = joinPath(g.prefix, path)

	if method == "" {
		return path
	}

	return method + " " + path
}

// wrap applies middleware in the standard outer → inner order.
func (g *Group) wrap(handler http.Handler) http.Handler {
	for i := len(g.middleware) - 1; i >= 0; i-- {
		handler = g.middleware[i](handler)
	}

	return handler
}

func splitPattern(pattern string) (method, path string) {
	for _, m := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
		http.MethodOptions,
	} {
		prefix := m + " "

		if len(pattern) >= len(prefix) &&
			pattern[:len(prefix)] == prefix {
			return m, pattern[len(prefix):]
		}
	}

	return "", pattern
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}

	if path[0] != '/' {
		return "/" + path
	}

	return path
}

func joinPath(prefix, path string) string {
	prefix = normalizePath(prefix)
	path = normalizePath(path)

	if prefix == "/" {
		return path
	}

	if path == "/" {
		return prefix
	}

	return prefix + path
}
