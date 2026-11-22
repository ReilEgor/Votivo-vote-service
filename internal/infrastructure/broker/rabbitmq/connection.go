// Package rabbitmq connects to RabbitMQ and publishes messages to it.
package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
	startupTimeout = 30 * time.Second
)

// Connection is an AMQP connection that reconnects when the broker drops it.
type Connection struct {
	url  string
	mu   sync.Mutex
	conn *amqp.Connection
}

// NewConnection dials the broker, retrying for startupTimeout because the broker may still be starting;
// a broker that stays unreachable is an error, so the service refuses to start.
// The returned function closes the connection.
func NewConnection(ctx context.Context, url string) (*Connection, func(), error) {
	const op = "rabbitmq.NewConnection"

	c := &Connection{url: url}
	ctx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err := c.dialWithRetry(ctx); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", op, err)
	}
	return c, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if err := c.conn.Close(); err != nil {
			slog.Warn("close rabbitmq connection", slog.String("error", err.Error()))
		}
	}, nil
}

// Channel opens a channel. A dropped connection is re-established with growing pauses until ctx ends.
func (c *Connection) Channel(ctx context.Context) (*amqp.Channel, error) {
	const op = "Connection.Channel"

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensure(ctx); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("%s: open channel: %w", op, err)
	}
	return ch, nil
}

// DeclareTopicExchange creates a durable topic exchange. Declaring an existing one with the same settings is a no-op.
func (c *Connection) DeclareTopicExchange(ctx context.Context, name string) error {
	const op = "Connection.DeclareTopicExchange"

	ch, err := c.Channel(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer closeChannel(ctx, ch)

	if err = ch.ExchangeDeclare(name, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("%s: declare %q: %w", op, name, err)
	}
	return nil
}

func (c *Connection) dialWithRetry(ctx context.Context) error {
	for {
		err := c.connect()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (last error: %s)", ctx.Err(), err.Error())
		case <-time.After(initialBackoff):
		}
	}
}

func (c *Connection) connect() error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Connection) ensure(ctx context.Context) error {
	backoff := initialBackoff
	for c.conn == nil || c.conn.IsClosed() {
		slog.WarnContext(ctx, "rabbitmq connection lost, reconnecting", slog.Duration("backoff", backoff))
		select {
		case <-ctx.Done():
			return fmt.Errorf("reconnect: %w", ctx.Err())
		case <-time.After(backoff):
		}
		if err := c.connect(); err == nil {
			slog.InfoContext(ctx, "rabbitmq reconnected")
			return nil
		}
		backoff = min(backoff*2, maxBackoff)
	}
	return nil
}

func closeChannel(ctx context.Context, ch *amqp.Channel) {
	if err := ch.Close(); err != nil {
		slog.WarnContext(ctx, "close rabbitmq channel", slog.String("error", err.Error()))
	}
}
