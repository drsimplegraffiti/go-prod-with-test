package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/example/goapi/internal/models"
)

// RequestLogRepository persists HTTP request/response audit records.
type RequestLogRepository struct {
	db *sql.DB
}

// NewRequestLogRepository constructs a RequestLogRepository.
func NewRequestLogRepository(db *sql.DB) *RequestLogRepository {
	return &RequestLogRepository{db: db}
}

// Create inserts a new request log entry and returns the fully populated
// record, including its generated id and created_at.
func (r *RequestLogRepository) Create(ctx context.Context, l *models.RequestLog) (*models.RequestLog, error) {
	const query = `
		INSERT INTO request_logs
			(request_id, method, path, status_code, request_body, response_body, remote_addr, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, request_id, method, path, status_code, request_body, response_body, remote_addr, duration_ms, created_at
	`
	out := &models.RequestLog{}
	err := r.db.QueryRowContext(ctx, query,
		l.RequestID, l.Method, l.Path, l.StatusCode, l.RequestBody, l.ResponseBody, l.RemoteAddr, l.DurationMS,
	).Scan(
		&out.ID, &out.RequestID, &out.Method, &out.Path, &out.StatusCode,
		&out.RequestBody, &out.ResponseBody, &out.RemoteAddr, &out.DurationMS, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert request log: %w", err)
	}
	return out, nil
}
