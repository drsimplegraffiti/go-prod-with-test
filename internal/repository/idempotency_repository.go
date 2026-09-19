package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/example/goapi/internal/models"
)

var ErrIdempotencyNotFound = errors.New("idempotency key not found")

type IdempotencyRepository struct {
	db *sql.DB
}

func NewIdempotencyRepository(db *sql.DB) *IdempotencyRepository {
	return &IdempotencyRepository{
		db: db,
	}
}

func (r *IdempotencyRepository) Create(
	ctx context.Context,
	key string,
	userID string,
	endpoint string,
	requestHash string,
) (bool, error) {
	const query = `
		INSERT INTO idempotency_keys (
			key,
			user_id,
			endpoint,
			request_hash,
			status
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key, user_id, endpoint)
		DO NOTHING
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		key,
		userID,
		endpoint,
		requestHash,
		models.IdempotencyProcessing,
	)
	if err != nil {
		return false, fmt.Errorf("create idempotency key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("get idempotency rows affected: %w", err)
	}

	return rows == 1, nil
}

func (r *IdempotencyRepository) Get(
	ctx context.Context,
	key string,
	userID string,
	endpoint string,
) (*models.IdempotencyKey, error) {
	const query = `
		SELECT
			id,
			key,
			user_id,
			endpoint,
			request_hash,
			status,
			response_status,
			response_body,
			created_at,
			completed_at
		FROM idempotency_keys
		WHERE key = $1
		  AND user_id = $2
		  AND endpoint = $3
	`

	var item models.IdempotencyKey

	err := r.db.QueryRowContext(
		ctx,
		query,
		key,
		userID,
		endpoint,
	).Scan(
		&item.ID,
		&item.Key,
		&item.UserID,
		&item.Endpoint,
		&item.RequestHash,
		&item.Status,
		&item.ResponseStatus,
		&item.ResponseBody,
		&item.CreatedAt,
		&item.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIdempotencyNotFound
		}

		return nil, fmt.Errorf("get idempotency key: %w", err)
	}

	return &item, nil
}

func (r *IdempotencyRepository) Complete(
	ctx context.Context,
	key string,
	userID string,
	endpoint string,
	status int,
	responseBody []byte,
) error {
	const query = `
		UPDATE idempotency_keys
		SET
			status = $5,
			response_status = $4,
			response_body = $6::jsonb,
			completed_at = NOW()
		WHERE key = $1
		  AND user_id = $2
		  AND endpoint = $3
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		key,
		userID,
		endpoint,
		status,
		models.IdempotencyCompleted,
		responseBody,
	)
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}

	return nil
}
