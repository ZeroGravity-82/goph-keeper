package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	serverApp "zerogravity-82/goph-keeper/internal/app/server"
	"zerogravity-82/goph-keeper/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Пользователь запросил справку по флагам командной строки; это штатное завершение.
		if errors.Is(err, config.ErrHelp) {
			return
		}

		// Логгер еще не сконфигурирован.
		log.Fatalf("config error: %v", err)
	}

	logger, err := newLogger(cfg.Logging)
	if err != nil {
		// логгер еще не сконфигурирован
		log.Fatalf("logger config error: %v", err)
	}

	if err := run(cfg, logger); err != nil {
		logger.Error("service terminated with error", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("service stopped (graceful)")
}

func run(cfg config.Config, logger *slog.Logger) error {
	application, err := serverApp.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("app init error: %w", err)
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return application.Run(ctx)
}
