package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/repository"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/service"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/broker/rabbitmq"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/clients/poll"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/outbox"
	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/probe"
	sharedPostgres "github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/storage/postgres"
	votehttp "github.com/ReilEgor/Votivo-vote-service/internal/transport/http"
	"github.com/ReilEgor/Votivo-vote-service/pkg/contracts"
)

// App is the assembled service.
type App struct {
	HTTP  http.Handler
	Relay *outbox.Relay
}

func newApp(router *gin.Engine, relay *outbox.Relay) *App {
	return &App{HTTP: router, Relay: relay}
}

// providePool opens the connection pool and checks the database; a service whose database is down does not start.
func providePool(ctx context.Context, cfg Config) (*pgxpool.Pool, func(), error) {
	pool, err := sharedPostgres.NewPool(ctx, cfg.DBURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	return pool, pool.Close, nil
}

// provideBroker connects to RabbitMQ and declares the queues the outbox publishes to.
func provideBroker(ctx context.Context, cfg Config) (*rabbitmq.Connection, func(), error) {
	conn, closeConn, err := rabbitmq.NewConnection(ctx, cfg.RabbitMQURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to broker: %w", err)
	}
	if err = conn.DeclareQueues(ctx, contracts.QueueVoteCast, contracts.QueueVoteRevoked); err != nil {
		closeConn()
		return nil, nil, fmt.Errorf("declare queues: %w", err)
	}
	return conn, closeConn, nil
}

// providePolls returns the gRPC client of Poll Service, or the static list while its address is not set.
func providePolls(cfg Config) (service.PollProvider, func(), error) {
	if cfg.PollServiceAddr == "" {
		slog.Warn("POLL_SERVICE_ADDR is not set, answering from the static list of polls")
		return poll.NewStaticClient(), func() {}, nil
	}
	conn, err := grpc.NewClient(cfg.PollServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("create poll service client: %w", err)
	}
	return poll.NewGRPCClient(conn), func() {
		if closeErr := conn.Close(); closeErr != nil {
			slog.Warn("close poll service client", slog.String("error", closeErr.Error()))
		}
	}, nil
}

func provideRouter(h *votehttp.Handler) (*gin.Engine, error) {
	router, err := votehttp.NewRouter(h, probe.NewHandler(commit))
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
