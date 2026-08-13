package worker

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const maxWorkerConcurrency = 32

type Config struct {
	DispatcherPool *pgxpool.Pool
	AppPool        *pgxpool.Pool
	Logger         *slog.Logger
	Concurrency    int
	Handlers       map[string]Handler
	WorkerID       string

	PollInterval     time.Duration
	LeaseDuration    time.Duration
	JobTimeout       time.Duration
	OperationTimeout time.Duration
	BackoffBase      time.Duration
	BackoffMaximum   time.Duration
	OutboxBatchSize  int32
	MaxAttempts      int32
}

func (config Config) normalized() (Config, error) {
	if config.DispatcherPool == nil {
		return Config{}, errors.New("worker dispatcher pool is required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Concurrency == 0 {
		config.Concurrency = 2
	}
	if config.Concurrency < 1 || config.Concurrency > maxWorkerConcurrency {
		return Config{}, fmt.Errorf("worker concurrency must be between 1 and %d", maxWorkerConcurrency)
	}
	if config.PollInterval == 0 {
		config.PollInterval = 500 * time.Millisecond
	}
	if config.PollInterval < 25*time.Millisecond || config.PollInterval > time.Minute {
		return Config{}, errors.New("worker poll interval must be between 25ms and 1m")
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 30 * time.Second
	}
	if config.LeaseDuration < 5*time.Second || config.LeaseDuration > 10*time.Minute {
		return Config{}, errors.New("worker lease duration must be between 5s and 10m")
	}
	if config.JobTimeout == 0 {
		config.JobTimeout = 20 * time.Second
	}
	if config.JobTimeout <= 0 || config.JobTimeout >= config.LeaseDuration {
		return Config{}, errors.New("worker job timeout must be positive and shorter than its lease")
	}
	if config.OperationTimeout == 0 {
		config.OperationTimeout = 10 * time.Second
	}
	if config.OperationTimeout <= 0 || config.OperationTimeout > time.Minute {
		return Config{}, errors.New("worker database operation timeout must be between 1ns and 1m")
	}
	if config.BackoffBase == 0 {
		config.BackoffBase = time.Second
	}
	if config.BackoffMaximum == 0 {
		config.BackoffMaximum = 5 * time.Minute
	}
	if config.BackoffBase <= 0 || config.BackoffMaximum < config.BackoffBase {
		return Config{}, errors.New("worker backoff maximum must be at least its positive base")
	}
	if config.OutboxBatchSize == 0 {
		config.OutboxBatchSize = 50
	}
	if config.OutboxBatchSize < 1 || config.OutboxBatchSize > 500 {
		return Config{}, errors.New("worker outbox batch size must be between 1 and 500")
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 8
	}
	if config.MaxAttempts < 1 || config.MaxAttempts > 100 {
		return Config{}, errors.New("worker max attempts must be between 1 and 100")
	}
	if config.WorkerID == "" {
		id, err := ids.NewV7()
		if err != nil {
			return Config{}, fmt.Errorf("generate worker ID: %w", err)
		}
		config.WorkerID = "worker-" + id.String()
	}
	if len(config.WorkerID) > 120 {
		return Config{}, errors.New("worker ID must not exceed 120 characters")
	}
	if config.Handlers == nil {
		config.Handlers = map[string]Handler{}
	}
	return config, nil
}
