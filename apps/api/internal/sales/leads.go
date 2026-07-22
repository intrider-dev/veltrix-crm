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

func (service *Service) ListLeads(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	filter LeadListFilter,
) (LeadPage, error) {
	validated, err := validateLeadFilter(filter)
	if err != nil {
		return LeadPage{}, err
	}
	fingerprint := strings.Join([]string{
		"q=" + validated.Query,
		"status=" + validated.Status,
		"owner=" + uuidString(validated.OwnerID),
	}, "&")
	cursorTime, cursorID, err := pagination.Decode(validated.Cursor, fingerprint)
	if err != nil {
		return LeadPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListLeadsAdvanced(ctx, dbgen.ListLeadsAdvancedParams{
		WorkspaceID: workspaceID.PG(), SearchQuery: validated.Query, StatusFilter: validated.Status,
		OwnerID: optionalUUID(validated.OwnerID), CursorUpdatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true},
		CursorID: cursorID.PG(), PageLimit: int32(validated.Limit + 1),
	})
	if err != nil {
		return LeadPage{}, fmt.Errorf("list leads: %w", err)
	}
	page := LeadPage{Items: make([]LeadRecord, 0, min(len(rows), validated.Limit))}
	for index, row := range rows {
		if index == validated.Limit {
			break
		}
		page.Items = append(page.Items, makeLeadRecord(
			row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
			row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
			row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
		))
	}
	if len(rows) > validated.Limit {
		last := rows[validated.Limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.UpdatedAt.Time, lastID, fingerprint)
		if err != nil {
			return LeadPage{}, fmt.Errorf("encode lead cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) GetLead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, leadID ids.UUID,
) (LeadRecord, error) {
	row, err := workspace.Queries.GetLeadAdvanced(ctx, dbgen.GetLeadAdvancedParams{
		WorkspaceID: workspaceID.PG(), ID: leadID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadRecord{}, errx.ErrNotFound
	}
	if err != nil {
		return LeadRecord{}, fmt.Errorf("get lead: %w", err)
	}
	return makeLeadRecord(
		row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
		row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
		row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) CreateLead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input LeadInput,
) (LeadRecord, error) {
	validated, emailNormalized, phoneNormalized, customFields, err := validateLeadInput(input)
	if err != nil {
		return LeadRecord{}, err
	}
	leadID, err := ids.NewV7()
	if err != nil {
		return LeadRecord{}, err
	}
	stageID, status, err := service.resolveLeadStage(ctx, workspace, metadata.WorkspaceID, validated.StageID, validated.Status)
	if err != nil {
		return LeadRecord{}, err
	}
	row, err := workspace.Queries.CreateLeadAdvanced(ctx, dbgen.CreateLeadAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: leadID.PG(), Name: validated.Name,
		Email: validated.Email, EmailNormalized: nullableNormalized(emailNormalized),
		Phone: validated.Phone, PhoneNormalized: nullableNormalized(phoneNormalized),
		CompanyName: validated.CompanyName, JobTitle: validated.JobTitle, Source: validated.Source,
		Status: status, StageID: stageID, OwnerUserID: optionalUUID(validated.OwnerID), TeamID: optionalUUID(validated.TeamID),
		CustomFields: customFields,
	})
	if err != nil {
		return LeadRecord{}, mapConstraintError(err)
	}
	if err := service.recordLeadStageHistory(ctx, workspace, metadata, leadID, pgtype.UUID{}, stageID); err != nil {
		return LeadRecord{}, err
	}
	if err := service.upsertLeadSearch(ctx, workspace, metadata.WorkspaceID, leadID, row.Name, row.Email, row.CompanyName, row.Version); err != nil {
		return LeadRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead.created", EventType: "sales.lead.created", AggregateType: "lead", AggregateID: leadID,
		Summary: map[string]any{"fields": []string{"name", "email", "phone", "companyName", "status", "ownerId", "teamId"}},
		Payload: map[string]any{"leadId": leadID.String(), "version": row.Version},
	}); err != nil {
		return LeadRecord{}, err
	}
	return makeLeadRecord(
		row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
		row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
		row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) UpdateLead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	leadID ids.UUID,
	version int64,
	input LeadInput,
) (LeadRecord, error) {
	validated, emailNormalized, phoneNormalized, customFields, err := validateLeadInput(input)
	if err != nil {
		return LeadRecord{}, err
	}
	if err := service.requireLeadVersion(ctx, workspace, metadata.WorkspaceID, leadID, version, false); err != nil {
		return LeadRecord{}, err
	}
	existing, err := service.GetLead(ctx, workspace, metadata.WorkspaceID, leadID)
	if err != nil {
		return LeadRecord{}, err
	}
	if validated.Status != existing.Status || (validated.StageID != nil && validated.StageID.String() != existing.StageID) {
		return LeadRecord{}, validation("/stageId", "validation.lead.use_stage_move")
	}
	stageID := ids.MustParse(existing.StageID).PG()
	row, err := workspace.Queries.UpdateLeadAdvanced(ctx, dbgen.UpdateLeadAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: leadID.PG(), Name: validated.Name,
		Email: validated.Email, EmailNormalized: nullableNormalized(emailNormalized),
		Phone: validated.Phone, PhoneNormalized: nullableNormalized(phoneNormalized),
		CompanyName: validated.CompanyName, JobTitle: validated.JobTitle, Source: validated.Source,
		Status: existing.Status, StageID: stageID, OwnerUserID: optionalUUID(validated.OwnerID), TeamID: optionalUUID(validated.TeamID),
		CustomFields: customFields, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadRecord{}, service.classifyLeadMutation(ctx, workspace, metadata.WorkspaceID, leadID, version, false)
	}
	if err != nil {
		return LeadRecord{}, mapConstraintError(err)
	}
	if err := service.upsertLeadSearch(ctx, workspace, metadata.WorkspaceID, leadID, row.Name, row.Email, row.CompanyName, row.Version); err != nil {
		return LeadRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead.updated", EventType: "sales.lead.updated", AggregateType: "lead", AggregateID: leadID,
		Summary: map[string]any{"fields": []string{"name", "email", "phone", "companyName", "jobTitle", "source", "status", "ownerId", "teamId", "customFields"}},
		Payload: map[string]any{"leadId": leadID.String(), "version": row.Version},
	}); err != nil {
		return LeadRecord{}, err
	}
	return makeLeadRecord(
		row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
		row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
		row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) DeleteLead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	leadID ids.UUID,
	version int64,
) error {
	if err := service.requireLeadVersion(ctx, workspace, metadata.WorkspaceID, leadID, version, false); err != nil {
		return err
	}
	newVersion, err := workspace.Queries.SoftDeleteLead(ctx, dbgen.SoftDeleteLeadParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: leadID.PG(), DeletedBy: metadata.ActorID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return service.classifyLeadMutation(ctx, workspace, metadata.WorkspaceID, leadID, version, false)
	}
	if err != nil {
		return fmt.Errorf("delete lead: %w", err)
	}
	if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "lead", EntityID: leadID.PG(),
	}); err != nil {
		return fmt.Errorf("delete lead search document: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead.deleted", EventType: "sales.lead.deleted", AggregateType: "lead", AggregateID: leadID,
		Summary: map[string]any{"softDelete": true}, Payload: map[string]any{"leadId": leadID.String(), "version": newVersion},
	})
}

