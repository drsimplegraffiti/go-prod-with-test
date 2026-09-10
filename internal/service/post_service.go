package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
)

// PostService implements post CRUD and listing with ownership enforcement.
type PostService struct {
	posts *repository.PostRepository
}

// NewPostService constructs a PostService.
func NewPostService(posts *repository.PostRepository) *PostService {
	return &PostService{posts: posts}
}

// Create makes a new post owned by userID.
func (s *PostService) Create(ctx context.Context, userID string, req models.CreatePostRequest) (*models.Post, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if len(title) > 300 {
		return nil, fmt.Errorf("%w: title must be at most 300 characters", ErrValidation)
	}

	status := req.Status
	if status == "" {
		status = models.StatusDraft
	}
	if !isValidStatus(status) {
		return nil, fmt.Errorf("%w: invalid status %q", ErrValidation, status)
	}

	post := &models.Post{
		UserID:  userID,
		Title:   title,
		Content: req.Content,
		Status:  status,
	}

	created, err := s.posts.Create(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}
	return created, nil
}

// Get fetches a single post by ID.
func (s *PostService) Get(ctx context.Context, id string) (*models.Post, error) {
	post, err := s.posts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	return post, nil
}

// List returns a validated, paginated, filtered, sorted page of posts.
func (s *PostService) List(ctx context.Context, params models.PostListParams) (*models.PaginatedResponse[models.Post], error) {
	if params.Status != "" && !isValidStatus(params.Status) {
		return nil, fmt.Errorf("%w: invalid status filter %q", ErrValidation, params.Status)
	}

	posts, total, err := s.posts.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	return &models.PaginatedResponse[models.Post]{
		Data:       posts,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalItems: total,
		TotalPages: totalPages(total, params.PageSize),
	}, nil
}

// Update applies a partial update, enforcing that only the owning user may modify the post.
func (s *PostService) Update(ctx context.Context, id, userID string, req models.UpdatePostRequest) (*models.Post, error) {
	existing, err := s.posts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	if existing.UserID != userID {
		return nil, ErrForbidden
	}

	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrValidation)
		}
		req.Title = &trimmed
	}
	if req.Status != nil && !isValidStatus(*req.Status) {
		return nil, fmt.Errorf("%w: invalid status %q", ErrValidation, *req.Status)
	}

	updated, err := s.posts.Update(ctx, id, userID, req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update post: %w", err)
	}
	return updated, nil
}

// Delete removes a post, enforcing ownership.
func (s *PostService) Delete(ctx context.Context, id, userID string) error {
	existing, err := s.posts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get post: %w", err)
	}
	if existing.UserID != userID {
		return ErrForbidden
	}

	if err := s.posts.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete post: %w", err)
	}
	return nil
}

func isValidStatus(status models.PostStatus) bool {
	switch status {
	case models.StatusDraft, models.StatusPublished, models.StatusArchived:
		return true
	default:
		return false
	}
}

func totalPages(total int64, pageSize int) int {
	if pageSize <= 0 {
		return 1
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		return 1
	}
	return pages
}
