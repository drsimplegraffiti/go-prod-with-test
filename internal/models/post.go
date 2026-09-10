package models

import "time"

// PostStatus enumerates the lifecycle states of a post.
type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusPublished PostStatus = "published"
	StatusArchived  PostStatus = "archived"
)

// Post is the persisted representation of a blog-style post owned by a user.
type Post struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Status    PostStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CreatePostRequest is the payload for creating a post.
type CreatePostRequest struct {
	Title   string     `json:"title"`
	Content string     `json:"content"`
	Status  PostStatus `json:"status"`
}

// UpdatePostRequest is the payload for partially updating a post. Pointer
// fields distinguish "not provided" from "provided as zero value".
type UpdatePostRequest struct {
	Title   *string     `json:"title"`
	Content *string     `json:"content"`
	Status  *PostStatus `json:"status"`
}

// PostListParams captures validated pagination, filtering and sorting
// options for GET /posts.
type PostListParams struct {
	Page     int
	PageSize int
	Status   PostStatus // empty means "any"
	UserID   string     // empty means "any"
	Search   string     // matches title, case-insensitive substring
	SortBy   string     // one of: created_at, updated_at, title
	SortDir  string     // asc | desc
}

// PaginatedResponse is a generic envelope for list endpoints.
type PaginatedResponse[T any] struct {
	Data       []T   `json:"data"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}
