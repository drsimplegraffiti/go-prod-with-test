package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/example/goapi/internal/models"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrWalletAlreadyExists = errors.New("wallet already exists")
)

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{
		db: db,
	}
}

func (r *WalletRepository) GetByCustomerID(
	ctx context.Context,
	customerID string,
	currency string,
) (*models.Wallet, error) {
	const query = `
		SELECT
			id,
			customer_id,
			currency,
			balance::text,
			status,
			created_at,
			updated_at
		FROM wallets
		WHERE customer_id = $1
		  AND currency = $2
	`

	var wallet models.Wallet

	err := r.db.QueryRowContext(
		ctx,
		query,
		customerID,
		currency,
	).Scan(
		&wallet.ID,
		&wallet.CustomerID,
		&wallet.Currency,
		&wallet.Balance,
		&wallet.Status,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWalletNotFound
		}

		return nil, fmt.Errorf("get wallet: %w", err)
	}

	return &wallet, nil
}

func (r *WalletRepository) Create(
	ctx context.Context,
	customerID string,
	currency string,
) (*models.Wallet, error) {
	const query = `
		INSERT INTO wallets (
			customer_id,
			currency
		)
		VALUES ($1, $2)
		ON CONFLICT (customer_id, currency)
		DO NOTHING
		RETURNING
			id,
			customer_id,
			currency,
			balance::text,
			status,
			created_at,
			updated_at
	`

	var wallet models.Wallet

	err := r.db.QueryRowContext(
		ctx,
		query,
		customerID,
		currency,
	).Scan(
		&wallet.ID,
		&wallet.CustomerID,
		&wallet.Currency,
		&wallet.Balance,
		&wallet.Status,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return r.GetByCustomerID(
				ctx,
				customerID,
				currency,
			)
		}

		return nil, fmt.Errorf("create wallet: %w", err)
	}

	return &wallet, nil
}
