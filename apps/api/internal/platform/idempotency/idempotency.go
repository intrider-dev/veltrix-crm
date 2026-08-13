package idempotency

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Replay struct {
	Status int
	Body   []byte
}

func Reserve(
	ctx context.Context,
	queries *dbgen.Queries,
	workspaceID, actorID ids.UUID,
	key, operation string,
	body []byte,
) (*Replay, error) {
	if len(key) < 16 || len(key) > 128 {
		return nil, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/headers/Idempotency-Key",
			Code:    "validation.idempotency_key.invalid",
		}}}
	}
	digest := sha256.Sum256(body)
	_, err := queries.ReserveIdempotencyKey(ctx, dbgen.ReserveIdempotencyKeyParams{
		WorkspaceID: workspaceID.PG(),
		Key:         key,
		ActorID:     actorID.PG(),
		Operation:   operation,
		RequestHash: digest[:],
	})
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("reserve idempotency key: %w", err)
	}
	existing, err := queries.GetIdempotencyKey(ctx, dbgen.GetIdempotencyKeyParams{
		WorkspaceID: workspaceID.PG(),
		Key:         key,
	})
	if err != nil {
		return nil, fmt.Errorf("load idempotency key: %w", err)
	}
	if existing.ActorID.Bytes != actorID || existing.Operation != operation ||
		len(existing.RequestHash) != len(digest) || subtle.ConstantTimeCompare(existing.RequestHash, digest[:]) != 1 {
		return nil, errx.ErrIdempotencyConflict
	}
	if existing.ResponseStatus == nil || len(existing.ResponseBody) == 0 {
		return nil, errx.ErrIdempotencyConflict
	}
	return &Replay{Status: int(*existing.ResponseStatus), Body: existing.ResponseBody}, nil
}

func Complete(ctx context.Context, queries *dbgen.Queries, workspaceID ids.UUID, key string, status int, body []byte) error {
	responseStatus := int32(status)
	if err := queries.CompleteIdempotencyKey(ctx, dbgen.CompleteIdempotencyKeyParams{
		WorkspaceID:    workspaceID.PG(),
		Key:            key,
		ResponseStatus: &responseStatus,
		ResponseBody:   body,
	}); err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}
