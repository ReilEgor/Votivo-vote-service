// Package outbox moves the messages written next to the data into the broker.
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/repository"
	"github.com/ReilEgor/Votivo-vote-service/pkg/ctxlog"
)

const (
	batchSize   = 100
	maxAttempts = 5
)

// Publisher sends a message to the exchange and returns once the broker has accepted it.
type Publisher interface {
	Publish(ctx context.Context, routingKey, messageID string, body []byte) error
}

// Relay periodically publishes the pending outbox messages. Delivery is at least once: a crash between the
// publish and the delete sends the message again, which is why every consumer must be idempotent.
type Relay struct {
	outbox    repository.OutboxRepository
	tx        repository.Transactor
	publisher Publisher
	interval  time.Duration
}

// NewRelay returns a Relay that looks for pending messages every interval.
func NewRelay(
	outbox repository.OutboxRepository,
	tx repository.Transactor,
	publisher Publisher,
	interval time.Duration,
) *Relay {
	return &Relay{outbox: outbox, tx: tx, publisher: publisher, interval: interval}
}

// Run publishes until ctx is canceled.
func (r *Relay) Run(ctx context.Context) error {
	ctx = ctxlog.WithInitiator(ctx, ctxlog.InitiatorSystem)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.process(ctx)
		}
	}
}

// process publishes one batch. The rows stay locked until the transaction ends, so a second relay skips them.
func (r *Relay) process(ctx context.Context) {
	const op = "Relay.process"

	err := r.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		msgs, err := r.outbox.FetchPending(txCtx, batchSize)
		if err != nil {
			return fmt.Errorf("fetch pending: %w", err)
		}
		r.sendAll(txCtx, msgs)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		slog.ErrorContext(ctx, "outbox batch failed", slog.String("op", op), slog.String("error", err.Error()))
	}
}

// sendAll stops at the first sign of shutdown; what is already sent is committed, the rest stays pending.
func (r *Relay) sendAll(ctx context.Context, msgs []model.OutboxMessage) {
	for _, msg := range msgs {
		if ctx.Err() != nil {
			return
		}
		r.send(ctx, msg)
	}
}

func (r *Relay) send(ctx context.Context, msg model.OutboxMessage) {
	const op = "Relay.send"

	if err := r.publisher.Publish(ctx, msg.RoutingKey, msg.EventID, msg.Payload); err != nil {
		slog.ErrorContext(ctx, "outbox publish failed",
			slog.String("op", op), slog.Int64("id", msg.ID), slog.String("routing_key", msg.RoutingKey),
			slog.String("error", err.Error()))
		if failErr := r.outbox.Fail(ctx, msg.ID, maxAttempts, err.Error()); failErr != nil {
			slog.ErrorContext(ctx, "outbox mark failed",
				slog.String("op", op), slog.Int64("id", msg.ID), slog.String("error", failErr.Error()))
		}
		return
	}
	if err := r.outbox.Delete(ctx, msg.ID); err != nil {
		slog.ErrorContext(ctx, "outbox delete after publish failed",
			slog.String("op", op), slog.Int64("id", msg.ID), slog.String("error", err.Error()))
	}
}
