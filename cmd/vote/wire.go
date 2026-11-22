//go:build wireinject

package main

import (
	"context"

	"github.com/google/wire"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/repository"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/service"
	domainusecase "github.com/ReilEgor/Votivo-vote-service/internal/domain/usecase"
	sharedPostgres "github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/storage/postgres"
	"github.com/ReilEgor/Votivo-vote-service/internal/repository/postgres"
	amqphandler "github.com/ReilEgor/Votivo-vote-service/internal/transport/amqp"
	votehttp "github.com/ReilEgor/Votivo-vote-service/internal/transport/http"
	"github.com/ReilEgor/Votivo-vote-service/internal/usecase"
)

// repositorySet provides the repositories and the transactor. Replacing it switches the storage.
var repositorySet = wire.NewSet(
	providePool,
	sharedPostgres.NewTransactor,
	wire.Bind(new(repository.Transactor), new(*sharedPostgres.Transactor)),
	postgres.NewVoteRepository,
	wire.Bind(new(repository.VoteRepository), new(*postgres.VoteRepository)),
	postgres.NewPollRepository,
	wire.Bind(new(repository.PollRepository), new(*postgres.PollRepository)),
	wire.Bind(new(service.PollProvider), new(*postgres.PollRepository)),
	postgres.NewOutboxRepository,
	wire.Bind(new(repository.OutboxRepository), new(*postgres.OutboxRepository)),
)

// brokerSet provides the connection to RabbitMQ and the publisher the outbox relay uses.
var brokerSet = wire.NewSet(
	provideBroker,
	providePublisher,
	provideRelay,
)

func initApp(ctx context.Context, cfg Config) (*App, func(), error) {
	wire.Build(
		repositorySet,
		brokerSet,
		usecase.NewPollEventUseCase,
		wire.Bind(new(domainusecase.PollEventUseCase), new(*usecase.PollEventUseCase)),
		amqphandler.NewHandler,
		newPollEventConsumer,
		usecase.NewVoteUseCase,
		wire.Bind(new(domainusecase.VoteUseCase), new(*usecase.VoteUseCase)),
		votehttp.NewHandler,
		provideRouter,
		newApp,
	)
	return nil, nil, nil
}
