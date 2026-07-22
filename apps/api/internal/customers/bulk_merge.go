package customers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) BulkAssignContacts(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	records []VersionedID,
	ownerID *ids.UUID,
) (BulkResult, error) {
	if err := validateVersionedRecords(records); err != nil {
		return BulkResult{}, err
	}
	if ownerID != nil {
		var exists bool
		if err := workspace.Tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM tenancy.memberships
			WHERE workspace_id = $1 AND user_id = $2 AND status = 'active'
		)`, metadata.WorkspaceID.PG(), ownerID.PG()).Scan(&exists); err != nil {
			return BulkResult{}, fmt.Errorf("validate bulk owner: %w", err)
		}
		if !exists {
			return BulkResult{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/ownerId", Code: "validation.reference.invalid"}}}
		}
	}
	payload, _ := marshalVersionedRecords(records)
	rows, err := workspace.Tx.Query(ctx, `
		WITH requested AS (
		  SELECT id, version FROM jsonb_to_recordset($2::jsonb) AS value(id uuid, version bigint)
		)
		UPDATE customers.contacts AS contact
		SET owner_user_id = $3, version = contact.version + 1, updated_at = now()
		FROM requested
		WHERE contact.workspace_id = $1 AND contact.id = requested.id
		  AND contact.version = requested.version AND contact.deleted_at IS NULL
		RETURNING contact.id`, metadata.WorkspaceID.PG(), payload, optionalUUID(ownerID))
	if err != nil {
		return BulkResult{}, fmt.Errorf("bulk assign contacts: %w", err)
	}
	updated, err := countRows(rows)
	if err != nil {
		return BulkResult{}, err
	}
	if updated != len(records) {
		return BulkResult{}, classifyBulkConflict(ctx, workspace, metadata.WorkspaceID, records)
	}
	return service.recordBulk(ctx, workspace, metadata, "contacts.assigned", records, map[string]any{"ownerId": optionalIDString(ownerID)})
}

func (service *Service) BulkTagContacts(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	records []VersionedID,
	tagIDs []ids.UUID,
	mode string,
) (BulkResult, error) {
	if err := validateVersionedRecords(records); err != nil {
		return BulkResult{}, err
	}
	if mode != "add" && mode != "remove" {
		return BulkResult{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/mode", Code: "validation.enum"}}}
	}
	if len(tagIDs) < 1 || len(tagIDs) > 100 {
		return BulkResult{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/tagIds", Code: "validation.items.range"}}}
	}
	uniqueTags := uniqueUUIDs(tagIDs)
	pgTags := pgUUIDs(uniqueTags)
	var tagCount int64
	if err := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM customers.tags
		WHERE workspace_id = $1 AND id = ANY($2::uuid[])`, metadata.WorkspaceID.PG(), pgTags).Scan(&tagCount); err != nil {
		return BulkResult{}, fmt.Errorf("validate bulk tags: %w", err)
	}
	if tagCount != int64(len(uniqueTags)) {
		return BulkResult{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/tagIds", Code: "validation.reference.invalid"}}}
	}
	payload, _ := marshalVersionedRecords(records)
	rows, err := workspace.Tx.Query(ctx, `
		WITH requested AS (
		  SELECT id, version FROM jsonb_to_recordset($2::jsonb) AS value(id uuid, version bigint)
		)
		UPDATE customers.contacts AS contact
		SET version = contact.version + 1, updated_at = now()
		FROM requested
		WHERE contact.workspace_id = $1 AND contact.id = requested.id
		  AND contact.version = requested.version AND contact.deleted_at IS NULL
		RETURNING contact.id`, metadata.WorkspaceID.PG(), payload)
	if err != nil {
		return BulkResult{}, fmt.Errorf("lock bulk contact tags: %w", err)
	}
	updated, err := collectRowIDs(rows)
	if err != nil {
		return BulkResult{}, err
	}
	if len(updated) != len(records) {
		return BulkResult{}, classifyBulkConflict(ctx, workspace, metadata.WorkspaceID, records)
	}
	pgContacts := pgUUIDs(updated)
	if mode == "add" {
		_, err = workspace.Tx.Exec(ctx, `
			INSERT INTO customers.contact_tags (workspace_id, contact_id, tag_id)
			SELECT $1, contact_id, tag_id
			FROM unnest($2::uuid[]) AS contact_id
			CROSS JOIN unnest($3::uuid[]) AS tag_id
			ON CONFLICT DO NOTHING`, metadata.WorkspaceID.PG(), pgContacts, pgTags)
	} else {
		_, err = workspace.Tx.Exec(ctx, `
			DELETE FROM customers.contact_tags
			WHERE workspace_id = $1 AND contact_id = ANY($2::uuid[]) AND tag_id = ANY($3::uuid[])`,
			metadata.WorkspaceID.PG(), pgContacts, pgTags)
	}
	if err != nil {
		return BulkResult{}, fmt.Errorf("bulk %s contact tags: %w", mode, err)
	}
	return service.recordBulk(ctx, workspace, metadata, "contacts.tags."+mode, records,
		map[string]any{"mode": mode, "tagIds": uuidStrings(uniqueTags)})
}

