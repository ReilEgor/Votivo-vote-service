package main

import "time"

// Config holds the settings of the vote service, read from environment variables.
type Config struct {
	HTTPPort string `env:"APP_HTTP_PORT"   envDefault:"8080"`
	DBURL    string `env:"DB_URL,required"`

	// InternalAuthSecret is shared with the gateway. Vote trusts the user the gateway forwards in X-User-ID, so it
	// serves only calls that carry this secret in X-Internal-Auth.
	InternalAuthSecret string `env:"INTERNAL_AUTH_SECRET,required,unset"`

	RabbitMQURL    string        `env:"RABBITMQ_URL,required"`
	OutboxInterval time.Duration `env:"OUTBOX_INTERVAL"       envDefault:"1s"`
}
