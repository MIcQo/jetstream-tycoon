// Command server runs the stateless HTTP API server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MIcQo/jetstream-tycoon/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := server.LoadConfig(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	if err := server.InitSentry(cfg); err != nil {
		logger.Error("sentry init failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg, logger); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
