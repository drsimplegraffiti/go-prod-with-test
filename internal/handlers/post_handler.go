package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

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
	posts       *service.PostService
	idempotency *service.IdempotencyService
}

// NewPostHandler constructs a PostHandler.
func NewPostHandler(
	posts *service.PostService,
	idempotency *service.IdempotencyService,
) *PostHandler {
	return &PostHandler{
		posts:       posts,
		idempotency: idempotency,
	}
}

// Create handles POST /api/v1/posts (auth required).
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// ------------------------------------------------------------
	// 1. Require Idempotency-Key
	// ------------------------------------------------------------

	idempotencyKey := strings.TrimSpace(
		r.Header.Get("Idempotency-Key"),
	)

	if idempotencyKey == "" {
		utils.WriteError(
			w,
			http.StatusBadRequest,
			"missing_idempotency_key",
			"Idempotency-Key header is required",
		)
		return
	}

	if len(idempotencyKey) > 255 {
		utils.WriteError(
			w,
			http.StatusBadRequest,
			"invalid_idempotency_key",
			"Idempotency-Key must not exceed 255 characters",
		)
		return
	}

	// ------------------------------------------------------------
	// 2. Read request body
	// ------------------------------------------------------------

	var req models.CreatePostRequest

	if !utils.ReadJSON(w, r, &req) {
		return
	}

	// ------------------------------------------------------------
	// 3. Hash the request
	//
	// We hash the decoded JSON structure rather than the raw HTTP
	// body so harmless whitespace differences don't produce
	// different hashes.
	// ------------------------------------------------------------

	requestBody, err := json.Marshal(req)
	if err != nil {
		utils.WriteError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"failed to prepare idempotency request",
		)
		return
	}

	requestHash := service.HashRequest(requestBody)

	endpoint := r.Method + " " + r.URL.Path

	// ------------------------------------------------------------
	// 4. Try to claim the idempotency key
	// ------------------------------------------------------------

	claimed, err := h.idempotency.Claim(
		r.Context(),
		idempotencyKey,
		userID,
		endpoint,
		requestHash,
	)
	if err != nil {
		utils.WriteError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			err.Error(),
		)
		return
	}

	// ------------------------------------------------------------
	// 5. Key already exists
	// ------------------------------------------------------------

	if !claimed {
		existing, err := h.idempotency.Get(
			r.Context(),
			idempotencyKey,
			userID,
			endpoint,
		)
		if err != nil {
			utils.WriteError(
				w,
				http.StatusInternalServerError,
				"internal_error",
				err.Error(),
			)
			return
		}

		// Same idempotency key but different request body.
		if existing.RequestHash != requestHash {
			utils.WriteError(
				w,
				http.StatusUnprocessableEntity,
				"idempotency_key_reused",
				"Idempotency-Key was already used with a different request",
			)
			return
		}

		// The original request hasn't completed yet.
		if existing.Status == models.IdempotencyProcessing {
			utils.WriteError(
				w,
				http.StatusConflict,
				"request_in_progress",
				"a request with this Idempotency-Key is already being processed",
			)
			return
		}

		// Original request completed.
		if existing.Status == models.IdempotencyCompleted {
			if existing.ResponseStatus == nil {
				utils.WriteError(
					w,
					http.StatusInternalServerError,
					"internal_error",
					"idempotency response status is missing",
				)
				return
			}

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			w.WriteHeader(*existing.ResponseStatus)

			_, _ = w.Write(existing.ResponseBody)

			return
		}

		utils.WriteError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"invalid idempotency state",
		)
		return
	}

	// ------------------------------------------------------------
	// 6. This is the first request with this key.
	// ------------------------------------------------------------

	post, err := h.posts.Create(
		r.Context(),
		userID,
		req,
	)
	if err != nil {
		writePostError(w, err)
		return
	}

	// ------------------------------------------------------------
	// 7. Build the exact response that we are going to return.
	// ------------------------------------------------------------

	responseBody, err := json.Marshal(post)
	if err != nil {
		utils.WriteError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"failed to prepare idempotency response",
		)
		return
	}

	// ------------------------------------------------------------
	// 8. Save successful response.
	// ------------------------------------------------------------
	if err := h.idempotency.Complete(
		r.Context(),
		idempotencyKey,
		userID,
		endpoint,
		http.StatusCreated,
		responseBody,
	); err != nil {
		utils.WriteError(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"post was created but failed to save idempotency response",
		)
		return
	}

	// ------------------------------------------------------------
	// 9. Return normal response.
	// ------------------------------------------------------------

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusCreated)

	_, _ = w.Write(responseBody)
}

// Create handles POST /api/v1/posts (auth required)
// func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
// 	userID, ok := requireUserID(w, r)
// 	if !ok {
// 		return
// 	}
//
// 	idempotencyKey := r.Header.Get("Idempotency-Key")
//
// 	if idempotencyKey == "" {
// 		utils.WriteError(
// 			w,
// 			http.StatusBadRequest,
// 			"missing_idempotency_key",
// 			"Idempotency-Key header is required",
// 		)
// 		return
// 	}
//
// 	var req models.CreatePostRequest
// 	if !utils.ReadJSON(w, r, &req) {
// 		return
// 	}
//
// 	post, err := h.posts.Create(r.Context(), userID, req)
// 	if err != nil {
// 		writePostError(w, err)
// 		return
// 	}
//
// 	utils.WriteJSON(w, http.StatusCreated, post)
// }

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
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	var req models.UpdatePostRequest
	if !utils.ReadJSON(w, r, &req) {
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
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	if err := h.posts.Delete(r.Context(), id, userID); err != nil {
		writePostError(w, err)
		return
	}

	// 204 No Content: no body is written at all.
	w.WriteHeader(http.StatusNoContent)
}

// writePostError maps post-service errors to HTTP responses.
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

// requireUserID extracts the authenticated user's ID from the request
// context. If the request is not authenticated it writes a 401 and returns
// false — the handler should simply `return` in that case:
//
//	userID, ok := requireUserID(w, r)
//	if !ok {
//	    return
//	}
//
// Note: middleware.Auth already rejects unauthenticated requests, so this
// guard is defense-in-depth for handlers that are (or might become)
// reachable without the middleware.
func requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		utils.WriteError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return "", false
	}
	return userID, true
}
