package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes persistent messages to a queue and waits for the broker to confirm them.
type Publisher struct {
	conn *Connection
}

// NewPublisher returns a Publisher on conn.
func NewPublisher(conn *Connection) *Publisher {
	return &Publisher{conn: conn}
}

// Publish sends body to queue and returns once the broker has confirmed it.
func (p *Publisher) Publish(ctx context.Context, queue string, body []byte) error {
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

	err = ch.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("%s: publish: %w", op, err)
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return fmt.Errorf("%s: broker nacked message for queue %q", op, queue)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: wait for confirm: %w", op, ctx.Err())
	}
}
