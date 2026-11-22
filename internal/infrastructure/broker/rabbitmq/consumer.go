package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

const (
	prefetch     = 16
	retryPause   = time.Second
	deadLetterEx = "x-dead-letter-exchange"
	deadLetterRK = "x-dead-letter-routing-key"
)

// Handler processes one message. A nil result acknowledges it, model.ErrInvalidEvent dead-letters it
// (no retry can fix such a message), and any other error returns it to the queue after a pause.
type Handler func(ctx context.Context, routingKey string, body []byte) error

// ConsumerConfig describes the queue a Consumer reads and what feeds it.
type ConsumerConfig struct {
	Queue string
	// Exchange is the topic exchange the queue is bound to, with RoutingKeys.
	Exchange    string
	RoutingKeys []string
	// DeadLetterExchange receives the messages the handler rejects; its queue is Queue + DeadLetterSuffix.
	DeadLetterExchange string
	DeadLetterSuffix   string
}

// Consume declares the queue with its bindings and the dead-letter queue, then handles messages until ctx ends.
// The consumer declares everything it needs, so the start order of the services does not matter. When the connection
// drops, it reconnects and resumes.
func (c *Connection) Consume(ctx context.Context, cfg ConsumerConfig, handle Handler) error {
	backoff := initialBackoff
	for ctx.Err() == nil {
		err := c.consumeOnce(ctx, cfg, handle)
		if ctx.Err() != nil {
			return nil //nolint:nilerr // shutdown is not a failure: the consumer was asked to stop
		}
		slog.WarnContext(ctx, "rabbitmq consumer stopped, restarting",
			slog.String("queue", cfg.Queue), slog.String("error", err.Error()), slog.Duration("backoff", backoff))
		if !sleep(ctx, backoff) {
			return nil
		}
		backoff = min(backoff*2, maxBackoff)
	}
	return nil
}

func (c *Connection) consumeOnce(ctx context.Context, cfg ConsumerConfig, handle Handler) error {
	const op = "Connection.consumeOnce"

	ch, err := c.Channel(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer closeChannel(ctx, ch)

	if err = declareTopology(ch, cfg); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err = ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("%s: qos: %w", op, err)
	}
	deliveries, err := ch.ConsumeWithContext(ctx, cfg.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("%s: consume %q: %w", op, cfg.Queue, err)
	}
	slog.InfoContext(ctx, "rabbitmq consumer started", slog.String("queue", cfg.Queue))

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			process(ctx, d, handle)
		}
	}
}

// process runs the handler and settles the delivery.
func process(ctx context.Context, d amqp.Delivery, handle Handler) {
	err := handle(ctx, d.RoutingKey, d.Body)
	switch {
	case err == nil:
		settle(ctx, d, d.Ack(false))
	case errors.Is(err, model.ErrInvalidEvent):
		slog.ErrorContext(
			ctx,
			"message dead-lettered",
			slog.String(
				"routing_key",
				d.RoutingKey,
			),
			slog.String("message_id", d.MessageId),
			slog.String("error", err.Error()),
		)
		settle(ctx, d, d.Nack(false, false))
	default:
		slog.WarnContext(
			ctx,
			"message failed, will be redelivered",
			slog.String(
				"routing_key",
				d.RoutingKey,
			),
			slog.String("message_id", d.MessageId),
			slog.String("error", err.Error()),
		)
		sleep(ctx, retryPause)
		settle(ctx, d, d.Nack(false, true))
	}
}

func settle(ctx context.Context, d amqp.Delivery, err error) {
	if err != nil {
		slog.WarnContext(
			ctx,
			"settle message",
			slog.String("message_id", d.MessageId),
			slog.String("error", err.Error()),
		)
	}
}

func declareTopology(ch *amqp.Channel, cfg ConsumerConfig) error {
	if err := ch.ExchangeDeclare(cfg.Exchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %q: %w", cfg.Exchange, err)
	}
	if err := ch.ExchangeDeclare(
		cfg.DeadLetterExchange,
		amqp.ExchangeDirect,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange %q: %w", cfg.DeadLetterExchange, err)
	}

	dlq := cfg.Queue + cfg.DeadLetterSuffix
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %q: %w", dlq, err)
	}
	if err := ch.QueueBind(dlq, cfg.Queue, cfg.DeadLetterExchange, false, nil); err != nil {
		return fmt.Errorf("bind queue %q: %w", dlq, err)
	}

	args := amqp.Table{deadLetterEx: cfg.DeadLetterExchange, deadLetterRK: cfg.Queue}
	if _, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare queue %q: %w", cfg.Queue, err)
	}
	for _, key := range cfg.RoutingKeys {
		if err := ch.QueueBind(cfg.Queue, key, cfg.Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %q to %q: %w", cfg.Queue, key, err)
		}
	}
	return nil
}

// sleep waits for d and reports false when ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
