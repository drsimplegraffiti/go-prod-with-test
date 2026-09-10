package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/example/goapi/internal/models"
	"github.com/lib/pq"
)

// ErrNotFound is returned when a lookup finds no matching row.
var ErrNotFound = errors.New("resource not found")

// ErrDuplicate is returned when a unique constraint (e.g. email) is violated.
var ErrDuplicate = errors.New("resource already exists")

// UserRepository provides persistence operations for User.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository constructs a UserRepository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user and returns the fully populated record.
func (r *UserRepository) Create(ctx context.Context, u *models.User) (*models.User, error) {
	const query = `
		INSERT INTO users (email, password_hash, name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, name, role, created_at, updated_at
	`
	out := &models.User{}
	err := r.db.QueryRowContext(ctx, query, u.Email, u.PasswordHash, u.Name, u.Role).Scan(
		&out.ID, &out.Email, &out.PasswordHash, &out.Name, &out.Role, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return out, nil
}

// GetByEmail looks up a user by their unique email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	const query = `
		SELECT id, email, password_hash, name, role, created_at, updated_at
		FROM users WHERE email = $1
	`
	return r.scanOne(r.db.QueryRowContext(ctx, query, email))
}

// GetByID looks up a user by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	const query = `
		SELECT id, email, password_hash, name, role, created_at, updated_at
		FROM users WHERE id = $1
	`
	return r.scanOne(r.db.QueryRowContext(ctx, query, id))
}

func (r *UserRepository) scanOne(row *sql.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}
