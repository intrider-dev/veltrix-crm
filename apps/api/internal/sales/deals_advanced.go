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

func (service *Service) ListDealsAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	filter DealListFilter,
) (DealPageAdvanced, error) {
	validated, err := validateDealFilter(filter)
	if err != nil {
		return DealPageAdvanced{}, err
	}
	fingerprint := strings.Join([]string{
		"q=" + validated.Query,
		"pipeline=" + uuidString(validated.PipelineID),
		"stage=" + uuidString(validated.StageID),
		"owner=" + uuidString(validated.OwnerID),
		"status=" + validated.Status,
	}, "&")
	cursorTime, cursorID, err := pagination.Decode(validated.Cursor, fingerprint)
	if err != nil {
		return DealPageAdvanced{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListDealsAdvanced(ctx, dbgen.ListDealsAdvancedParams{
		WorkspaceID: workspaceID.PG(), SearchQuery: validated.Query,
		PipelineID: optionalUUID(validated.PipelineID), StageID: optionalUUID(validated.StageID),
		OwnerID: optionalUUID(validated.OwnerID), StatusFilter: validated.Status,
		CursorUpdatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true}, CursorID: cursorID.PG(),
		PageLimit: int32(validated.Limit + 1),
	})
	if err != nil {
		return DealPageAdvanced{}, fmt.Errorf("list deals: %w", err)
	}
	page := DealPageAdvanced{Items: make([]DealRecord, 0, min(len(rows), validated.Limit))}
	for index, row := range rows {
		if index == validated.Limit {
			break
		}
		page.Items = append(page.Items, makeDealRecord(
			row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID,
			row.OwnerUserID, row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position,
			row.Status, row.LostReason, row.ForecastCategory, row.WonAt, row.LostAt,
			row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
		))
	}
	if len(rows) > validated.Limit {
		last := rows[validated.Limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.UpdatedAt.Time, lastID, fingerprint)
		if err != nil {
			return DealPageAdvanced{}, fmt.Errorf("encode deal cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) GetDealRecord(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
) (DealRecord, error) {
	row, err := workspace.Queries.GetDealAdvanced(ctx, dbgen.GetDealAdvancedParams{
		WorkspaceID: workspaceID.PG(), ID: dealID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DealRecord{}, errx.ErrNotFound
	}
	if err != nil {
		return DealRecord{}, fmt.Errorf("get deal: %w", err)
	}
	return makeDealRecord(
		row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID,
		row.OwnerUserID, row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position,
		row.Status, row.LostReason, row.ForecastCategory, row.WonAt, row.LostAt,
		row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) UpdateDeal(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	version int64,
	input DealUpdateInput,
) (DealRecord, error) {
	validated, customFields, err := validateDealUpdate(input)
	if err != nil {
		return DealRecord{}, err
	}
	existing, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, version, false)
	if err != nil {
		return DealRecord{}, err
	}
	existingPipeline, _ := ids.FromPG(existing.PipelineID)
	existingStage, _ := ids.FromPG(existing.StageID)
	if existingPipeline != validated.PipelineID || existingStage != validated.StageID {
		return DealRecord{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/stageId", Code: "validation.deal.use_stage_move",
		}}}
	}
	if (existing.Status == "open" && validated.ForecastCategory == "closed") ||
		(existing.Status != "open" && validated.ForecastCategory != "closed") {
		return DealRecord{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/forecastCategory", Code: "validation.deal.forecast_status_mismatch",
		}}}
	}
	row, err := workspace.Queries.UpdateDealAdvanced(ctx, dbgen.UpdateDealAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), PipelineID: validated.PipelineID.PG(),
		StageID: validated.StageID.PG(), Name: validated.Name, ContactID: optionalUUID(validated.ContactID),
		CompanyID: optionalUUID(validated.CompanyID), OwnerUserID: optionalUUID(validated.OwnerID),
		AmountMinor: validated.AmountMinor, Currency: validated.Currency,
		PlannedStartDate: optionalDate(validated.PlannedStartDate), ExpectedCloseDate: optionalDate(validated.ExpectedCloseDate),
		ForecastCategory: validated.ForecastCategory,
		CustomFields:     customFields, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DealRecord{}, service.classifyDealMutation(ctx, workspace, metadata.WorkspaceID, dealID, version, false)
	}
	if err != nil {
		return DealRecord{}, mapConstraintError(err)
	}
	if err := service.upsertDealSearch(ctx, workspace, metadata.WorkspaceID, dealID, row.Name, row.Version); err != nil {
		return DealRecord{}, err
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return DealRecord{}, fmt.Errorf("refresh dashboard: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "deal.updated", EventType: "sales.deal.updated", AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"fields": []string{"name", "contactId", "companyId", "ownerId", "amountMinor", "currency", "plannedStartDate", "expectedCloseDate", "forecastCategory", "customFields"}},
		Payload: map[string]any{"dealId": dealID.String(), "version": row.Version},
	}); err != nil {
		return DealRecord{}, err
	}
	return makeDealRecord(
		row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID,
		row.OwnerUserID, row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position,
		row.Status, row.LostReason, row.ForecastCategory, row.WonAt, row.LostAt,
		row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) SetDealOutcomeAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	version int64,
	input DealOutcomeInput,
) (DealRecord, error) {
	validated, err := validateDealOutcome(input)
	if err != nil {
		return DealRecord{}, err
	}
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, version, false); err != nil {
		return DealRecord{}, err
	}
	lostReason := ""
	if validated.LostReason != nil {
		lostReason = *validated.LostReason
	}
	row, err := workspace.Queries.SetDealOutcome(ctx, dbgen.SetDealOutcomeParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), Status: validated.Status,
		Column4: lostReason, ForecastCategory: validated.ForecastCategory, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DealRecord{}, service.classifyDealMutation(ctx, workspace, metadata.WorkspaceID, dealID, version, false)
	}
	if err != nil {
		return DealRecord{}, mapConstraintError(err)
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return DealRecord{}, fmt.Errorf("refresh dashboard: %w", err)
	}
	eventType := "sales.deal.updated"
	action := "deal.reopened"
	if row.Status == "won" {
		eventType, action = "sales.deal.won", "deal.won"
	} else if row.Status == "lost" {
		eventType, action = "sales.deal.lost", "deal.lost"
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: action, EventType: eventType, AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"status": row.Status, "forecastCategory": row.ForecastCategory},
		Payload: map[string]any{"dealId": dealID.String(), "status": row.Status, "version": row.Version},
	}); err != nil {
		return DealRecord{}, err
	}
	return makeDealRecord(
		row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID,
		row.OwnerUserID, row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position,
		row.Status, row.LostReason, row.ForecastCategory, row.WonAt, row.LostAt,
		row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) DeleteDeal(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	version int64,
) error {
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, version, false); err != nil {
		return err
	}
	newVersion, err := workspace.Queries.SoftDeleteDeal(ctx, dbgen.SoftDeleteDealParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), DeletedBy: metadata.ActorID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return service.classifyDealMutation(ctx, workspace, metadata.WorkspaceID, dealID, version, false)
	}
	if err != nil {
		return fmt.Errorf("delete deal: %w", err)
	}
	if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "deal", EntityID: dealID.PG(),
	}); err != nil {
		return fmt.Errorf("delete deal search document: %w", err)
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return fmt.Errorf("refresh dashboard: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "deal.deleted", EventType: "sales.deal.deleted", AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"softDelete": true}, Payload: map[string]any{"dealId": dealID.String(), "version": newVersion},
	})
}

