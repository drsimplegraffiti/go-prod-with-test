package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/example/goapi/internal/events"
	"github.com/example/goapi/internal/service"
)

type WalletConsumer struct {
	wallets *service.WalletService
}

func NewWalletConsumer(
	wallets *service.WalletService,
) *WalletConsumer {
	return &WalletConsumer{
		wallets: wallets,
	}
}

func (c *WalletConsumer) Handle(
	ctx context.Context,
	payload []byte,
) error {
	var event events.WalletCreationRequested

	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf(
			"decode wallet creation event: %w",
			err,
		)
	}

	slog.Info(
		"processing wallet creation",
		"customer_id",
		event.CustomerID,
		"event_id",
		event.EventID,
	)

	_, err := c.wallets.CreateForCustomer(
		ctx,
		event.CustomerID,
	)
	if err != nil {
		return fmt.Errorf(
			"create wallet for customer %s: %w",
			event.CustomerID,
			err,
		)
	}

	return nil
}
