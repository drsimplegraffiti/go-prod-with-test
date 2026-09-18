package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/example/goapi/internal/file"
	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
)

type ProfileService struct {
	profiles *repository.ProfileRepository
	files    *file.Service
}

func NewProfileService(
	profiles *repository.ProfileRepository,
	files *file.Service,
) *ProfileService {
	return &ProfileService{
		profiles: profiles,
		files:    files,
	}
}

func (s *ProfileService) GetProfile(
	ctx context.Context,
	userID string,
) (*models.Profile, error) {
	return s.profiles.GetByUserID(ctx, userID)
}

func (s *ProfileService) UpdateProfile(
	ctx context.Context,
	userID string,
	req models.UpdateProfileRequest,
) (*models.Profile, error) {
	profile, err := s.profiles.GetByUserID(ctx, userID)
	if err != nil {
		if err != repository.ErrNotFound {
			return nil, err
		}

		profile = &models.Profile{
			UserID: userID,
		}
	}

	profile.FirstName = strings.TrimSpace(req.FirstName)
	profile.LastName = strings.TrimSpace(req.LastName)
	profile.Phone = strings.TrimSpace(req.Phone)
	profile.Bio = strings.TrimSpace(req.Bio)

	if profile.ID == "" {
		return s.profiles.Create(ctx, profile)
	}

	return s.profiles.Update(ctx, profile)
}

func (s *ProfileService) UpdateAvatar(
	ctx context.Context,
	userID string,
	fileHeader *multipart.FileHeader,
) (*models.Profile, error) {
	profile, err := s.profiles.GetByUserID(ctx, userID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}

		profile = &models.Profile{
			UserID: userID,
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("open avatar: %w", err)
	}
	defer file.Close()

	result, err := s.files.Upload(
		ctx,
		file,
		fileHeader,
		fmt.Sprintf("profiles/%s", userID),
	)
	if err != nil {
		return nil, fmt.Errorf("upload avatar: %w", err)
	}

	profile.AvatarURL = result.URL

	if profile.ID == "" {
		return s.profiles.Create(ctx, profile)
	}

	return s.profiles.Update(ctx, profile)
}
