package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrUnroutable means the exchange has no queue bound for the routing key, so nobody would receive the message.
var ErrUnroutable = errors.New("message is unroutable")

// Publisher publishes persistent messages to a topic exchange and waits for the broker to confirm them.
type Publisher struct {
	conn     *Connection
	exchange string
}

// NewPublisher returns a Publisher on conn that publishes to exchange.
func NewPublisher(conn *Connection, exchange string) *Publisher {
	return &Publisher{conn: conn, exchange: exchange}
}

// Publish sends body with the routing key and returns once the broker has confirmed it. A message no queue
// is bound to is returned by the broker and reported as ErrUnroutable, so the outbox keeps it and retries
// instead of losing the event while the consumers are not declared yet.
func (p *Publisher) Publish(ctx context.Context, routingKey, messageID string, body []byte) error {
	const op = "Publisher.Publish"

	ch, err := p.conn.Channel(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer closeChannel(ctx, ch)

	if err = ch.Confirm(false); err != nil {
		return fmt.Errorf("%s: enable confirms: %w", op, err)
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))

	err = ch.PublishWithContext(ctx, p.exchange, routingKey, true, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    messageID,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("%s: publish: %w", op, err)
	}

	// The broker sends the return before the confirm of the same message.
	select {
	case confirm := <-confirms:
		select {
		case <-returns:
			return fmt.Errorf("%s: exchange %q, key %q: %w", op, p.exchange, routingKey, ErrUnroutable)
		default:
		}
		if !confirm.Ack {
			return fmt.Errorf("%s: broker nacked message for key %q", op, routingKey)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: wait for confirm: %w", op, ctx.Err())
	}
}