func (service *Service) BulkDeleteContacts(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	records []VersionedID,
) (BulkResult, error) {
	if err := validateVersionedRecords(records); err != nil {
		return BulkResult{}, err
	}
	payload, _ := marshalVersionedRecords(records)
	rows, err := workspace.Tx.Query(ctx, `
		WITH requested AS (
		  SELECT id, version FROM jsonb_to_recordset($2::jsonb) AS value(id uuid, version bigint)
		)
		UPDATE customers.contacts AS contact
		SET deleted_at = now(), deleted_by = $3, version = contact.version + 1, updated_at = now()
		FROM requested
		WHERE contact.workspace_id = $1 AND contact.id = requested.id
		  AND contact.version = requested.version AND contact.deleted_at IS NULL
		RETURNING contact.id`, metadata.WorkspaceID.PG(), payload, metadata.ActorID.PG())
	if err != nil {
		return BulkResult{}, fmt.Errorf("bulk delete contacts: %w", err)
	}
	updated, err := collectRowIDs(rows)
	if err != nil {
		return BulkResult{}, err
	}
	if len(updated) != len(records) {
		return BulkResult{}, classifyBulkConflict(ctx, workspace, metadata.WorkspaceID, records)
	}
	if _, err := workspace.Tx.Exec(ctx, `DELETE FROM search.documents
		WHERE workspace_id = $1 AND entity_type = 'contact' AND entity_id = ANY($2::uuid[])`,
		metadata.WorkspaceID.PG(), pgUUIDs(updated)); err != nil {
		return BulkResult{}, fmt.Errorf("remove bulk search documents: %w", err)
	}
	return service.recordBulk(ctx, workspace, metadata, "contacts.deleted", records, map[string]any{"softDelete": true})
}