func (service *Service) RestoreLead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	leadID ids.UUID,
	version int64,
) (LeadRecord, error) {
	if err := service.requireLeadVersion(ctx, workspace, metadata.WorkspaceID, leadID, version, true); err != nil {
		return LeadRecord{}, err
	}
	row, err := workspace.Queries.RestoreLead(ctx, dbgen.RestoreLeadParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: leadID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadRecord{}, service.classifyLeadMutation(ctx, workspace, metadata.WorkspaceID, leadID, version, true)
	}
	if err != nil {
		return LeadRecord{}, fmt.Errorf("restore lead: %w", err)
	}
	if err := service.upsertLeadSearch(ctx, workspace, metadata.WorkspaceID, leadID, row.Name, row.Email, row.CompanyName, row.Version); err != nil {
		return LeadRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead.restored", EventType: "sales.lead.restored", AggregateType: "lead", AggregateID: leadID,
		Summary: map[string]any{"restored": true}, Payload: map[string]any{"leadId": leadID.String(), "version": row.Version},
	}); err != nil {
		return LeadRecord{}, err
	}
	return makeLeadRecord(
		row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
		row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
		row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) ListLeadTrash(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	cursor string,
	limit int,
) (DeletedSalesPage, error) {
	limit = clampPageSize(limit, DefaultPageSize, MaxPageSize)
	cursorTime, cursorID, err := pagination.Decode(cursor, "lead-trash")
	if err != nil {
		return DeletedSalesPage{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListLeadTrash(ctx, dbgen.ListLeadTrashParams{
		WorkspaceID: workspaceID.PG(), CursorDeletedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true},
		CursorID: cursorID.PG(), PageLimit: int32(limit + 1),
	})
	if err != nil {
		return DeletedSalesPage{}, fmt.Errorf("list lead trash: %w", err)
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
		page.NextCursor, err = pagination.Encode(last.DeletedAt.Time, lastID, "lead-trash")
		if err != nil {
			return DeletedSalesPage{}, fmt.Errorf("encode lead trash cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) ConvertLead(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	leadID ids.UUID,
	version int64,
	references LeadConversionReferences,
) (LeadRecord, error) {
	if references.ContactID == nil {
		return LeadRecord{}, validation("/contactId", "validation.required")
	}
	if err := service.requireLeadVersion(ctx, workspace, metadata.WorkspaceID, leadID, version, false); err != nil {
		return LeadRecord{}, err
	}
	existing, err := service.GetLead(ctx, workspace, metadata.WorkspaceID, leadID)
	if err != nil {
		return LeadRecord{}, err
	}
	row, err := workspace.Queries.MarkLeadConverted(ctx, dbgen.MarkLeadConvertedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: leadID.PG(),
		ConvertedContactID: optionalUUID(references.ContactID), ConvertedCompanyID: optionalUUID(references.CompanyID),
		ConvertedDealID: optionalUUID(references.DealID), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadRecord{}, service.classifyLeadMutation(ctx, workspace, metadata.WorkspaceID, leadID, version, false)
	}
	if err != nil {
		return LeadRecord{}, mapConstraintError(err)
	}
	if err := service.recordLeadStageHistory(ctx, workspace, metadata, leadID,
		ids.MustParse(existing.StageID).PG(), row.StageID); err != nil {
		return LeadRecord{}, err
	}
	if err := service.upsertLeadSearch(ctx, workspace, metadata.WorkspaceID, leadID, row.Name, row.Email, row.CompanyName, row.Version); err != nil {
		return LeadRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead.converted", EventType: "sales.lead.converted", AggregateType: "lead", AggregateID: leadID,
		Summary: map[string]any{"contactId": references.ContactID.String(), "companyId": uuidString(references.CompanyID), "dealId": uuidString(references.DealID)},
		Payload: map[string]any{"leadId": leadID.String(), "contactId": references.ContactID.String(), "companyId": uuidString(references.CompanyID), "dealId": uuidString(references.DealID), "version": row.Version},
	}); err != nil {
		return LeadRecord{}, err
	}
	return makeLeadRecord(
		row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
		row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
		row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) MoveLeadStage(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	leadID, targetStageID ids.UUID,
	version int64,
) (LeadRecord, error) {
	if err := service.requireLeadVersion(ctx, workspace, metadata.WorkspaceID, leadID, version, false); err != nil {
		return LeadRecord{}, err
	}
	existing, err := service.GetLead(ctx, workspace, metadata.WorkspaceID, leadID)
	if err != nil {
		return LeadRecord{}, err
	}
	if existing.StageID == targetStageID.String() {
		return existing, nil
	}
	stageID, _, err := service.resolveLeadStage(ctx, workspace, metadata.WorkspaceID, &targetStageID, "")
	if err != nil {
		return LeadRecord{}, err
	}
	row, err := workspace.Queries.MoveLeadToStage(ctx, dbgen.MoveLeadToStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), LeadID: leadID.PG(), Version: version, StageID: stageID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadRecord{}, service.classifyLeadMutation(ctx, workspace, metadata.WorkspaceID, leadID, version, false)
	}
	if err != nil {
		return LeadRecord{}, mapConstraintError(err)
	}
	if err := service.recordLeadStageHistory(ctx, workspace, metadata, leadID,
		ids.MustParse(existing.StageID).PG(), stageID); err != nil {
		return LeadRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead.stage_changed", EventType: "sales.lead.stage_changed",
		AggregateType: "lead", AggregateID: leadID,
		Summary: map[string]any{"fromStageId": existing.StageID, "toStageId": targetStageID.String()},
		Payload: map[string]any{"leadId": leadID.String(), "stageId": targetStageID.String(), "version": row.Version},
	}); err != nil {
		return LeadRecord{}, err
	}
	return makeLeadRecord(
		row.ID, row.Name, row.Email, row.Phone, row.CompanyName, row.JobTitle, row.Source,
		row.Status, row.StageID, row.OwnerUserID, row.TeamID, row.ConvertedContactID, row.ConvertedCompanyID,
		row.ConvertedDealID, row.CustomFields, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
	), nil
}

func (service *Service) resolveLeadStage(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	requested *ids.UUID,
	expectedCategory string,
) (pgtype.UUID, string, error) {
	if requested == nil {
		row, err := workspace.Queries.GetDefaultLeadStageByCategory(ctx, dbgen.GetDefaultLeadStageByCategoryParams{
			WorkspaceID: workspaceID.PG(), Category: expectedCategory,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, "", validation("/status", "validation.reference.invalid")
		}
		if err != nil {
			return pgtype.UUID{}, "", fmt.Errorf("resolve default lead stage: %w", err)
		}
		return row.ID, row.Category, nil
	}
	row, err := workspace.Queries.GetLeadStage(ctx, dbgen.GetLeadStageParams{
		WorkspaceID: workspaceID.PG(), ID: requested.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, "", validation("/stageId", "validation.reference.invalid")
	}
	if err != nil {
		return pgtype.UUID{}, "", fmt.Errorf("resolve lead stage: %w", err)
	}
	if row.Category == "converted" {
		return pgtype.UUID{}, "", validation("/stageId", "validation.lead.convert_required")
	}
	if expectedCategory != "" && row.Category != expectedCategory {
		return pgtype.UUID{}, "", validation("/stageId", "validation.lead.stage_category")
	}
	return row.ID, row.Category, nil
}

func (service *Service) recordLeadStageHistory(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	leadID ids.UUID,
	fromStageID, toStageID pgtype.UUID,
) error {
	historyID, err := ids.NewV7()
	if err != nil {
		return err
	}
	if err := workspace.Queries.InsertLeadStageHistory(ctx, dbgen.InsertLeadStageHistoryParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: historyID.PG(), LeadID: leadID.PG(),
		FromStageID: fromStageID, ToStageID: toStageID, ChangedBy: metadata.ActorID.PG(),
	}); err != nil {
		return fmt.Errorf("record lead stage history: %w", err)
	}
	return nil
}

func (service *Service) requireLeadVersion(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, leadID ids.UUID,
	version int64,
	requireDeleted bool,
) error {
	if version < 1 {
		return validation("/headers/If-Match", "validation.etag.invalid")
	}
	return service.classifyLeadMutation(ctx, workspace, workspaceID, leadID, version, requireDeleted)
}

func (service *Service) classifyLeadMutation(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, leadID ids.UUID,
	version int64,
	requireDeleted bool,
) error {
	var currentVersion int64
	var deleted bool
	var status string
	err := workspace.Tx.QueryRow(ctx, `
		SELECT version, deleted_at IS NOT NULL, status
		FROM sales.leads WHERE workspace_id = $1 AND id = $2`,
		workspaceID.PG(), leadID.PG(),
	).Scan(&currentVersion, &deleted, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify lead mutation: %w", err)
	}
	if currentVersion != version {
		return errx.ErrVersionConflict
	}
	if deleted != requireDeleted {
		return errx.ErrNotFound
	}
	if !requireDeleted && status == "converted" {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/status", Code: "validation.lead.already_converted"}}}
	}
	return nil
}

func (service *Service) upsertLeadSearch(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, leadID ids.UUID,
	name string,
	email, companyName *string,
	version int64,
) error {
	searchable := strings.Join(nonEmpty(name, dereference(email), dereference(companyName)), " ")
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: workspaceID.PG(), EntityType: "lead", EntityID: leadID.PG(), Title: name,
		Subtitle: companyName, SearchableText: searchable, RankBoost: 1.0, Version: version,
	}); err != nil {
		return fmt.Errorf("upsert lead search document: %w", err)
	}
	return nil
}

func makeLeadRecord(
	id pgtype.UUID,
	name string,
	email, phone, companyName, jobTitle, source *string,
	status string,
	stageID, ownerID, teamID, contactID, companyID, dealID pgtype.UUID,
	customFields []byte,
	version int64,
	createdAt, updatedAt time.Time,
) LeadRecord {
	leadID, _ := ids.FromPG(id)
	return LeadRecord{
		ID: leadID.String(), Name: name, Email: email, Phone: phone, CompanyName: companyName,
		JobTitle: jobTitle, Source: source, Status: status, StageID: stageIDString(stageID), OwnerID: optionalIDString(ownerID),
		TeamID: optionalIDString(teamID), ConvertedContactID: optionalIDString(contactID),
		ConvertedCompanyID: optionalIDString(companyID), ConvertedDealID: optionalIDString(dealID),
		CustomFields: decodeCustomFields(customFields), Version: version,
		CreatedAt: createdAt.UTC(), UpdatedAt: updatedAt.UTC(),
	}
}

func optionalIDString(value pgtype.UUID) *string {
	id, ok := ids.FromPG(value)
	if !ok {
		return nil
	}
	result := id.String()
	return &result
}

func stageIDString(value pgtype.UUID) string {
	id, _ := ids.FromPG(value)
	return id.String()
}

func nullableNormalized(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
