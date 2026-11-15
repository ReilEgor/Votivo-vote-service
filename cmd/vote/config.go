package main

import "time"

// Config holds the settings of the vote service, read from environment variables.
type Config struct {
	HTTPPort string `env:"APP_HTTP_PORT"   envDefault:"8080"`
	DBURL    string `env:"DB_URL,required"`

	RabbitMQURL    string        `env:"RABBITMQ_URL,required"`
	OutboxInterval time.Duration `env:"OUTBOX_INTERVAL"       envDefault:"1s"`

	// PollServiceAddr is the host:port of the gRPC server of Poll Service.
	// When it is empty the service answers from a fixed list of polls, see clients/poll/static.go.
	PollServiceAddr string `env:"POLL_SERVICE_ADDR"`
}