func (service *Service) ContactDuplicateCandidates(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, contactID ids.UUID,
) ([]DuplicateCandidate, error) {
	rows, err := workspace.Tx.Query(ctx, `
		WITH source AS (
		  SELECT display_name, email_normalized, phone_normalized
		  FROM customers.contacts WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL
		)
		SELECT candidate.id, candidate.display_name, candidate.email, candidate.phone,
		       CASE
		         WHEN source.email_normalized IS NOT NULL AND candidate.email_normalized = source.email_normalized THEN 'email_exact'
		         WHEN source.phone_normalized IS NOT NULL AND candidate.phone_normalized = source.phone_normalized THEN 'phone_exact'
		         ELSE 'name_similar'
		       END AS reason,
		       greatest(
		         CASE WHEN source.email_normalized IS NOT NULL AND candidate.email_normalized = source.email_normalized THEN 1 ELSE 0 END,
		         CASE WHEN source.phone_normalized IS NOT NULL AND candidate.phone_normalized = source.phone_normalized THEN 0.98 ELSE 0 END,
		         similarity(candidate.display_name, source.display_name) * 0.85
		       )::float8 AS score
		FROM source
		JOIN customers.contacts candidate ON candidate.workspace_id = $1
		WHERE candidate.id <> $2 AND candidate.deleted_at IS NULL
		  AND (
		    (source.email_normalized IS NOT NULL AND candidate.email_normalized = source.email_normalized)
		    OR (source.phone_normalized IS NOT NULL AND candidate.phone_normalized = source.phone_normalized)
		    OR similarity(candidate.display_name, source.display_name) >= 0.62
		  )
		ORDER BY score DESC, candidate.updated_at DESC, candidate.id DESC
		LIMIT 20`, workspaceID.PG(), contactID.PG())
	if err != nil {
		return nil, fmt.Errorf("find contact duplicates: %w", err)
	}
	defer rows.Close()
	candidates := make([]DuplicateCandidate, 0)
	for rows.Next() {
		var id pgtype.UUID
		var candidate DuplicateCandidate
		if err := rows.Scan(&id, &candidate.DisplayName, &candidate.Email, &candidate.Phone, &candidate.Reason, &candidate.Score); err != nil {
			return nil, fmt.Errorf("scan contact duplicate: %w", err)
		}
		candidate.ID = idString(id)
		candidate.Score = roundedScore(candidate.Score)
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		var exists bool
		if err := workspace.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM customers.contacts
			WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL)`, workspaceID.PG(), contactID.PG()).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, errx.ErrNotFound
		}
	}
	return candidates, rows.Err()
}

func (service *Service) CompanyDuplicateCandidates(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, companyID ids.UUID,
) ([]DuplicateCandidate, error) {
	rows, err := workspace.Tx.Query(ctx, `
		WITH source AS (
		  SELECT name, domain_normalized FROM customers.companies
		  WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL
		)
		SELECT candidate.id, candidate.name, candidate.domain,
		       CASE WHEN source.domain_normalized IS NOT NULL AND candidate.domain_normalized = source.domain_normalized
		         THEN 'domain_exact' ELSE 'name_similar' END AS reason,
		       greatest(
		         CASE WHEN source.domain_normalized IS NOT NULL AND candidate.domain_normalized = source.domain_normalized THEN 1 ELSE 0 END,
		         similarity(candidate.name, source.name) * 0.9
		       )::float8 AS score
		FROM source
		JOIN customers.companies candidate ON candidate.workspace_id = $1
		WHERE candidate.id <> $2 AND candidate.deleted_at IS NULL
		  AND ((source.domain_normalized IS NOT NULL AND candidate.domain_normalized = source.domain_normalized)
		       OR similarity(candidate.name, source.name) >= 0.62)
		ORDER BY score DESC, candidate.updated_at DESC, candidate.id DESC
		LIMIT 20`, workspaceID.PG(), companyID.PG())
	if err != nil {
		return nil, fmt.Errorf("find company duplicates: %w", err)
	}
	defer rows.Close()
	candidates := make([]DuplicateCandidate, 0)
	for rows.Next() {
		var id pgtype.UUID
		var candidate DuplicateCandidate
		if err := rows.Scan(&id, &candidate.DisplayName, &candidate.Domain, &candidate.Reason, &candidate.Score); err != nil {
			return nil, fmt.Errorf("scan company duplicate: %w", err)
		}
		candidate.ID = idString(id)
		candidate.Score = roundedScore(candidate.Score)
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		var exists bool
		if err := workspace.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM customers.companies
			WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL)`, workspaceID.PG(), companyID.PG()).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, errx.ErrNotFound
		}
	}
	return candidates, rows.Err()
}

