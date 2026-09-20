package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/example/goapi/internal/models"
)

var (
	ErrRoleNotFound       = errors.New("role not found")
	ErrPermissionNotFound = errors.New("permission not found")
)

type RBACRepository struct {
	db *sql.DB
}

func NewRBACRepository(db *sql.DB) *RBACRepository {
	return &RBACRepository{db: db}
}

func (r *RBACRepository) GetRoleByID(
	ctx context.Context,
	id string,
) (*models.Role, error) {
	const query = `
		SELECT
			id,
			name,
			COALESCE(description, ''),
			created_at,
			updated_at
		FROM roles
		WHERE id = $1
	`

	role := &models.Role{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}

		return nil, fmt.Errorf("get role: %w", err)
	}

	return role, nil
}

func (r *RBACRepository) GetRoleByName(
	ctx context.Context,
	name string,
) (*models.Role, error) {
	const query = `
		SELECT
			id,
			name,
			COALESCE(description, ''),
			created_at,
			updated_at
		FROM roles
		WHERE name = $1
	`

	role := &models.Role{}

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}

		return nil, fmt.Errorf("get role by name: %w", err)
	}

	return role, nil
}

func (r *RBACRepository) GetPermissions(
	ctx context.Context,
	roleID string,
) ([]models.Permission, error) {
	const query = `
		SELECT
			p.id,
			p.name,
			COALESCE(p.description, ''),
			p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp
			ON rp.permission_id = p.id
		WHERE rp.role_id = $1
		ORDER BY p.name
	`

	rows, err := r.db.QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("get role permissions: %w", err)
	}
	defer rows.Close()

	var permissions []models.Permission

	for rows.Next() {
		var p models.Permission

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}

		permissions = append(permissions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permissions: %w", err)
	}

	return permissions, nil
}

func (r *RBACRepository) HasPermission(
	ctx context.Context,
	roleID string,
	permission string,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM role_permissions rp
			INNER JOIN permissions p
				ON p.id = rp.permission_id
			WHERE rp.role_id = $1
			AND p.name = $2
		)
	`

	var exists bool

	if err := r.db.QueryRowContext(
		ctx,
		query,
		roleID,
		permission,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}

	return exists, nil
}

func (r *RBACRepository) AddPermission(
	ctx context.Context,
	roleID string,
	permissionID string,
) error {
	const query = `
		INSERT INTO role_permissions (
			role_id,
			permission_id
		)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		roleID,
		permissionID,
	); err != nil {
		return fmt.Errorf("add role permission: %w", err)
	}

	return nil
}

func (r *RBACRepository) RemovePermission(
	ctx context.Context,
	roleID string,
	permissionID string,
) error {
	const query = `
		DELETE FROM role_permissions
		WHERE role_id = $1
		AND permission_id = $2
	`

	if _, err := r.db.ExecContext(
		ctx,
		query,
		roleID,
		permissionID,
	); err != nil {
		return fmt.Errorf("remove role permission: %w", err)
	}

	return nil
}

func (r *RBACRepository) GetPermissionByID(
	ctx context.Context,
	id string,
) (*models.Permission, error) {
	const query = `
		SELECT
			id,
			name,
			COALESCE(description, ''),
			created_at
		FROM permissions
		WHERE id = $1
	`

	p := &models.Permission{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.Description,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPermissionNotFound
		}

		return nil, fmt.Errorf("get permission: %w", err)
	}

	return p, nil
}
