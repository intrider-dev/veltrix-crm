package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

const maxSearchJobPayload = 8 << 10

type eventPointer struct {
	OutboxEventID string `json:"outboxEventId"`
	EventType     string `json:"eventType"`
	SchemaVersion int32  `json:"schemaVersion"`
	AggregateType string `json:"aggregateType"`
	AggregateID   string `json:"aggregateId"`
	CorrelationID string `json:"correlationId"`
}

// Reconciler deliberately loads the current source row instead of replaying
// the mutation payload. Consequently an old retried job cannot overwrite a
// newer document, and a missing/soft-deleted source removes its tombstone.
type Reconciler interface {
	Reconcile(context.Context, worker.Job, eventPointer) error
}

func NewWorkerHandler(reconciler Reconciler) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if reconciler == nil {
			return searchJobError{"search_not_configured", "search reconciler is not configured"}
		}
		pointer, err := decodeEventPointer(job)
		if err != nil {
			return err
		}
		if err := reconciler.Reconcile(ctx, job, pointer); err != nil {
			return searchJobError{"search_reconciliation_failed", err.Error()}
		}
		return nil
	}
}

func decodeEventPointer(job worker.Job) (eventPointer, error) {
	if job.SchemaVersion != 1 || len(job.Payload) == 0 || len(job.Payload) > maxSearchJobPayload {
		return eventPointer{}, searchJobError{"search_payload_invalid", "search job payload is invalid"}
	}
	decoder := json.NewDecoder(bytes.NewReader(job.Payload))
	decoder.DisallowUnknownFields()
	var pointer eventPointer
	if err := decoder.Decode(&pointer); err != nil {
		return eventPointer{}, searchJobError{"search_payload_invalid", "search job payload is invalid"}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return eventPointer{}, searchJobError{"search_payload_invalid", "search job payload has trailing data"}
	}
	if pointer.OutboxEventID == "" || pointer.SchemaVersion != 1 || pointer.AggregateType == "" || pointer.AggregateID == "" {
		return eventPointer{}, searchJobError{"search_payload_invalid", "search job pointer is incomplete"}
	}
	if _, err := ids.Parse(pointer.OutboxEventID); err != nil {
		return eventPointer{}, searchJobError{"search_payload_invalid", "outbox event ID is invalid"}
	}
	if _, err := ids.Parse(pointer.AggregateID); err != nil {
		return eventPointer{}, searchJobError{"search_payload_invalid", "aggregate ID is invalid"}
	}
	return pointer, nil
}

type PostgresReconciler struct {
	pool *pgxpool.Pool
}

func NewPostgresReconciler(pool *pgxpool.Pool) *PostgresReconciler {
	return &PostgresReconciler{pool: pool}
}

func (reconciler *PostgresReconciler) Reconcile(
	ctx context.Context, job worker.Job, pointer eventPointer,
) error {
	if reconciler == nil || reconciler.pool == nil {
		return errors.New("search database pool is required")
	}
	eventID, _ := ids.Parse(pointer.OutboxEventID)
	pointerAggregateID, _ := ids.Parse(pointer.AggregateID)
	tx, err := reconciler.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin search reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: job.WorkspaceID.String(), RequestID: "search:" + job.ID.String(),
	}); err != nil {
		return fmt.Errorf("set search tenant context: %w", err)
	}
	event, err := queries.GetSearchOutboxEvent(ctx, dbgen.GetSearchOutboxEventParams{
		WorkspaceID: job.WorkspaceID.PG(), EventID: eventID.PG(),
	})
	if err != nil {
		return fmt.Errorf("load search outbox event: %w", err)
	}
	aggregateID, valid := ids.FromPG(event.AggregateID)
	if !valid || event.SchemaVersion != pointer.SchemaVersion || event.AggregateType != pointer.AggregateType || aggregateID != pointerAggregateID {
		return errors.New("search job pointer does not match its tenant outbox event")
	}
	documentType, source, found, err := loadSearchSource(ctx, queries, job.WorkspaceID, event.AggregateType, aggregateID)
	if err != nil {
		return err
	}
	if !found {
		if err := queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
			WorkspaceID: job.WorkspaceID.PG(), EntityType: documentType, EntityID: aggregateID.PG(),
		}); err != nil {
			return fmt.Errorf("delete search tombstone: %w", err)
		}
	} else if err := queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: job.WorkspaceID.PG(), EntityType: documentType, EntityID: aggregateID.PG(),
		Title: source.title, Subtitle: source.subtitle, SearchableText: source.searchableText,
		RankBoost: source.rankBoost, Version: source.version,
	}); err != nil {
		return fmt.Errorf("reconcile search document: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit search reconciliation: %w", err)
	}
	return nil
}

type searchSource struct {
	title          string
	subtitle       *string
	searchableText string
	rankBoost      float32
	version        int64
}

func loadSearchSource(
	ctx context.Context, queries *dbgen.Queries, workspaceID ids.UUID, aggregateType string, aggregateID ids.UUID,
) (string, searchSource, bool, error) {
	switch aggregateType {
	case "contact":
		row, err := queries.GetContactSearchSource(ctx, dbgen.GetContactSearchSourceParams{WorkspaceID: workspaceID.PG(), EntityID: aggregateID.PG()})
		return sourceResult("contact", row.Title, row.Subtitle, row.SearchableText, 1.2, row.Version, err)
	case "company":
		row, err := queries.GetCompanySearchSource(ctx, dbgen.GetCompanySearchSourceParams{WorkspaceID: workspaceID.PG(), EntityID: aggregateID.PG()})
		return sourceResult("company", row.Title, row.Subtitle, row.SearchableText, 1.1, row.Version, err)
	case "lead":
		row, err := queries.GetLeadSearchSource(ctx, dbgen.GetLeadSearchSourceParams{WorkspaceID: workspaceID.PG(), EntityID: aggregateID.PG()})
		return sourceResult("lead", row.Title, row.Subtitle, row.SearchableText, 1.0, row.Version, err)
	case "deal":
		row, err := queries.GetDealSearchSource(ctx, dbgen.GetDealSearchSourceParams{WorkspaceID: workspaceID.PG(), EntityID: aggregateID.PG()})
		return sourceResult("deal", row.Title, row.Subtitle, row.SearchableText, 1.0, row.Version, err)
	case "activity":
		row, err := queries.GetNoteSearchSource(ctx, dbgen.GetNoteSearchSourceParams{WorkspaceID: workspaceID.PG(), EntityID: aggregateID.PG()})
		return sourceResult("note", row.Title, row.Subtitle, row.SearchableText, 0.7, row.Version, err)
	default:
		return "", searchSource{}, false, fmt.Errorf("unsupported searchable aggregate %q", aggregateType)
	}
}

func sourceResult(
	documentType, title string, subtitle *string, searchableText string, rankBoost float32, version int64, err error,
) (string, searchSource, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return documentType, searchSource{}, false, nil
	}
	if err != nil {
		return documentType, searchSource{}, false, fmt.Errorf("load %s search source: %w", documentType, err)
	}
	return documentType, searchSource{
		title: title, subtitle: subtitle, searchableText: searchableText, rankBoost: rankBoost, version: version,
	}, true, nil
}

type searchJobError struct {
	code    string
	message string
}

func (failure searchJobError) Error() string       { return failure.message }
func (failure searchJobError) FailureCode() string { return failure.code }

var _ Reconciler = (*PostgresReconciler)(nil)