func (service *Service) MergeContacts(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input MergeInput,
) (MergeResult, error) {
	if err := validateMergeInput(input); err != nil {
		return MergeResult{}, err
	}
	versions, err := lockMergeRecords(ctx, workspace, "customers.contacts", metadata.WorkspaceID, input.SourceID, input.TargetID)
	if err != nil {
		return MergeResult{}, err
	}
	if versions[input.SourceID] != input.SourceVersion || versions[input.TargetID] != input.TargetVersion {
		return MergeResult{}, errx.ErrVersionConflict
	}
	var targetVersion, sourceVersion int64
	err = workspace.Tx.QueryRow(ctx, `
		WITH source AS (
		  SELECT * FROM customers.contacts WHERE workspace_id = $1 AND id = $2
		)
		UPDATE customers.contacts AS target
		SET email = COALESCE(target.email, source.email),
		    email_normalized = COALESCE(target.email_normalized, source.email_normalized),
		    phone = COALESCE(target.phone, source.phone),
		    phone_normalized = COALESCE(target.phone_normalized, source.phone_normalized),
		    job_title = COALESCE(target.job_title, source.job_title),
		    company_id = COALESCE(target.company_id, source.company_id),
		    owner_user_id = COALESCE(target.owner_user_id, source.owner_user_id),
		    team_id = COALESCE(target.team_id, source.team_id),
		    source = COALESCE(target.source, source.source),
		    address = source.address || target.address,
		    custom_fields = source.custom_fields || target.custom_fields,
		    last_contacted_at = greatest(target.last_contacted_at, source.last_contacted_at),
		    next_activity_at = COALESCE(target.next_activity_at, source.next_activity_at),
		    version = target.version + 1, updated_at = now()
		FROM source
		WHERE target.workspace_id = $1 AND target.id = $3
		RETURNING target.version`, metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()).Scan(&targetVersion)
	if err != nil {
		return MergeResult{}, fmt.Errorf("merge contact fields: %w", err)
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`UPDATE sales.deals SET contact_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND contact_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`UPDATE sales.leads SET converted_contact_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND converted_contact_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`UPDATE activities.activities SET related_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND related_type = 'contact' AND related_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`INSERT INTO customers.contact_tags (workspace_id, contact_id, tag_id)
		  SELECT workspace_id, $3, tag_id FROM customers.contact_tags
		  WHERE workspace_id = $1 AND contact_id = $2 ON CONFLICT DO NOTHING`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`INSERT INTO customers.custom_field_values (workspace_id, definition_id, entity_type, entity_id, value, schema_version)
		  SELECT workspace_id, definition_id, entity_type, $3, value, schema_version
		  FROM customers.custom_field_values
		  WHERE workspace_id = $1 AND entity_type = 'contact' AND entity_id = $2
		  ON CONFLICT DO NOTHING`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`DELETE FROM customers.custom_field_values
		  WHERE workspace_id = $1 AND entity_type = 'contact' AND entity_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG()}},
	}
	for _, statement := range statements {
		if _, err := workspace.Tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return MergeResult{}, fmt.Errorf("repoint contact merge relationship: %w", err)
		}
	}
	if err := workspace.Tx.QueryRow(ctx, `
		UPDATE customers.contacts SET deleted_at = now(), deleted_by = $3,
		  version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 RETURNING version`,
		metadata.WorkspaceID.PG(), input.SourceID.PG(), metadata.ActorID.PG()).Scan(&sourceVersion); err != nil {
		return MergeResult{}, fmt.Errorf("close merged contact: %w", err)
	}
	if err := service.recordMerge(ctx, workspace, metadata, "contact", input, sourceVersion, targetVersion); err != nil {
		return MergeResult{}, err
	}
	target, err := service.GetContact(ctx, workspace, metadata.WorkspaceID, input.TargetID)
	if err != nil {
		return MergeResult{}, err
	}
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "contact", EntityID: input.TargetID.PG(),
		Title: target.DisplayName, Subtitle: target.Email,
		SearchableText: strings.Join(nonEmpty(target.DisplayName, pointerValue(target.Email), pointerValue(target.Phone)), " "),
		RankBoost:      1.2, Version: target.Version,
	}); err != nil {
		return MergeResult{}, err
	}
	if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "contact", EntityID: input.SourceID.PG(),
	}); err != nil {
		return MergeResult{}, err
	}
	return MergeResult{TargetID: input.TargetID.String(), TargetVersion: targetVersion, SourceID: input.SourceID.String(), SourceVersion: sourceVersion}, nil
}

