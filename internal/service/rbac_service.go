package service

import (
	"context"
	"fmt"

	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
)

// var ErrForbidden = errors.New("forbidden")

type RBACService struct {
	repo *repository.RBACRepository
}

func NewRBACService(repo *repository.RBACRepository) *RBACService {
	return &RBACService{repo: repo}
}

func (s *RBACService) Authorize(
	ctx context.Context,
	roleID string,
	permission string,
) error {
	ok, err := s.repo.HasPermission(
		ctx,
		roleID,
		permission,
	)
	if err != nil {
		return fmt.Errorf("authorize: %w", err)
	}

	if !ok {
		return ErrForbidden
	}

	return nil
}

func (s *RBACService) GetRole(
	ctx context.Context,
	roleID string,
) (*models.Role, error) {
	return s.repo.GetRoleByID(ctx, roleID)
}

func (s *RBACService) GetRolePermissions(
	ctx context.Context,
	roleID string,
) ([]models.Permission, error) {
	return s.repo.GetPermissions(ctx, roleID)
}

func (s *RBACService) AddPermission(
	ctx context.Context,
	roleID string,
	permissionID string,
) error {
	if _, err := s.repo.GetRoleByID(ctx, roleID); err != nil {
		return err
	}

	if _, err := s.repo.GetPermissionByID(ctx, permissionID); err != nil {
		return err
	}

	return s.repo.AddPermission(
		ctx,
		roleID,
		permissionID,
	)
}

func (s *RBACService) RemovePermission(
	ctx context.Context,
	roleID string,
	permissionID string,
) error {
	return s.repo.RemovePermission(
		ctx,
		roleID,
		permissionID,
	)
}
