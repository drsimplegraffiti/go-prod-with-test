package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
)

type IdempotencyService struct {
	repository *repository.IdempotencyRepository
}

func NewIdempotencyService(
	repository *repository.IdempotencyRepository,
) *IdempotencyService {
	return &IdempotencyService{
		repository: repository,
	}
}

func HashRequest(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func (s *IdempotencyService) Get(
	ctx context.Context,
	key string,
	userID string,
	endpoint string,
) (*models.IdempotencyKey, error) {
	return s.repository.Get(
		ctx,
		key,
		userID,
		endpoint,
	)
}

func (s *IdempotencyService) Claim(
	ctx context.Context,
	key string,
	userID string,
	endpoint string,
	requestHash string,
) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("idempotency key is required")
	}

	if len(key) > 255 {
		return false, fmt.Errorf("idempotency key is too long")
	}

	return s.repository.Create(
		ctx,
		key,
		userID,
		endpoint,
		requestHash,
	)
}

func (s *IdempotencyService) Complete(
	ctx context.Context,
	key string,
	userID string,
	endpoint string,
	status int,
	responseBody []byte,
) error {
	return s.repository.Complete(
		ctx,
		key,
		userID,
		endpoint,
		status,
		responseBody,
	)
}
