package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/pagination"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type DealInput struct {
	Name              string
	PipelineID        ids.UUID
	StageID           ids.UUID
	ContactID         *ids.UUID
	CompanyID         *ids.UUID
	OwnerID           *ids.UUID
	AmountMinor       int64
	Currency          string
	PlannedStartDate  *time.Time
	ExpectedCloseDate *time.Time
}

type DealPage struct {
	Items      []dbgen.ListDealsRow
	NextCursor string
}

type Pipeline struct {
	Row    dbgen.ListPipelinesRow
	Stages []dbgen.ListPipelineStagesRow
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) ListPipelines(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID) ([]Pipeline, error) {
	rows, err := workspace.Queries.ListPipelines(ctx, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	stageRows, err := workspace.Queries.ListPipelineStagesForWorkspace(ctx, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("list pipeline stages: %w", err)
	}
	stagesByPipeline := make(map[ids.UUID][]dbgen.ListPipelineStagesRow, len(rows))
	for _, stage := range stageRows {
		pipelineID, ok := ids.FromPG(stage.PipelineID)
		if !ok {
			return nil, fmt.Errorf("list pipeline stages: invalid pipeline id")
		}
		stagesByPipeline[pipelineID] = append(stagesByPipeline[pipelineID], dbgen.ListPipelineStagesRow{
			ID: stage.ID, PipelineID: stage.PipelineID, Name: stage.Name, Probability: stage.Probability,
			ForecastCategory: stage.ForecastCategory, Position: stage.Position,
			CreatedAt: stage.CreatedAt, UpdatedAt: stage.UpdatedAt,
		})
	}
	result := make([]Pipeline, 0, len(rows))
	for _, row := range rows {
		pipelineID, ok := ids.FromPG(row.ID)
		if !ok {
			return nil, fmt.Errorf("list pipelines: invalid id")
		}
		stages := stagesByPipeline[pipelineID]
		if stages == nil {
			stages = []dbgen.ListPipelineStagesRow{}
		}
		result = append(result, Pipeline{Row: row, Stages: stages})
	}
	return result, nil
}

func (service *Service) ListDeals(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	pipelineID, stageID *ids.UUID,
	cursor string,
	limit int,
) (DealPage, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	filter := "pipeline=" + uuidString(pipelineID) + "&stage=" + uuidString(stageID)
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return DealPage{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/cursor", Code: "validation.cursor.invalid"}}}
	}
	rows, err := workspace.Queries.ListDeals(ctx, dbgen.ListDealsParams{
		WorkspaceID: workspaceID.PG(), PipelineID: optionalUUID(pipelineID), StageID: optionalUUID(stageID),
		CursorUpdatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true}, CursorID: cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return DealPage{}, fmt.Errorf("list deals: %w", err)
	}
	page := DealPage{Items: rows}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.UpdatedAt.Time, lastID, filter)
		if err != nil {
			return DealPage{}, err
		}
		page.Items = rows[:limit]
	}
	return page, nil
}

func (service *Service) GetDeal(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, dealID ids.UUID) (dbgen.GetDealRow, error) {
	row, err := workspace.Queries.GetDeal(ctx, dbgen.GetDealParams{WorkspaceID: workspaceID.PG(), ID: dealID.PG()})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.GetDealRow{}, errx.ErrNotFound
	}
	if err != nil {
		return dbgen.GetDealRow{}, fmt.Errorf("get deal: %w", err)
	}
	return row, nil
}

