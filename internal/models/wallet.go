package models

import "time"

type Wallet struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Currency   string    `json:"currency"`
	Balance    string    `json:"balance"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
