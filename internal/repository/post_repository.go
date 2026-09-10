package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/example/goapi/internal/models"
)

// PostRepository provides persistence operations for Post.
type PostRepository struct {
	db *sql.DB
}

// NewPostRepository constructs a PostRepository.
func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

// Create inserts a new post owned by userID.
func (r *PostRepository) Create(ctx context.Context, p *models.Post) (*models.Post, error) {
	const query = `
		INSERT INTO posts (user_id, title, content, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, title, content, status, created_at, updated_at
	`
	out := &models.Post{}
	err := r.db.QueryRowContext(ctx, query, p.UserID, p.Title, p.Content, p.Status).Scan(
		&out.ID, &out.UserID, &out.Title, &out.Content, &out.Status, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert post: %w", err)
	}
	return out, nil
}

// GetByID fetches a single post by primary key.
func (r *PostRepository) GetByID(ctx context.Context, id string) (*models.Post, error) {
	const query = `
		SELECT id, user_id, title, content, status, created_at, updated_at
		FROM posts WHERE id = $1
	`
	p := &models.Post{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Content, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan post: %w", err)
	}
	return p, nil
}

// sortColumn is the allow-list of columns that may be interpolated into the
// ORDER BY clause. Only values already validated by utils.ParseSort ever
// reach here, but this second map keeps the repository safe even if a
// caller forgets that step.
var sortColumn = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"title":      "title",
}

// List returns a page of posts matching the given filters, along with the
// total count of matching rows (ignoring pagination) for building pagination
// metadata.
func (r *PostRepository) List(ctx context.Context, params models.PostListParams) ([]models.Post, int64, error) {
	var (
		conditions []string
		args       []interface{}
		argPos     = 1
	)

	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argPos))
		args = append(args, params.Status)
		argPos++
	}
	if params.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
		args = append(args, params.UserID)
		argPos++
	}
	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf("title ILIKE $%d", argPos))
		args = append(args, "%"+params.Search+"%")
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matches first (same filters, no pagination).
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM posts %s`, whereClause)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	col, ok := sortColumn[params.SortBy]
	if !ok {
		col = "created_at"
	}
	dir := "DESC"
	if strings.EqualFold(params.SortDir, "asc") {
		dir = "ASC"
	}

	limitArg := argPos
	offsetArg := argPos + 1
	args = append(args, params.PageSize, (params.Page-1)*params.PageSize)

	// col and dir are interpolated but both are drawn exclusively from the
	// fixed allow-lists above, never from raw user input, so this remains
	// injection-safe despite not being a bind parameter.
	listQuery := fmt.Sprintf(`
		SELECT id, user_id, title, content, status, created_at, updated_at
		FROM posts
		%s
		ORDER BY %s %s, id ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, col, dir, limitArg, offsetArg)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	posts := make([]models.Post, 0, params.PageSize)
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Content, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan post row: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate post rows: %w", err)
	}

	return posts, total, nil
}

// Update applies a partial update to a post owned by userID and returns the
// refreshed record. Returns ErrNotFound if no such post exists for that user.
func (r *PostRepository) Update(ctx context.Context, id, userID string, req models.UpdatePostRequest) (*models.Post, error) {
	var (
		sets   []string
		args   []interface{}
		argPos = 1
	)

	if req.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", argPos))
		args = append(args, *req.Title)
		argPos++
	}
	if req.Content != nil {
		sets = append(sets, fmt.Sprintf("content = $%d", argPos))
		args = append(args, *req.Content)
		argPos++
	}
	if req.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argPos))
		args = append(args, *req.Status)
		argPos++
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}
	sets = append(sets, "updated_at = now()")

	idArg := argPos
	userArg := argPos + 1
	args = append(args, id, userID)

	query := fmt.Sprintf(`
		UPDATE posts SET %s
		WHERE id = $%d AND user_id = $%d
		RETURNING id, user_id, title, content, status, created_at, updated_at
	`, strings.Join(sets, ", "), idArg, userArg)

	p := &models.Post{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Content, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update post: %w", err)
	}
	return p, nil
}

// Delete removes a post owned by userID. Returns ErrNotFound if no rows matched.
func (r *PostRepository) Delete(ctx context.Context, id, userID string) error {
	const query = `DELETE FROM posts WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
