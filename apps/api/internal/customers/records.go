package customers

import (
	"context"
	"encoding/json"
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

func (service *Service) ListContactTrash(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	cursor string,
	limit int,
) (DeletedPage, error) {
	limit = boundedPageLimit(limit)
	cursorTime, cursorID, err := pagination.Decode(cursor, "trash=contact")
	if err != nil {
		return DeletedPage{}, invalidCursor()
	}
	rows, err := workspace.Tx.Query(ctx, `
		SELECT id, display_name, email, company_id, owner_user_id, version,
		       deleted_at, deleted_by, created_at, updated_at
		FROM customers.contacts
		WHERE workspace_id = $1 AND deleted_at IS NOT NULL
		  AND (deleted_at, id) < ($2, $3)
		ORDER BY deleted_at DESC, id DESC
		LIMIT $4`, workspaceID.PG(), cursorTime, cursorID.PG(), int32(limit+1))
	if err != nil {
		return DeletedPage{}, fmt.Errorf("list contact trash: %w", err)
	}
	defer rows.Close()
	page := DeletedPage{Items: make([]DeletedRecord, 0, limit)}
	for rows.Next() {
		var id, companyID, ownerID, deletedBy pgtype.UUID
		var name string
		var email *string
		var version int64
		var deletedAt, createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&id, &name, &email, &companyID, &ownerID, &version, &deletedAt, &deletedBy, &createdAt, &updatedAt); err != nil {
			return DeletedPage{}, fmt.Errorf("scan contact trash: %w", err)
		}
		page.Items = append(page.Items, DeletedRecord{
			ID: idString(id), DisplayName: name, Email: email, CompanyID: idStringPointer(companyID),
			OwnerID: idStringPointer(ownerID), Version: version, DeletedAt: deletedAt.Time.UTC(),
			DeletedBy: idStringPointer(deletedBy), CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return DeletedPage{}, fmt.Errorf("iterate contact trash: %w", err)
	}
	return finalizeDeletedPage(page, limit, "trash=contact")
}

func (service *Service) RestoreContact(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	contactID ids.UUID,
	version int64,
) (int64, error) {
	var displayName string
	var email, phone *string
	var newVersion int64
	err := workspace.Tx.QueryRow(ctx, `
		UPDATE customers.contacts
		SET deleted_at = NULL, deleted_by = NULL, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NOT NULL
		RETURNING display_name, email, phone, version`, metadata.WorkspaceID.PG(), contactID.PG(), version).
		Scan(&displayName, &email, &phone, &newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, versionedRecordError(ctx, workspace, "customers.contacts", metadata.WorkspaceID, contactID, true)
	}
	if err != nil {
		return 0, fmt.Errorf("restore contact: %w", err)
	}
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "contact", EntityID: contactID.PG(),
		Title: displayName, Subtitle: email,
		SearchableText: strings.Join(nonEmpty(displayName, pointerValue(email), pointerValue(phone)), " "),
		RankBoost:      1.2, Version: newVersion,
	}); err != nil {
		return 0, fmt.Errorf("restore contact search document: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "contact.restored", EventType: "customers.contact.restored", AggregateType: "contact", AggregateID: contactID,
		Summary: map[string]any{"restored": true}, Payload: map[string]any{"contactId": contactID.String(), "version": newVersion},
	}); err != nil {
		return 0, err
	}
	return newVersion, nil
}

