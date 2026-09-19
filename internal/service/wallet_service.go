package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/goapi/internal/models"
	"github.com/example/goapi/internal/repository"
)

type WalletService struct {
	wallets *repository.WalletRepository
}

func NewWalletService(
	wallets *repository.WalletRepository,
) *WalletService {
	return &WalletService{
		wallets: wallets,
	}
}

func (s *WalletService) CreateForCustomer(
	ctx context.Context,
	customerID string,
) (*models.Wallet, error) {
	if customerID == "" {
		return nil, errors.New("customer ID is required")
	}

	wallet, err := s.wallets.Create(
		ctx,
		customerID,
		"NGN",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create wallet for customer: %w",
			err,
		)
	}

	return wallet, nil
}
