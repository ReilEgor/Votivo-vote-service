package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/repository"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/broker/rabbitmq"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/outbox"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/probe"
	sharedPostgres "github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/storage/postgres"
	amqphandler "github.com/ReilEgor/Votivo-vote-service/internal/transport/amqp"
	votehttp "github.com/ReilEgor/Votivo-vote-service/internal/transport/http"
	"github.com/ReilEgor/Votivo-vote-service/pkg/contracts"
)

// App is the assembled service.
type App struct {
	HTTP http.Handler
	// Relay publishes the vote events, PollEvents consumes the poll events.
	Relay      *outbox.Relay
	PollEvents *PollEventConsumer
}

// PollEventConsumer reads the poll events from the broker until its context ends.
type PollEventConsumer struct {
	conn    *rabbitmq.Connection
	handler *amqphandler.Handler
}

// Run consumes the poll events until ctx is canceled.
func (c *PollEventConsumer) Run(ctx context.Context) error {
	err := c.conn.Consume(ctx, rabbitmq.ConsumerConfig{
		Queue:              contracts.QueuePollEvents,
		Exchange:           contracts.ExchangePolls,
		RoutingKeys:        []string{contracts.RoutingKeyPollStarted, contracts.RoutingKeyPollEnded},
		DeadLetterExchange: contracts.ExchangeDeadLetter,
		DeadLetterSuffix:   contracts.DeadLetterQueueSuffix,
	}, c.handler.Handle)
	if err != nil {
		return fmt.Errorf("consume %s: %w", contracts.QueuePollEvents, err)
	}
	return nil
}

func newPollEventConsumer(conn *rabbitmq.Connection, handler *amqphandler.Handler) *PollEventConsumer {
	return &PollEventConsumer{conn: conn, handler: handler}
}

func newApp(router *gin.Engine, relay *outbox.Relay, pollEvents *PollEventConsumer) *App {
	return &App{HTTP: router, Relay: relay, PollEvents: pollEvents}
}

// providePool opens the connection pool and checks the database; a service whose database is down does not start.
func providePool(ctx context.Context, cfg Config) (*pgxpool.Pool, func(), error) {
	pool, err := sharedPostgres.NewPool(ctx, cfg.DBURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	return pool, pool.Close, nil
}

// provideBroker connects to RabbitMQ and declares the exchange the outbox publishes to. The queues are declared
// by the consumers; the broker definitions of the integration repository create them up front as well.
func provideBroker(ctx context.Context, cfg Config) (*rabbitmq.Connection, func(), error) {
	conn, closeConn, err := rabbitmq.NewConnection(ctx, cfg.RabbitMQURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to broker: %w", err)
	}
	if err = conn.DeclareTopicExchange(ctx, contracts.ExchangeVotes); err != nil {
		closeConn()
		return nil, nil, fmt.Errorf("declare exchange: %w", err)
	}
	return conn, closeConn, nil
}

// providePublisher returns the publisher of the vote events.
func providePublisher(conn *rabbitmq.Connection) *rabbitmq.Publisher {
	return rabbitmq.NewPublisher(conn, contracts.ExchangeVotes)
}

func provideRouter(cfg Config, h *votehttp.Handler) (*gin.Engine, error) {
	router, err := votehttp.NewRouter(h, probe.NewHandler(commit), cfg.InternalAuthSecret)
	if err != nil {
		return nil, fmt.Errorf("create router: %w", err)
	}
	return router, nil
}

func provideRelay(
	cfg Config,
	outboxRepo repository.OutboxRepository,
	tx repository.Transactor,
	publisher *rabbitmq.Publisher,
) *outbox.Relay {
	return outbox.NewRelay(outboxRepo, tx, publisher, cfg.OutboxInterval)
}
