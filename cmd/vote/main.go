// Command vote runs the vote service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"

	"github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/httpserver"
	"github.com/ReilEgor/Votivo-vote-service/pkg/ctxlog"
)

// commit is the git commit the binary was built from. It is set with -ldflags "-X main.commit=<hash>".
var commit = "dev"

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(ctxlog.NewHandler(jsonHandler)).With(slog.String("service", "vote"))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		logger.Error("service stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	app, cleanup, err := initApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("assemble service: %w", err)
	}
	defer cleanup()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)
	go func() { errCh <- httpserver.Run(ctx, ":"+cfg.HTTPPort, app.HTTP) }()
	go func() { errCh <- app.Relay.Run(ctx) }()
	go func() { errCh <- app.PollEvents.Run(ctx) }()

	slog.InfoContext(ctx, "vote starting",
		slog.String("http_port", cfg.HTTPPort),
		slog.String("version", commit),
	)

	err = <-errCh
	cancel()
	<-errCh
	<-errCh
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