func (service *Service) ListCompanyTrash(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	cursor string,
	limit int,
) (DeletedPage, error) {
	limit = boundedPageLimit(limit)
	cursorTime, cursorID, err := pagination.Decode(cursor, "trash=company")
	if err != nil {
		return DeletedPage{}, invalidCursor()
	}
	rows, err := workspace.Tx.Query(ctx, `
		SELECT id, name, domain, owner_user_id, version, deleted_at, deleted_by, created_at, updated_at
		FROM customers.companies
		WHERE workspace_id = $1 AND deleted_at IS NOT NULL
		  AND (deleted_at, id) < ($2, $3)
		ORDER BY deleted_at DESC, id DESC
		LIMIT $4`, workspaceID.PG(), cursorTime, cursorID.PG(), int32(limit+1))
	if err != nil {
		return DeletedPage{}, fmt.Errorf("list company trash: %w", err)
	}
	defer rows.Close()
	page := DeletedPage{Items: make([]DeletedRecord, 0, limit)}
	for rows.Next() {
		var id, ownerID, deletedBy pgtype.UUID
		var name string
		var domain *string
		var version int64
		var deletedAt, createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&id, &name, &domain, &ownerID, &version, &deletedAt, &deletedBy, &createdAt, &updatedAt); err != nil {
			return DeletedPage{}, fmt.Errorf("scan company trash: %w", err)
		}
		page.Items = append(page.Items, DeletedRecord{
			ID: idString(id), DisplayName: name, Domain: domain, OwnerID: idStringPointer(ownerID), Version: version,
			DeletedAt: deletedAt.Time.UTC(), DeletedBy: idStringPointer(deletedBy),
			CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return DeletedPage{}, fmt.Errorf("iterate company trash: %w", err)
	}
	return finalizeDeletedPage(page, limit, "trash=company")
}

func (service *Service) UpdateCompany(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	companyID ids.UUID,
	version int64,
	input CompanyUpdateInput,
) (CompanyRecord, error) {
	validated, err := validateCompanyUpdate(input)
	if err != nil {
		return CompanyRecord{}, err
	}
	customJSON, values, err := service.prepareCustomFields(ctx, workspace, metadata.WorkspaceID, "company", validated.CustomFields)
	if err != nil {
		return CompanyRecord{}, err
	}
	address, _ := json.Marshal(validated.Address)
	var domainNormalized *string
	if validated.Domain != nil {
		value := strings.ToLower(*validated.Domain)
		validated.Domain = &value
		domainNormalized = &value
	}
	row, err := scanCompany(workspace.Tx.QueryRow(ctx, `
		UPDATE customers.companies
		SET name = $3, domain = $4, domain_normalized = $5, industry = $6,
		    status = $7, owner_user_id = $8, team_id = $9, address = $10,
		    custom_fields = $11, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $12 AND deleted_at IS NULL
		RETURNING id, name, domain, industry, status, owner_user_id, team_id, address,
		          custom_fields, version, created_at, updated_at`,
		metadata.WorkspaceID.PG(), companyID.PG(), validated.Name, validated.Domain, domainNormalized,
		validated.Industry, validated.Status, optionalUUID(validated.OwnerID), optionalUUID(validated.TeamID),
		address, customJSON, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return CompanyRecord{}, versionedRecordError(ctx, workspace, "customers.companies", metadata.WorkspaceID, companyID, false)
	}
	if err != nil {
		return CompanyRecord{}, mapConstraintError(err)
	}
	if err := service.replaceCustomFieldValues(ctx, workspace, metadata.WorkspaceID, "company", companyID, values); err != nil {
		return CompanyRecord{}, err
	}
	if err := service.indexCompany(ctx, workspace, metadata.WorkspaceID, companyID, row); err != nil {
		return CompanyRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "company.updated", EventType: "customers.company.updated", AggregateType: "company", AggregateID: companyID,
		Summary: map[string]any{"fields": []string{"name", "domain", "industry", "status", "ownerId", "teamId", "address", "customFields"}},
		Payload: map[string]any{"companyId": companyID.String(), "version": row.Version},
	}); err != nil {
		return CompanyRecord{}, err
	}
	return row, nil
}

func (service *Service) DeleteCompany(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	companyID ids.UUID,
	version int64,
) error {
	var newVersion int64
	err := workspace.Tx.QueryRow(ctx, `
		UPDATE customers.companies
		SET deleted_at = now(), deleted_by = $3, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $4 AND deleted_at IS NULL
		RETURNING version`, metadata.WorkspaceID.PG(), companyID.PG(), metadata.ActorID.PG(), version).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return versionedRecordError(ctx, workspace, "customers.companies", metadata.WorkspaceID, companyID, false)
	}
	if err != nil {
		return fmt.Errorf("delete company: %w", err)
	}
	if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "company", EntityID: companyID.PG(),
	}); err != nil {
		return fmt.Errorf("delete company search document: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "company.deleted", EventType: "customers.company.deleted", AggregateType: "company", AggregateID: companyID,
		Summary: map[string]any{"softDelete": true}, Payload: map[string]any{"companyId": companyID.String(), "version": newVersion},
	})
}

