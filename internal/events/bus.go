package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type Handler func(context.Context, []byte) error

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	queue    chan Message
}

type Message struct {
	ID      string
	Type    string
	Payload []byte
}

func NewBus(bufferSize int) *Bus {
	if bufferSize < 1 {
		bufferSize = 1000
	}

	return &Bus{
		handlers: make(map[string][]Handler),
		queue:    make(chan Message, bufferSize),
	}
}

func (b *Bus) Subscribe(
	eventType string,
	handler Handler,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(
		b.handlers[eventType],
		handler,
	)
}

func (b *Bus) Publish(
	ctx context.Context,
	message Message,
) error {
	select {
	case b.queue <- message:
		return nil

	case <-ctx.Done():
		return fmt.Errorf("publish event: %w", ctx.Err())
	}
}

func (b *Bus) Start(
	ctx context.Context,
	workers int,
) {
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			b.worker(ctx)
		}()
	}

	<-ctx.Done()

	wg.Wait()
}

func (b *Bus) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case message := <-b.queue:
			b.dispatch(ctx, message)
		}
	}
}

func (b *Bus) dispatch(
	ctx context.Context,
	message Message,
) {
	b.mu.RLock()

	handlers := append(
		[]Handler(nil),
		b.handlers[message.Type]...,
	)

	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, message.Payload); err != nil {
			slog.Error(
				"event handler failed",
				"event_id", message.ID,
				"event_type", message.Type,
				"error", err,
			)
		}
	}
}
