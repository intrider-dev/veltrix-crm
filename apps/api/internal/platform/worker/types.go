package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

var ErrLeaseLost = errors.New("job lease lost")

type Job struct {
	WorkspaceID    ids.UUID
	ID             ids.UUID
	Kind           string
	SchemaVersion  int32
	IdempotencyKey string
	Payload        json.RawMessage
	Attempts       int32
	MaxAttempts    int32
	LockedAt       time.Time
	LockedUntil    time.Time
	FencingToken   int64
}

type Dependencies struct {
	// AppPool is optional. Domain handlers that use it must still open a
	// transaction and establish transaction-local tenant and actor context.
	AppPool *pgxpool.Pool
}

type Handler func(context.Context, Dependencies, Job) error

type codedError interface {
	FailureCode() string
}

type unknownJobKindError struct {
	kind string
}

func (failure unknownJobKindError) Error() string {
	return fmt.Sprintf("no handler is registered for job kind %q", failure.kind)
}

func (failure unknownJobKindError) FailureCode() string {
	return "unknown_job_kind"
}

func failureCode(err error) string {
	if err == nil {
		return "none"
	}
	var coded codedError
	if errors.As(err, &coded) {
		code := coded.FailureCode()
		if code != "" && len(code) <= 120 {
			return code
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "job_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "job_cancelled"
	}
	return "handler_failed"
}

func (job Job) leaseValid(now time.Time) bool {
	return !job.LockedUntil.IsZero() && now.Before(job.LockedUntil)
}
