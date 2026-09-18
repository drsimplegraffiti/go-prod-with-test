package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/example/goapi/internal/models"
)

type ProfileRepository struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) GetByUserID(
	ctx context.Context,
	userID string,
) (*models.Profile, error) {
	const query = `
		SELECT
			id,
			user_id,
			first_name,
			last_name,
			phone,
			bio,
			avatar_url,
			created_at,
			updated_at
		FROM profiles
		WHERE user_id = $1
	`

	var profile models.Profile

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.FirstName,
		&profile.LastName,
		&profile.Phone,
		&profile.Bio,
		&profile.AvatarURL,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get profile: %w", err)
	}

	return &profile, nil
}

func (r *ProfileRepository) Create(
	ctx context.Context,
	profile *models.Profile,
) (*models.Profile, error) {
	const query = `
		INSERT INTO profiles (
			user_id,
			first_name,
			last_name,
			phone,
			bio,
			avatar_url
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id,
			user_id,
			first_name,
			last_name,
			phone,
			bio,
			avatar_url,
			created_at,
			updated_at
	`

	var created models.Profile

	err := r.db.QueryRowContext(
		ctx,
		query,
		profile.UserID,
		profile.FirstName,
		profile.LastName,
		profile.Phone,
		profile.Bio,
		profile.AvatarURL,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.FirstName,
		&created.LastName,
		&created.Phone,
		&created.Bio,
		&created.AvatarURL,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}

	return &created, nil
}

func (r *ProfileRepository) Update(
	ctx context.Context,
	profile *models.Profile,
) (*models.Profile, error) {
	const query = `
		UPDATE profiles
		SET
			first_name = $2,
			last_name = $3,
			phone = $4,
			bio = $5,
			avatar_url = $6,
			updated_at = NOW()
		WHERE user_id = $1
		RETURNING
			id,
			user_id,
			first_name,
			last_name,
			phone,
			bio,
			avatar_url,
			created_at,
			updated_at
	`

	var updated models.Profile

	err := r.db.QueryRowContext(
		ctx,
		query,
		profile.UserID,
		profile.FirstName,
		profile.LastName,
		profile.Phone,
		profile.Bio,
		profile.AvatarURL,
	).Scan(
		&updated.ID,
		&updated.UserID,
		&updated.FirstName,
		&updated.LastName,
		&updated.Phone,
		&updated.Bio,
		&updated.AvatarURL,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("update profile: %w", err)
	}

	return &updated, nil
}
