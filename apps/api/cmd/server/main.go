package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/app"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/automation"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/webui"
	platformworker "github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/seed"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string, logger *slog.Logger) error {
	command := "serve"
	if len(arguments) > 0 {
		command = arguments[0]
		arguments = arguments[1:]
	}
	var cfg config.Config
	var err error
	switch command {
	case "bootstrap", "migrate", "seed":
		cfg, err = config.LoadBootstrap()
	default:
		cfg, err = config.Load()
	}
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch command {
	case "serve":
		if len(arguments) != 0 {
			return fmt.Errorf("serve does not accept arguments")
		}
		return serve(ctx, cfg, logger)
	case "bootstrap":
		if len(arguments) != 0 {
			return fmt.Errorf("bootstrap does not accept arguments")
		}
		if err := migrate(ctx, cfg); err != nil {
			return err
		}
		if cfg.DemoSeed {
			return seedDatabase(ctx, cfg, "demo", os.Stdout)
		}
		return nil
	case "migrate":
		if len(arguments) != 0 {
			return fmt.Errorf("migrate does not accept arguments")
		}
		return migrate(ctx, cfg)
	case "seed":
		profile := "demo"
		if len(arguments) == 2 && arguments[0] == "--profile" {
			profile = arguments[1]
		} else if len(arguments) != 0 {
			return fmt.Errorf("usage: veltrix-crm seed [--profile demo|small|benchmark]")
		}
		return seedDatabase(ctx, cfg, profile, os.Stdout)
	case "worker":
		if len(arguments) != 0 {
			return fmt.Errorf("worker does not accept arguments")
		}
		return runWorker(ctx, cfg, logger)
	default:
		return fmt.Errorf("unknown command %q; expected serve, bootstrap, migrate, seed, or worker", command)
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.MaxDBConnections, "veltrix-crm-server")
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.DatabaseDispatcherURL == cfg.DatabaseURL {
		return fmt.Errorf("DATABASE_DISPATCHER_URL must use the dedicated dispatcher database role")
	}
	dispatcherConnections := int32(cfg.WorkerConcurrency + 1)
	if dispatcherConnections < 2 {
		dispatcherConnections = 2
	}
	dispatcherPool, err := database.Open(ctx, cfg.DatabaseDispatcherURL, dispatcherConnections, "veltrix-crm-dispatcher")
	if err != nil {
		return fmt.Errorf("open dispatcher database: %w", err)
	}
	defer dispatcherPool.Close()
	assets, err := webui.New()
	if err != nil {
		return fmt.Errorf("load embedded web UI: %w", err)
	}
	application, err := app.New(cfg, logger, pool, assets)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	defer func() {
		if err := application.Close(); err != nil {
			logger.Warn("close application resources", "error", err)
		}
	}()

	server := &http.Server{
		Addr: cfg.Address, Handler: application.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
		IdleTimeout: 75 * time.Second, MaxHeaderBytes: 32 << 10,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	listenerErrors := make(chan error, 1)
	workerErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server ready", "address", cfg.Address, "environment", cfg.Environment)
		listenerErrors <- server.ListenAndServe()
	}()
	go func() {
		if err := application.Hub().Run(ctx, cfg.DatabaseDispatcherURL); err != nil && ctx.Err() == nil {
			logger.Error("SSE listener stopped", "error", err)
		}
	}()
	go func() {
		logger.Info("integrated worker ready", "concurrency", cfg.WorkerConcurrency)
		workerErrors <- platformworker.Run(ctx, platformworker.Config{
			DispatcherPool: dispatcherPool,
			AppPool:        pool,
			Logger:         logger,
			Concurrency:    cfg.WorkerConcurrency,
			Handlers:       application.WorkerHandlers(),
		})
	}()
	go automation.RunTriggerProducer(ctx, dispatcherPool, logger, time.Minute)

	select {
	case err := <-listenerErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-workerErrors:
		if err == nil && ctx.Err() != nil {
			return nil
		}
		if err == nil {
			return errors.New("integrated worker stopped unexpectedly")
		}
		return fmt.Errorf("integrated worker: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
		return nil
	}
}

func migrate(ctx context.Context, cfg config.Config) error {
	if strings.TrimSpace(cfg.DatabaseAdminURL) == "" || cfg.AppDBPassword == "" {
		return fmt.Errorf("DATABASE_ADMIN_URL and APP_DB_PASSWORD are required for migrations")
	}
	return database.Migrate(ctx, cfg.DatabaseAdminURL, cfg.AppDBPassword)
}

func seedDatabase(ctx context.Context, cfg config.Config, profileName string, progress io.Writer) error {
	if strings.EqualFold(cfg.Environment, "production") {
		return fmt.Errorf("synthetic seeds are disabled in production")
	}
	profile, err := seed.ParseProfile(profileName)
	if err != nil {
		return err
	}
	if cfg.DatabaseAdminURL == "" {
		return fmt.Errorf("DATABASE_ADMIN_URL is required for seeding")
	}
	pool, err := database.Open(ctx, cfg.DatabaseAdminURL, 1, "veltrix-crm-seed")
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = seed.Run(ctx, pool, identity.NewPasswordHasher(1), profile, seed.Options{
		Environment: cfg.Environment, Email: cfg.DemoEmail, Password: cfg.DemoPassword, Progress: progress,
	})
	return err
}

func runWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if cfg.DatabaseDispatcherURL == cfg.DatabaseURL {
		return fmt.Errorf("DATABASE_DISPATCHER_URL must use the dedicated dispatcher database role")
	}
	dispatcherPool, err := database.Open(ctx, cfg.DatabaseDispatcherURL, int32(cfg.WorkerConcurrency+1), "veltrix-crm-worker")
	if err != nil {
		return fmt.Errorf("open dispatcher database: %w", err)
	}
	defer dispatcherPool.Close()
	appPool, err := database.Open(ctx, cfg.DatabaseURL, cfg.MaxDBConnections, "veltrix-crm-worker-domain")
	if err != nil {
		return fmt.Errorf("open application database: %w", err)
	}
	defer appPool.Close()
	handlers, err := app.BuildWorkerHandlers(cfg, logger, appPool, nil)
	if err != nil {
		return fmt.Errorf("configure worker handlers: %w", err)
	}
	logger.Info("standalone worker ready", "concurrency", cfg.WorkerConcurrency)
	go automation.RunTriggerProducer(ctx, dispatcherPool, logger, time.Minute)
	return platformworker.Run(ctx, platformworker.Config{
		DispatcherPool: dispatcherPool,
		AppPool:        appPool,
		Logger:         logger,
		Concurrency:    cfg.WorkerConcurrency,
		Handlers:       handlers,
	})
}
