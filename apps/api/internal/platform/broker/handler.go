package broker

import (
	"context"
	"errors"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func NewPublishHandler(publisher Publisher) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if publisher == nil {
			return errors.New("broker publisher is not configured")
		}
		envelope, err := ParseEnvelope(job.Payload)
		if err != nil {
			return err
		}
		if envelope.WorkspaceID != job.WorkspaceID.String() || envelope.EventID != job.IdempotencyKey {
			return errors.New("broker job does not match its tenant or idempotency key")
		}
		return publisher.Publish(ctx, envelope)
	}
}
