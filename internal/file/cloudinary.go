package file

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type Service struct {
	cld *cloudinary.Cloudinary
}

type Config struct {
	CloudName string
	APIKey    string
	APISecret string
}

type UploadResult struct {
	URL      string
	PublicID string
	Format   string
}

func NewCloudinaryService(cfg Config) (*Service, error) {
	cld, err := cloudinary.NewFromParams(
		cfg.CloudName,
		cfg.APIKey,
		cfg.APISecret,
	)
	if err != nil {
		return nil, fmt.Errorf("create cloudinary client: %w", err)
	}

	return &Service{cld: cld}, nil
}

func (s *Service) Upload(
	ctx context.Context,
	file multipart.File,
	header *multipart.FileHeader,
	folder string,
) (*UploadResult, error) {
	defer file.Close()

	result, err := s.cld.Upload.Upload(
		ctx,
		file,
		uploader.UploadParams{
			Folder: folder,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	return &UploadResult{
		URL:      result.SecureURL,
		PublicID: result.PublicID,
		Format:   filepath.Ext(header.Filename),
	}, nil
}

func (s *Service) UploadMultiple(
	ctx context.Context,
	files []*multipart.FileHeader,
	folder string,
) ([]*UploadResult, error) {
	results := make([]*UploadResult, 0, len(files))

	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("open file %s: %w", header.Filename, err)
		}

		result, err := s.Upload(ctx, file, header, folder)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

func (s *Service) Delete(ctx context.Context, publicID string) error {
	_, err := s.cld.Upload.Destroy(
		ctx,
		uploader.DestroyParams{
			PublicID: publicID,
		},
	)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}