func (service *Service) RestoreCompany(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	companyID ids.UUID,
	version int64,
) (CompanyRecord, error) {
	row, err := scanCompany(workspace.Tx.QueryRow(ctx, `
		UPDATE customers.companies
		SET deleted_at = NULL, deleted_by = NULL, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NOT NULL
		RETURNING id, name, domain, industry, status, owner_user_id, team_id, address,
		          custom_fields, version, created_at, updated_at`, metadata.WorkspaceID.PG(), companyID.PG(), version))
	if errors.Is(err, pgx.ErrNoRows) {
		return CompanyRecord{}, versionedRecordError(ctx, workspace, "customers.companies", metadata.WorkspaceID, companyID, true)
	}
	if err != nil {
		return CompanyRecord{}, mapConstraintError(err)
	}
	if err := service.indexCompany(ctx, workspace, metadata.WorkspaceID, companyID, row); err != nil {
		return CompanyRecord{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "company.restored", EventType: "customers.company.restored", AggregateType: "company", AggregateID: companyID,
		Summary: map[string]any{"restored": true}, Payload: map[string]any{"companyId": companyID.String(), "version": row.Version},
	}); err != nil {
		return CompanyRecord{}, err
	}
	return row, nil
}

func (service *Service) indexCompany(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, companyID ids.UUID, row CompanyRecord) error {
	return workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: workspaceID.PG(), EntityType: "company", EntityID: companyID.PG(),
		Title: row.Name, Subtitle: row.Domain,
		SearchableText: strings.Join(nonEmpty(row.Name, pointerValue(row.Domain), pointerValue(row.Industry)), " "),
		RankBoost:      1.1, Version: row.Version,
	})
}

type rowScanner interface{ Scan(...any) error }

func scanCompany(row rowScanner) (CompanyRecord, error) {
	var id, ownerID, teamID pgtype.UUID
	var record CompanyRecord
	var address, customFields []byte
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(&id, &record.Name, &record.Domain, &record.Industry, &record.Status, &ownerID, &teamID,
		&address, &customFields, &record.Version, &createdAt, &updatedAt)
	if err != nil {
		return CompanyRecord{}, err
	}
	record.ID = idString(id)
	record.OwnerID = idStringPointer(ownerID)
	record.TeamID = idStringPointer(teamID)
	record.Address = decodeStringMap(address)
	record.CustomFields = decodeAnyMap(customFields)
	record.CreatedAt = createdAt.Time.UTC()
	record.UpdatedAt = updatedAt.Time.UTC()
	return record, nil
}

func boundedPageLimit(limit int) int {
	if limit < 1 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func finalizeDeletedPage(page DeletedPage, limit int, filter string) (DeletedPage, error) {
	if len(page.Items) <= limit {
		return page, nil
	}
	last := page.Items[limit-1]
	id, err := ids.Parse(last.ID)
	if err != nil {
		return DeletedPage{}, err
	}
	page.NextCursor, err = pagination.Encode(last.DeletedAt, id, filter)
	if err != nil {
		return DeletedPage{}, err
	}
	page.Items = page.Items[:limit]
	return page, nil
}

func versionedRecordError(ctx context.Context, workspace *tenancy.WorkspaceTx, table string, workspaceID, recordID ids.UUID, deleted bool) error {
	if table != "customers.contacts" && table != "customers.companies" {
		return errx.ErrNotFound
	}
	query := "SELECT version, deleted_at IS NOT NULL FROM " + table + " WHERE workspace_id = $1 AND id = $2"
	var currentVersion int64
	var isDeleted bool
	err := workspace.Tx.QueryRow(ctx, query, workspaceID.PG(), recordID.PG()).Scan(&currentVersion, &isDeleted)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && isDeleted != deleted) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify versioned record failure: %w", err)
	}
	return errx.ErrVersionConflict
}

func invalidCursor() error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/cursor", Code: "validation.cursor.invalid"}}}
}

func idString(value pgtype.UUID) string {
	id, valid := ids.FromPG(value)
	if !valid {
		return ""
	}
	return id.String()
}

func idStringPointer(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	result := idString(value)
	return &result
}

func decodeStringMap(value []byte) map[string]string {
	result := map[string]string{}
	_ = json.Unmarshal(value, &result)
	return result
}

func decodeAnyMap(value []byte) map[string]any {
	result := map[string]any{}
	_ = json.Unmarshal(value, &result)
	return result
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
