package models

import "time"

const (
	OutboxPending    = "pending"
	OutboxProcessing = "processing"
	OutboxPublished  = "published"
	OutboxFailed     = "failed"
)

type OutboxEvent struct {
	ID            string
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       []byte
	Status        string
	Attempts      int
	AvailableAt   time.Time
	LockedAt      *time.Time
	PublishedAt   *time.Time
	LastError     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
