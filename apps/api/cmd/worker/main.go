package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/app"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/automation"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error_code", "worker_failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if cfg.DatabaseDispatcherURL == cfg.DatabaseURL {
		return fmt.Errorf("DATABASE_DISPATCHER_URL must use the dedicated dispatcher database role")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dispatcherPool, err := database.Open(ctx, cfg.DatabaseDispatcherURL, cfg.MaxDBConnections, "crm-worker")
	if err != nil {
		return fmt.Errorf("open dispatcher database: %w", err)
	}
	defer dispatcherPool.Close()
	appPool, err := database.Open(ctx, cfg.DatabaseURL, cfg.MaxDBConnections, "crm-worker-domain")
	if err != nil {
		return fmt.Errorf("open application database: %w", err)
	}
	defer appPool.Close()
	handlers, err := app.BuildWorkerHandlers(cfg, logger, appPool, nil)
	if err != nil {
		return fmt.Errorf("configure worker handlers: %w", err)
	}

	logger.Info("worker starting", "concurrency", cfg.WorkerConcurrency)
	go automation.RunTriggerProducer(ctx, dispatcherPool, logger, time.Minute)
	if err := worker.Run(ctx, worker.Config{
		DispatcherPool: dispatcherPool,
		AppPool:        appPool,
		Logger:         logger,
		Concurrency:    cfg.WorkerConcurrency,
		Handlers:       handlers,
	}); err != nil {
		return err
	}
	logger.Info("worker stopped")
	return nil
}
