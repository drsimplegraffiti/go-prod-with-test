package models

import "time"

type IdempotencyKey struct {
	ID             string
	Key            string
	UserID         string
	Endpoint       string
	RequestHash    string
	Status         string
	ResponseStatus *int
	ResponseBody   []byte
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

const (
	IdempotencyProcessing = "processing"
	IdempotencyCompleted  = "completed"
)
