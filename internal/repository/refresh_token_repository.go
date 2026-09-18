package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RefreshTokenRecord represents one issued refresh token. Only the SHA-256
// hash of the token is ever stored — the raw token cannot be recovered from
// the database.
type RefreshTokenRecord struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// RefreshTokenRepository provides persistence for refresh tokens.
type RefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create stores a newly issued refresh token hash.
func (r *RefreshTokenRepository) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	const query = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`
	if _, err := r.db.ExecContext(ctx, query, userID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// FindValid returns the record for a token hash that exists, has not been
// revoked, and has not expired. Returns ErrNotFound otherwise.
func (r *RefreshTokenRepository) FindValid(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
	`
	rec := &RefreshTokenRecord{}
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&rec.ID, &rec.UserID, &rec.TokenHash, &rec.ExpiresAt, &rec.RevokedAt, &rec.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return rec, nil
}

// Revoke marks a token hash as no longer usable (rotation / logout).
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	const query = `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	if _, err := r.db.ExecContext(ctx, query, tokenHash); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllForUser invalidates every refresh token of a user. Call this on
// password change / account recovery.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	const query = `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`
	if _, err := r.db.ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return nil
}
