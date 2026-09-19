package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/example/goapi/internal/events"
	"github.com/example/goapi/internal/models"
	"github.com/google/uuid"
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

func (r *UserRepository) CreateWithOutbox(
	ctx context.Context,
	user *models.User,
	outbox *OutboxRepository,
) (*models.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin registration transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO users (
			email,
			password_hash,
			name,
			role
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			email,
			password_hash,
			name,
			role,
			created_at,
			updated_at
	`

	created := &models.User{}

	err = tx.QueryRowContext(
		ctx,
		query,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.Role,
	).Scan(
		&created.ID,
		&created.Email,
		&created.PasswordHash,
		&created.Name,
		&created.Role,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}

		return nil, fmt.Errorf("insert user: %w", err)
	}

	event := events.WalletCreationRequested{
		EventID:    uuid.NewString(),
		CustomerID: created.ID,
		CreatedBy:  "system",
		OccurredAt: time.Now().UTC(),
	}

	payload, err := event.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal wallet event: %w", err)
	}

	if err := outbox.Create(
		ctx,
		tx,
		events.WalletCreationRequestedName,
		"user",
		created.ID,
		payload,
	); err != nil {
		return nil, fmt.Errorf("create wallet outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit registration: %w", err)
	}

	return created, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error

	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}

	return false
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
