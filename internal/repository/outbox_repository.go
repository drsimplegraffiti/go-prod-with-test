package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/example/goapi/internal/models"
)

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{
		db: db,
	}
}

func (r *OutboxRepository) Create(
	ctx context.Context,
	tx *sql.Tx,
	eventType string,
	aggregateType string,
	aggregateID string,
	payload []byte,
) error {
	const query = `
		INSERT INTO outbox_events (
			event_type,
			aggregate_type,
			aggregate_id,
			payload
		)
		VALUES ($1, $2, $3, $4::jsonb)
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		eventType,
		aggregateType,
		aggregateID,
		json.RawMessage(payload),
	)
	if err != nil {
		return fmt.Errorf("create outbox event: %w", err)
	}

	return nil
}

func (r *OutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
) ([]models.OutboxEvent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim transaction: %w", err)
	}

	defer tx.Rollback()

	const query = `
		SELECT
			id,
			event_type,
			aggregate_type,
			aggregate_id,
			payload,
			status,
			attempts,
			available_at,
			locked_at,
			published_at,
			last_error,
			created_at,
			updated_at
		FROM outbox_events
		WHERE
			(
				status = 'pending'
				AND available_at <= NOW()
			)
			OR
			(
				status = 'processing'
				AND locked_at < NOW() - INTERVAL '5 minutes'
			)
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}

	defer rows.Close()

	var events []models.OutboxEvent

	for rows.Next() {
		var event models.OutboxEvent

		if err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.AggregateType,
			&event.AggregateID,
			&event.Payload,
			&event.Status,
			&event.Attempts,
			&event.AvailableAt,
			&event.LockedAt,
			&event.PublishedAt,
			&event.LastError,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	for _, event := range events {
		_, err := tx.ExecContext(
			ctx,
			`
			UPDATE outbox_events
			SET
				status = 'processing',
				locked_at = NOW(),
				attempts = attempts + 1,
				updated_at = NOW()
			WHERE id = $1
			`,
			event.ID,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"mark outbox event processing: %w",
				err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}

	return events, nil
}

func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	eventID string,
) error {
	const query = `
		UPDATE outbox_events
		SET
			status = 'published',
			published_at = NOW(),
			locked_at = NULL,
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}

	return nil
}

func (r *OutboxRepository) MarkFailed(
	ctx context.Context,
	eventID string,
	errMessage string,
) error {
	const query = `
		UPDATE outbox_events
		SET
			status = CASE
				WHEN attempts >= 10 THEN 'failed'
				ELSE 'pending'
			END,
			available_at = CASE
				WHEN attempts >= 10 THEN available_at
				ELSE NOW() + (
					LEAST(
						POWER(2, attempts),
						300
					) * INTERVAL '1 second'
				)
			END,
			locked_at = NULL,
			last_error = $2,
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		eventID,
		errMessage,
	)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}

	return nil
}
