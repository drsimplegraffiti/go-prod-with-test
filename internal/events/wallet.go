package events

import (
	"encoding/json"
	"time"
)

const WalletCreationRequestedName = "wallet.creation_requested"

type WalletCreationRequested struct {
	EventID    string    `json:"event_id"`
	CustomerID string    `json:"customer_id"`
	CreatedBy  string    `json:"created_by"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (e WalletCreationRequested) Name() string {
	return WalletCreationRequestedName
}

func (e WalletCreationRequested) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