func (service *Service) MergeCompanies(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input MergeInput,
) (MergeResult, error) {
	if err := validateMergeInput(input); err != nil {
		return MergeResult{}, err
	}
	versions, err := lockMergeRecords(ctx, workspace, "customers.companies", metadata.WorkspaceID, input.SourceID, input.TargetID)
	if err != nil {
		return MergeResult{}, err
	}
	if versions[input.SourceID] != input.SourceVersion || versions[input.TargetID] != input.TargetVersion {
		return MergeResult{}, errx.ErrVersionConflict
	}
	var sourceVersion int64
	if err := workspace.Tx.QueryRow(ctx, `
		UPDATE customers.companies SET deleted_at = now(), deleted_by = $3,
		  version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 RETURNING version`,
		metadata.WorkspaceID.PG(), input.SourceID.PG(), metadata.ActorID.PG()).Scan(&sourceVersion); err != nil {
		return MergeResult{}, fmt.Errorf("close merged company: %w", err)
	}
	var targetVersion int64
	err = workspace.Tx.QueryRow(ctx, `
		WITH source AS (
		  SELECT * FROM customers.companies WHERE workspace_id = $1 AND id = $2
		)
		UPDATE customers.companies AS target
		SET domain = COALESCE(target.domain, source.domain),
		    domain_normalized = COALESCE(target.domain_normalized, source.domain_normalized),
		    industry = COALESCE(target.industry, source.industry),
		    owner_user_id = COALESCE(target.owner_user_id, source.owner_user_id),
		    team_id = COALESCE(target.team_id, source.team_id),
		    address = source.address || target.address,
		    custom_fields = source.custom_fields || target.custom_fields,
		    version = target.version + 1, updated_at = now()
		FROM source WHERE target.workspace_id = $1 AND target.id = $3
		RETURNING target.version`, metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()).Scan(&targetVersion)
	if err != nil {
		return MergeResult{}, mapConstraintError(err)
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`UPDATE customers.contacts SET company_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND company_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`UPDATE sales.deals SET company_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND company_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`UPDATE sales.leads SET converted_company_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND converted_company_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`UPDATE activities.activities SET related_id = $3, version = version + 1, updated_at = now()
		  WHERE workspace_id = $1 AND related_type = 'company' AND related_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`INSERT INTO customers.custom_field_values (workspace_id, definition_id, entity_type, entity_id, value, schema_version)
		  SELECT workspace_id, definition_id, entity_type, $3, value, schema_version
		  FROM customers.custom_field_values
		  WHERE workspace_id = $1 AND entity_type = 'company' AND entity_id = $2
		  ON CONFLICT DO NOTHING`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG(), input.TargetID.PG()}},
		{`DELETE FROM customers.custom_field_values
		  WHERE workspace_id = $1 AND entity_type = 'company' AND entity_id = $2`, []any{metadata.WorkspaceID.PG(), input.SourceID.PG()}},
	}
	for _, statement := range statements {
		if _, err := workspace.Tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return MergeResult{}, fmt.Errorf("repoint company merge relationship: %w", err)
		}
	}
	if err := service.recordMerge(ctx, workspace, metadata, "company", input, sourceVersion, targetVersion); err != nil {
		return MergeResult{}, err
	}
	row, err := scanCompany(workspace.Tx.QueryRow(ctx, `
		SELECT id, name, domain, industry, status, owner_user_id, team_id, address,
		       custom_fields, version, created_at, updated_at
		FROM customers.companies WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		metadata.WorkspaceID.PG(), input.TargetID.PG()))
	if err != nil {
		return MergeResult{}, err
	}
	if err := service.indexCompany(ctx, workspace, metadata.WorkspaceID, input.TargetID, row); err != nil {
		return MergeResult{}, err
	}
	if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "company", EntityID: input.SourceID.PG(),
	}); err != nil {
		return MergeResult{}, err
	}
	return MergeResult{TargetID: input.TargetID.String(), TargetVersion: targetVersion, SourceID: input.SourceID.String(), SourceVersion: sourceVersion}, nil
}

func (service *Service) recordMerge(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	entityType string,
	input MergeInput,
	sourceVersion, targetVersion int64,
) error {
	_, err := workspace.Tx.Exec(ctx, `
		INSERT INTO customers.record_merges (
		  workspace_id, entity_type, source_id, target_id, actor_user_id, source_version, target_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`, metadata.WorkspaceID.PG(), entityType,
		input.SourceID.PG(), input.TargetID.PG(), metadata.ActorID.PG(), sourceVersion, targetVersion)
	if err != nil {
		return mapConstraintError(err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: entityType + ".merged", EventType: "customers." + entityType + ".merged",
		AggregateType: entityType, AggregateID: input.TargetID,
		Summary: map[string]any{"sourceId": input.SourceID.String(), "targetId": input.TargetID.String()},
		Payload: map[string]any{"sourceId": input.SourceID.String(), "targetId": input.TargetID.String(), "version": targetVersion},
	})
}

func (service *Service) recordBulk(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	action string,
	records []VersionedID,
	extra map[string]any,
) (BulkResult, error) {
	operationID, err := ids.NewV7()
	if err != nil {
		return BulkResult{}, err
	}
	identifiers := make([]string, 0, len(records))
	for _, record := range records {
		identifiers = append(identifiers, record.ID.String())
	}
	summary := map[string]any{"count": len(records), "recordIds": identifiers}
	for key, value := range extra {
		summary[key] = value
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: action, EventType: "customers." + action, AggregateType: "contact_bulk", AggregateID: operationID,
		Summary: summary, Payload: map[string]any{"operationId": operationID.String(), "count": len(records)},
	}); err != nil {
		return BulkResult{}, err
	}
	return BulkResult{OperationID: operationID.String(), Updated: len(records)}, nil
}

func validateMergeInput(input MergeInput) error {
	if input.SourceID == (ids.UUID{}) || input.TargetID == (ids.UUID{}) || input.SourceID == input.TargetID {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/sourceId", Code: "validation.merge.invalid"}}}
	}
	if input.SourceVersion < 1 || input.TargetVersion < 1 {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/version", Code: "validation.etag.invalid"}}}
	}
	return nil
}

func lockMergeRecords(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	table string,
	workspaceID, sourceID, targetID ids.UUID,
) (map[ids.UUID]int64, error) {
	if table != "customers.contacts" && table != "customers.companies" {
		return nil, errx.ErrNotFound
	}
	rows, err := workspace.Tx.Query(ctx, `SELECT id, version FROM `+table+`
		WHERE workspace_id = $1 AND id = ANY($2::uuid[]) AND deleted_at IS NULL
		ORDER BY id FOR UPDATE`, workspaceID.PG(), pgUUIDs([]ids.UUID{sourceID, targetID}))
	if err != nil {
		return nil, fmt.Errorf("lock merge records: %w", err)
	}
	defer rows.Close()
	versions := make(map[ids.UUID]int64, 2)
	for rows.Next() {
		var id pgtype.UUID
		var version int64
		if err := rows.Scan(&id, &version); err != nil {
			return nil, err
		}
		converted, _ := ids.FromPG(id)
		versions[converted] = version
	}
	if len(versions) != 2 {
		return nil, errx.ErrNotFound
	}
	return versions, rows.Err()
}

func classifyBulkConflict(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, records []VersionedID) error {
	idsOnly := make([]ids.UUID, 0, len(records))
	for _, record := range records {
		idsOnly = append(idsOnly, record.ID)
	}
	var count int64
	if err := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM customers.contacts
		WHERE workspace_id = $1 AND id = ANY($2::uuid[]) AND deleted_at IS NULL`, workspaceID.PG(), pgUUIDs(idsOnly)).Scan(&count); err != nil {
		return fmt.Errorf("classify bulk conflict: %w", err)
	}
	if count != int64(len(records)) {
		return errx.ErrNotFound
	}
	return errx.ErrVersionConflict
}

func marshalVersionedRecords(records []VersionedID) ([]byte, error) {
	values := make([]struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}, 0, len(records))
	for _, record := range records {
		values = append(values, struct {
			ID      string `json:"id"`
			Version int64  `json:"version"`
		}{ID: record.ID.String(), Version: record.Version})
	}
	return json.Marshal(values)
}

func countRows(rows pgx.Rows) (int, error) {
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate bulk results: %w", err)
	}
	return count, nil
}

func collectRowIDs(rows pgx.Rows) ([]ids.UUID, error) {
	defer rows.Close()
	result := make([]ids.UUID, 0)
	for rows.Next() {
		var raw pgtype.UUID
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan bulk result: %w", err)
		}
		id, valid := ids.FromPG(raw)
		if !valid {
			return nil, errors.New("bulk mutation returned invalid UUID")
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func uniqueUUIDs(values []ids.UUID) []ids.UUID {
	seen := make(map[ids.UUID]struct{}, len(values))
	result := make([]ids.UUID, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func pgUUIDs(values []ids.UUID) []pgtype.UUID {
	result := make([]pgtype.UUID, 0, len(values))
	for _, value := range values {
		result = append(result, value.PG())
	}
	return result
}

func optionalIDString(value *ids.UUID) any {
	if value == nil {
		return nil
	}
	return value.String()
}

func roundedScore(value float64) float64 { return math.Round(value*1000) / 1000 }
