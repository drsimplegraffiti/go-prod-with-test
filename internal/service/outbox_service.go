package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/example/goapi/internal/events"
	"github.com/example/goapi/internal/repository"
)

type OutboxService struct {
	repository *repository.OutboxRepository
	bus        *events.Bus
}

func NewOutboxService(
	repository *repository.OutboxRepository,
	bus *events.Bus,
) *OutboxService {
	return &OutboxService{
		repository: repository,
		bus:        bus,
	}
}

func (s *OutboxService) Run(
	ctx context.Context,
) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.publishBatch(ctx)
		}
	}
}

func (s *OutboxService) publishBatch(ctx context.Context) {
	items, err := s.repository.ClaimBatch(
		ctx,
		100,
	)
	if err != nil {
		slog.Error(
			"outbox claim failed",
			"error", err,
		)
		return
	}

	for _, item := range items {
		err := s.bus.Publish(
			ctx,
			events.Message{
				ID:      item.ID,
				Type:    item.EventType,
				Payload: item.Payload,
			},
		)
		if err != nil {
			_ = s.repository.MarkFailed(
				ctx,
				item.ID,
				err.Error(),
			)

			continue
		}

		if err := s.repository.MarkPublished(
			ctx,
			item.ID,
		); err != nil {
			slog.Error(
				"failed to mark outbox event published",
				"event_id", item.ID,
				"error", err,
			)
		}
	}
}