func (service *Service) RestoreDeal(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	version int64,
) (DealRecord, error) {
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, version, true); err != nil {
		return DealRecord{}, err
	}
	row, err := workspace.Queries.RestoreDeal(ctx, dbgen.RestoreDealParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DealRecord{}, service.classifyDealMutation(ctx, workspace, metadata.WorkspaceID, dealID, version, true)
	}
	if err != nil {
		return DealRecord{}, fmt.Errorf("restore deal: %w", err)
	}
	if err := service.upsertDealSearch(ctx, workspace, metadata.WorkspaceID, dealID, row.Name, row.Version); err != nil {
		return DealRecord{}, err
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return DealRecord{}, fmt.Errorf("refresh dashboard: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "deal.restored", EventType: "sales.deal.restored", AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"restored": true}, Payload: map[string]any{"dealId": dealID.String(), "version": row.Version},
	}); err != nil {
		return DealRecord{}, err
	}
	return makeDealRecord(
		row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID,
		row.OwnerUserID, row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position,
		row.Status, row.LostReason, row.ForecastCategory, row.WonAt, row.LostAt,
		row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) ListDealTrash(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	cursor string,
	limit int,
) (DeletedSalesPage, error) {
	limit = clampPageSize(limit, DefaultPageSize, MaxPageSize)
	cursorTime, cursorID, err := pagination.Decode(cursor, "deal-trash")
	if err != nil {
		return DeletedSalesPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListDealTrash(ctx, dbgen.ListDealTrashParams{
		WorkspaceID: workspaceID.PG(), CursorDeletedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true},
		CursorID: cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return DeletedSalesPage{}, fmt.Errorf("list deal trash: %w", err)
	}
	page := DeletedSalesPage{Items: make([]DeletedSalesRecord, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			break
		}
		id, _ := ids.FromPG(row.ID)
		page.Items = append(page.Items, DeletedSalesRecord{
			ID: id.String(), Name: row.Name, Status: row.Status, Version: row.Version,
			DeletedAt: row.DeletedAt.Time.UTC(), DeletedBy: optionalIDString(row.DeletedBy),
			CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
		})
	}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.DeletedAt.Time, lastID, "deal-trash")
		if err != nil {
			return DeletedSalesPage{}, fmt.Errorf("encode deal trash cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) ListKanbanStage(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, pipelineID, stageID ids.UUID,
	cursor string,
	limit int,
) (KanbanPage, error) {
	limit = clampPageSize(limit, DefaultPageSize, MaxKanbanPageSize)
	if _, err := workspace.Queries.GetPipelineStage(ctx, dbgen.GetPipelineStageParams{
		WorkspaceID: workspaceID.PG(), PipelineID: pipelineID.PG(), ID: stageID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return KanbanPage{}, errx.ErrNotFound
	} else if err != nil {
		return KanbanPage{}, fmt.Errorf("get pipeline stage: %w", err)
	}
	fingerprint := "pipeline=" + pipelineID.String() + "&stage=" + stageID.String()
	afterPosition, afterID, err := decodeKanbanCursor(cursor, fingerprint)
	if err != nil {
		return KanbanPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListKanbanStageDeals(ctx, dbgen.ListKanbanStageDealsParams{
		WorkspaceID: workspaceID.PG(), PipelineID: pipelineID.PG(), StageID: stageID.PG(),
		AfterPosition: afterPosition, AfterID: afterID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return KanbanPage{}, fmt.Errorf("list kanban stage: %w", err)
	}
	page := KanbanPage{Items: make([]KanbanCard, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			break
		}
		page.Items = append(page.Items, makeKanbanCard(row))
	}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = encodeKanbanCursor(last.Position, lastID, fingerprint)
		if err != nil {
			return KanbanPage{}, fmt.Errorf("encode kanban cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) ListDealStageHistory(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
	cursor string,
	limit int,
) (StageHistoryPage, error) {
	limit = clampPageSize(limit, DefaultPageSize, MaxPageSize)
	if _, err := service.GetDeal(ctx, workspace, workspaceID, dealID); err != nil {
		return StageHistoryPage{}, err
	}
	fingerprint := "deal-history=" + dealID.String()
	cursorTime, cursorID, err := pagination.Decode(cursor, fingerprint)
	if err != nil {
		return StageHistoryPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListDealStageHistoryAdvanced(ctx, dbgen.ListDealStageHistoryAdvancedParams{
		WorkspaceID: workspaceID.PG(), DealID: dealID.PG(),
		CursorChangedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true}, CursorID: cursorID.PG(),
		PageLimit: int32(limit + 1),
	})
	if err != nil {
		return StageHistoryPage{}, fmt.Errorf("list deal stage history: %w", err)
	}
	page := StageHistoryPage{Items: make([]StageHistoryRecord, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			break
		}
		id, _ := ids.FromPG(row.ID)
		parentID, _ := ids.FromPG(row.DealID)
		toID, _ := ids.FromPG(row.ToStageID)
		actorID, _ := ids.FromPG(row.ChangedBy)
		page.Items = append(page.Items, StageHistoryRecord{
			ID: id.String(), DealID: parentID.String(), FromStageID: optionalIDString(row.FromStageID),
			ToStageID: toID.String(), ChangedBy: actorID.String(), ChangedAt: row.ChangedAt.Time.UTC(),
		})
	}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.ChangedAt.Time, lastID, fingerprint)
		if err != nil {
			return StageHistoryPage{}, fmt.Errorf("encode stage history cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) requireDealVersion(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
	version int64,
	requireDeleted bool,
) (dbgen.GetDealAdvancedRow, error) {
	if version < 1 {
		return dbgen.GetDealAdvancedRow{}, validation("/headers/If-Match", "validation.etag.invalid")
	}
	if requireDeleted {
		if err := service.classifyDealMutation(ctx, workspace, workspaceID, dealID, version, true); err != nil {
			return dbgen.GetDealAdvancedRow{}, err
		}
		return dbgen.GetDealAdvancedRow{}, nil
	}
	row, err := workspace.Queries.GetDealAdvanced(ctx, dbgen.GetDealAdvancedParams{
		WorkspaceID: workspaceID.PG(), ID: dealID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.GetDealAdvancedRow{}, errx.ErrNotFound
	}
	if err != nil {
		return dbgen.GetDealAdvancedRow{}, fmt.Errorf("get deal: %w", err)
	}
	if row.Version != version {
		return dbgen.GetDealAdvancedRow{}, errx.ErrVersionConflict
	}
	return row, nil
}

func (service *Service) classifyDealMutation(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
	version int64,
	requireDeleted bool,
) error {
	var currentVersion int64
	var deleted bool
	err := workspace.Tx.QueryRow(ctx, `
		SELECT version, deleted_at IS NOT NULL FROM sales.deals
		WHERE workspace_id = $1 AND id = $2`, workspaceID.PG(), dealID.PG(),
	).Scan(&currentVersion, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify deal mutation: %w", err)
	}
	if currentVersion != version {
		return errx.ErrVersionConflict
	}
	if deleted != requireDeleted {
		return errx.ErrNotFound
	}
	return nil
}

func (service *Service) upsertDealSearch(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
	name string,
	version int64,
) error {
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: workspaceID.PG(), EntityType: "deal", EntityID: dealID.PG(),
		Title: name, SearchableText: name, RankBoost: 1.0, Version: version,
	}); err != nil {
		return fmt.Errorf("upsert deal search document: %w", err)
	}
	return nil
}

func makeDealRecord(
	id, pipelineID, stageID pgtype.UUID,
	name string,
	contactID, companyID, ownerID pgtype.UUID,
	amountMinor int64,
	currency string,
	plannedStartDate, expectedCloseDate pgtype.Date,
	position int32,
	status string,
	lostReason *string,
	forecastCategory string,
	wonAt, lostAt pgtype.Timestamptz,
	customFields []byte,
	version int64,
	createdAt, updatedAt time.Time,
) DealRecord {
	dealID, _ := ids.FromPG(id)
	pipeline, _ := ids.FromPG(pipelineID)
	stage, _ := ids.FromPG(stageID)
	return DealRecord{
		ID: dealID.String(), PipelineID: pipeline.String(), StageID: stage.String(), Name: name,
		ContactID: optionalIDString(contactID), CompanyID: optionalIDString(companyID), OwnerID: optionalIDString(ownerID),
		AmountMinor: amountMinor, Currency: currency, PlannedStartDate: optionalDateString(plannedStartDate),
		ExpectedCloseDate: optionalDateString(expectedCloseDate),
		Position:          int(position), Status: status, LostReason: lostReason, ForecastCategory: forecastCategory,
		WonAt: optionalTimestamp(wonAt), LostAt: optionalTimestamp(lostAt), CustomFields: decodeCustomFields(customFields),
		Version: version, CreatedAt: createdAt.UTC(), UpdatedAt: updatedAt.UTC(),
	}
}

func makeKanbanCard(row dbgen.ListKanbanStageDealsRow) KanbanCard {
	id, _ := ids.FromPG(row.ID)
	pipelineID, _ := ids.FromPG(row.PipelineID)
	stageID, _ := ids.FromPG(row.StageID)
	return KanbanCard{
		ID: id.String(), PipelineID: pipelineID.String(), StageID: stageID.String(), Name: row.Name,
		ContactID: optionalIDString(row.ContactID), CompanyID: optionalIDString(row.CompanyID), OwnerID: optionalIDString(row.OwnerUserID),
		AmountMinor: row.AmountMinor, Currency: row.Currency, PlannedStartDate: optionalDateString(row.PlannedStartDate),
		ExpectedCloseDate: optionalDateString(row.ExpectedCloseDate),
		Position:          int(row.Position), ForecastCategory: row.ForecastCategory, Version: row.Version, UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func optionalDateString(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}
	result := value.Time.Format("2006-01-02")
	return &result
}

func optionalTimestamp(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
