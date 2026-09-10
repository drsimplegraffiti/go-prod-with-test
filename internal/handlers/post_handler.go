package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/example/goapi/internal/middleware"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/service"
	"github.com/example/goapi/internal/utils"
)

// allowedSortFields is the public allow-list of columns clients may sort
// posts by. Keeping this in the handler (rather than trusting raw input all
// the way to SQL) means the same list is easy to document in the API spec.
var allowedSortFields = []string{"created_at", "updated_at", "title"}

// PostHandler exposes HTTP endpoints for post CRUD and listing.
type PostHandler struct {
	posts *service.PostService
}

// NewPostHandler constructs a PostHandler.
func NewPostHandler(posts *service.PostService) *PostHandler {
	return &PostHandler{posts: posts}
}

// Create handles POST /api/v1/posts (auth required)
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	var req models.CreatePostRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+jsonErrDetail(err))
		return
	}

	post, err := h.posts.Create(r.Context(), userID, req)
	if err != nil {
		writePostError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, post)
}

// List handles GET /api/v1/posts?page=&page_size=&status=&user_id=&search=&sort_by=&sort_dir=
func (h *PostHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	sortBy, sortDir := utils.ParseSort(q, allowedSortFields, "created_at")

	params := models.PostListParams{
		Page:     utils.ParsePage(q),
		PageSize: utils.ParsePageSize(q),
		Status:   models.PostStatus(q.Get("status")),
		UserID:   q.Get("user_id"),
		Search:   q.Get("search"),
		SortBy:   sortBy,
		SortDir:  sortDir,
	}

	result, err := h.posts.List(r.Context(), params)
	if err != nil {
		writePostError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, result)
}

// Get handles GET /api/v1/posts/{id}
func (h *PostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	post, err := h.posts.Get(r.Context(), id)
	if err != nil {
		writePostError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, post)
}

// Update handles PATCH /api/v1/posts/{id} (auth required, owner only)
func (h *PostHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	id := r.PathValue("id")

	var req models.UpdatePostRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+jsonErrDetail(err))
		return
	}

	post, err := h.posts.Update(r.Context(), id, userID, req)
	if err != nil {
		writePostError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, post)
}

// Delete handles DELETE /api/v1/posts/{id} (auth required, owner only)
func (h *PostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	id := r.PathValue("id")

	if err := h.posts.Delete(r.Context(), id, userID); err != nil {
		writePostError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusNoContent, nil)
}

func writePostError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		utils.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, service.ErrNotFound):
		utils.WriteError(w, http.StatusNotFound, "not_found", "post not found")
	case errors.Is(err, service.ErrForbidden):
		utils.WriteError(w, http.StatusForbidden, "forbidden", "you do not own this post")
	default:
		slog.Error("post handler internal error", "error", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}