func (service *Service) CreateDeal(ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, input DealInput) (dbgen.CreateDealRow, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	var fields []errx.FieldError
	if len(input.Name) < 1 || len(input.Name) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if input.AmountMinor < 0 {
		fields = append(fields, errx.FieldError{Pointer: "/amountMinor", Code: "validation.minimum"})
	}
	if !currencyCodePattern.MatchString(input.Currency) {
		fields = append(fields, errx.FieldError{Pointer: "/currency", Code: "validation.currency.invalid"})
	}
	if input.PlannedStartDate != nil && input.ExpectedCloseDate != nil && input.PlannedStartDate.After(*input.ExpectedCloseDate) {
		fields = append(fields, errx.FieldError{Pointer: "/plannedStartDate", Code: "validation.date.range"})
	}
	if len(fields) > 0 {
		return dbgen.CreateDealRow{}, &errx.ValidationError{Fields: fields}
	}
	if _, err := workspace.Queries.GetPipelineStage(ctx, dbgen.GetPipelineStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), PipelineID: input.PipelineID.PG(), ID: input.StageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return dbgen.CreateDealRow{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stageId", Code: "validation.reference.invalid"}}}
	} else if err != nil {
		return dbgen.CreateDealRow{}, err
	}
	position, err := workspace.Queries.NextDealPosition(ctx, dbgen.NextDealPositionParams{
		WorkspaceID: metadata.WorkspaceID.PG(), PipelineID: input.PipelineID.PG(), StageID: input.StageID.PG(),
	})
	if err != nil {
		return dbgen.CreateDealRow{}, err
	}
	dealID, err := ids.NewV7()
	if err != nil {
		return dbgen.CreateDealRow{}, err
	}
	row, err := workspace.Queries.CreateDeal(ctx, dbgen.CreateDealParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), PipelineID: input.PipelineID.PG(), StageID: input.StageID.PG(),
		Name: input.Name, ContactID: optionalUUID(input.ContactID), CompanyID: optionalUUID(input.CompanyID), OwnerUserID: optionalUUID(input.OwnerID),
		AmountMinor: input.AmountMinor, Currency: input.Currency, PlannedStartDate: optionalDate(input.PlannedStartDate),
		ExpectedCloseDate: optionalDate(input.ExpectedCloseDate), Position: position,
	})
	if err != nil {
		return dbgen.CreateDealRow{}, mapConstraintError(err)
	}
	historyID, err := ids.NewV7()
	if err != nil {
		return dbgen.CreateDealRow{}, err
	}
	if err := workspace.Queries.AddDealStageHistory(ctx, dbgen.AddDealStageHistoryParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: historyID.PG(), DealID: dealID.PG(), FromStageID: pgtype.UUID{},
		ToStageID: input.StageID.PG(), ChangedBy: metadata.ActorID.PG(),
	}); err != nil {
		return dbgen.CreateDealRow{}, err
	}
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "deal", EntityID: dealID.PG(), Title: row.Name,
		SearchableText: row.Name, RankBoost: 1.0, Version: row.Version,
	}); err != nil {
		return dbgen.CreateDealRow{}, err
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return dbgen.CreateDealRow{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "deal.created", EventType: "sales.deal.created", AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"fields": []string{"name", "stageId", "amountMinor", "currency", "plannedStartDate", "expectedCloseDate"}},
		Payload: map[string]any{"dealId": dealID.String(), "stageId": input.StageID.String(), "version": row.Version},
	}); err != nil {
		return dbgen.CreateDealRow{}, err
	}
	return row, nil
}

func (service *Service) MoveDeal(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID, stageID ids.UUID,
	position int,
	version int64,
) (dbgen.MoveDealRow, error) {
	existing, err := service.GetDeal(ctx, workspace, metadata.WorkspaceID, dealID)
	if err != nil {
		return dbgen.MoveDealRow{}, err
	}
	if existing.Version != version {
		return dbgen.MoveDealRow{}, errx.ErrVersionConflict
	}
	if existing.Status != "open" {
		return dbgen.MoveDealRow{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/status", Code: "validation.deal.closed_move"}}}
	}
	if position < 0 {
		return dbgen.MoveDealRow{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/position", Code: "validation.minimum"}}}
	}
	if _, err := workspace.Queries.GetPipelineStage(ctx, dbgen.GetPipelineStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), PipelineID: existing.PipelineID, ID: stageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return dbgen.MoveDealRow{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stageId", Code: "validation.reference.invalid"}}}
	} else if err != nil {
		return dbgen.MoveDealRow{}, err
	}
	row, err := workspace.Queries.MoveDeal(ctx, dbgen.MoveDealParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), StageID: stageID.PG(), Position: int32(position), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.MoveDealRow{}, errx.ErrVersionConflict
	}
	if err != nil {
		return dbgen.MoveDealRow{}, err
	}
	historyID, err := ids.NewV7()
	if err != nil {
		return dbgen.MoveDealRow{}, err
	}
	if err := workspace.Queries.AddDealStageHistory(ctx, dbgen.AddDealStageHistoryParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: historyID.PG(), DealID: dealID.PG(), FromStageID: existing.StageID,
		ToStageID: stageID.PG(), ChangedBy: metadata.ActorID.PG(),
	}); err != nil {
		return dbgen.MoveDealRow{}, err
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return dbgen.MoveDealRow{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "deal.stage_changed", EventType: "sales.deal.stage_changed", AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"fromStageId": pgUUIDString(existing.StageID), "toStageId": stageID.String()},
		Payload: map[string]any{"dealId": dealID.String(), "fromStageId": pgUUIDString(existing.StageID), "toStageId": stageID.String(), "version": row.Version},
	}); err != nil {
		return dbgen.MoveDealRow{}, err
	}
	return row, nil
}

func optionalUUID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func optionalDate(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: value.UTC(), Valid: true}
}

func uuidString(value *ids.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func pgUUIDString(value pgtype.UUID) string {
	id, ok := ids.FromPG(value)
	if !ok {
		return ""
	}
	return id.String()
}

func mapConstraintError(err error) error {
	var pgError interface{ SQLState() string }
	if errors.As(err, &pgError) {
		switch pgError.SQLState() {
		case "23503":
			return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/relation", Code: "validation.reference.invalid"}}}
		case "23505":
			return errx.ErrConflict
		case "23514":
			return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/record", Code: "validation.constraint"}}}
		}
	}
	return fmt.Errorf("database constraint: %w", err)
}
