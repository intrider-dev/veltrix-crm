package customers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	platformworker "github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type ContactImportJobPayload struct {
	ImportSessionID string `json:"importSessionId"`
	ActorUserID     string `json:"actorUserId"`
}

func DecodeContactImportJobPayload(raw json.RawMessage) (ContactImportJobPayload, error) {
	var payload ContactImportJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ContactImportJobPayload{}, errors.New("invalid contact import job payload")
	}
	if _, err := ids.Parse(payload.ImportSessionID); err != nil {
		return ContactImportJobPayload{}, errors.New("invalid contact import session ID")
	}
	if _, err := ids.Parse(payload.ActorUserID); err != nil {
		return ContactImportJobPayload{}, errors.New("invalid contact import actor ID")
	}
	return payload, nil
}

func (service *Service) ContactImportJobHandler(
	ctx context.Context,
	dependencies platformworker.Dependencies,
	job platformworker.Job,
) error {
	payload, err := DecodeContactImportJobPayload(job.Payload)
	if err != nil {
		return err
	}
	return service.ProcessContactImportJob(ctx, dependencies.AppPool, job.WorkspaceID, payload)
}

// ProcessContactImportJob executes one bounded batch and atomically queues a
// continuation when rows remain. A committed batch is never repeated: staged
// row state and the continuation idempotency key form the resume checkpoint.
func (service *Service) ProcessContactImportJob(
	ctx context.Context,
	pool *pgxpool.Pool,
	workspaceID ids.UUID,
	payload ContactImportJobPayload,
) error {
	if pool == nil {
		return errors.New("contact import app pool is required")
	}
	sessionID, _ := ids.Parse(payload.ImportSessionID)
	actorID, _ := ids.Parse(payload.ActorUserID)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin contact import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetActorContext(ctx, actorID.String()); err != nil {
		return fmt.Errorf("set import actor context: %w", err)
	}
	membership, err := queries.GetActiveMembership(ctx, dbgen.GetActiveMembershipParams{
		WorkspaceID: workspaceID.PG(), UserID: actorID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("load import membership: %w", err)
	}
	requestID, err := ids.NewV7()
	if err != nil {
		return err
	}
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: workspaceID.String(), RequestID: requestID.String(),
	}); err != nil {
		return fmt.Errorf("set import tenant context: %w", err)
	}
	workspace := &tenancy.WorkspaceTx{Tx: tx, Queries: queries, Membership: membership}
	metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: actorID, RequestID: requestID.String()}
	if err := service.processContactImportBatch(ctx, workspace, metadata, sessionID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit contact import batch: %w", err)
	}
	return nil
}

type stagedImportRow struct {
	rowNumber int32
	values    map[string]string
}

type preparedImportContact struct {
	rowNumber   int32
	id          ids.UUID
	firstName   string
	lastName    string
	displayName string
	email       *string
	phone       *string
	jobTitle    *string
	companyID   *ids.UUID
	ownerID     *ids.UUID
	status      string
	source      *string
	customJSON  []byte
	errorCode   string
	errorField  string
}

func (service *Service) processContactImportBatch(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	sessionID ids.UUID,
) error {
	var state string
	var mappingJSON []byte
	err := workspace.Tx.QueryRow(ctx, `
		SELECT status, mapping FROM customers.import_sessions
		WHERE workspace_id = $1 AND id = $2 AND actor_user_id = $3
		FOR UPDATE`, metadata.WorkspaceID.PG(), sessionID.PG(), metadata.ActorID.PG()).Scan(&state, &mappingJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock contact import session: %w", err)
	}
	if state == "completed" {
		return nil
	}
	if state != "queued" && state != "running" {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/status", Code: "validation.import.not_queued"}}}
	}
	var mapping ContactImportMapping
	if err := json.Unmarshal(mappingJSON, &mapping); err != nil {
		return fmt.Errorf("decode contact import mapping: %w", err)
	}
	if _, err := workspace.Tx.Exec(ctx, `UPDATE customers.import_sessions
		SET status = 'running', started_at = COALESCE(started_at, now()), updated_at = now()
		WHERE workspace_id = $1 AND id = $2`, metadata.WorkspaceID.PG(), sessionID.PG()); err != nil {
		return fmt.Errorf("start contact import: %w", err)
	}
	rows, err := workspace.Tx.Query(ctx, `
		SELECT row_number, source_values
		FROM customers.import_rows
		WHERE workspace_id = $1 AND import_session_id = $2 AND state = 'pending'
		ORDER BY row_number
		LIMIT $3
		FOR UPDATE SKIP LOCKED`, metadata.WorkspaceID.PG(), sessionID.PG(), ImportProcessingLot)
	if err != nil {
		return fmt.Errorf("claim contact import rows: %w", err)
	}
	staged := make([]stagedImportRow, 0, ImportProcessingLot)
	for rows.Next() {
		var row stagedImportRow
		var raw []byte
		if err := rows.Scan(&row.rowNumber, &raw); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(raw, &row.values); err != nil {
			rows.Close()
			return fmt.Errorf("decode staged import row: %w", err)
		}
		staged = append(staged, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(staged) == 0 {
		return service.finishOrContinueImport(ctx, workspace, metadata, sessionID)
	}
	definitions, err := service.ListCustomFieldDefinitions(ctx, workspace, metadata.WorkspaceID, "contact")
	if err != nil {
		return err
	}
	companies, err := preloadImportCompanies(ctx, workspace, metadata.WorkspaceID, staged, mapping.CompanyName)
	if err != nil {
		return err
	}
	owners, err := preloadImportOwners(ctx, workspace, metadata.WorkspaceID, staged, mapping.OwnerEmail)
	if err != nil {
		return err
	}
	prepared := make([]preparedImportContact, 0, len(staged))
	for _, row := range staged {
		prepared = append(prepared, prepareImportContact(row, mapping, definitions, companies, owners))
	}
	if err := validatePreparedImportUserReferences(ctx, workspace, metadata.WorkspaceID, prepared, definitions); err != nil {
		return err
	}
	if err := service.writePreparedImportContacts(ctx, workspace, metadata, sessionID, prepared, definitions); err != nil {
		return err
	}
	return service.finishOrContinueImport(ctx, workspace, metadata, sessionID)
}

func validatePreparedImportUserReferences(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	contacts []preparedImportContact,
	definitions []CustomFieldDefinition,
) error {
	userReferenceKeys := make(map[string]struct{})
	for _, definition := range definitions {
		if definition.ValueType == "user_reference" {
			userReferenceKeys[definition.FieldKey] = struct{}{}
		}
	}
	if len(userReferenceKeys) == 0 {
		return nil
	}
	referencedByRow := make(map[int32][]ids.UUID)
	unique := make(map[ids.UUID]struct{})
	for _, contact := range contacts {
		if contact.errorCode != "" {
			continue
		}
		values := map[string]json.RawMessage{}
		if json.Unmarshal(contact.customJSON, &values) != nil {
			continue
		}
		for key, raw := range values {
			if _, relevant := userReferenceKeys[key]; !relevant {
				continue
			}
			var rawID string
			if json.Unmarshal(raw, &rawID) != nil {
				continue
			}
			id, _ := ids.Parse(rawID)
			referencedByRow[contact.rowNumber] = append(referencedByRow[contact.rowNumber], id)
			unique[id] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}
	identifiers := make([]ids.UUID, 0, len(unique))
	for id := range unique {
		identifiers = append(identifiers, id)
	}
	rows, err := workspace.Tx.Query(ctx, `SELECT user_id FROM tenancy.memberships
		WHERE workspace_id = $1 AND status = 'active' AND user_id = ANY($2::uuid[])`,
		workspaceID.PG(), pgUUIDs(identifiers))
	if err != nil {
		return fmt.Errorf("validate import user references: %w", err)
	}
	valid := make(map[ids.UUID]struct{}, len(unique))
	for rows.Next() {
		var raw pgtype.UUID
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		id, _ := ids.FromPG(raw)
		valid[id] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range contacts {
		for _, id := range referencedByRow[contacts[index].rowNumber] {
			if _, exists := valid[id]; !exists {
				contacts[index].errorCode = "validation.custom_field.user_reference"
				contacts[index].errorField = "customFields"
				break
			}
		}
	}
	return nil
}

func prepareImportContact(
	row stagedImportRow,
	mapping ContactImportMapping,
	definitions []CustomFieldDefinition,
	companies map[string]ids.UUID,
	owners map[string]ids.UUID,
) preparedImportContact {
	input := ContactInput{
		FirstName: row.values[mapping.FirstName], LastName: row.values[mapping.LastName],
		Email: importOptional(row.values, mapping.Email), Phone: importOptional(row.values, mapping.Phone),
		JobTitle: importOptional(row.values, mapping.JobTitle), Status: importValue(row.values, mapping.Status),
		Source: importOptional(row.values, mapping.Source), Address: map[string]string{}, CustomFields: map[string]any{},
	}
	if company := strings.ToLower(strings.TrimSpace(importValue(row.values, mapping.CompanyName))); company != "" {
		id, exists := companies[company]
		if !exists {
			return preparedImportContact{rowNumber: row.rowNumber, errorCode: "validation.import.company_not_found", errorField: "companyName"}
		}
		input.CompanyID = &id
	}
	if owner := strings.ToLower(strings.TrimSpace(importValue(row.values, mapping.OwnerEmail))); owner != "" {
		id, exists := owners[owner]
		if !exists {
			return preparedImportContact{rowNumber: row.rowNumber, errorCode: "validation.import.owner_not_found", errorField: "ownerEmail"}
		}
		input.OwnerID = &id
	}
	definitionByKey := make(map[string]CustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		definitionByKey[definition.FieldKey] = definition
	}
	for key, header := range mapping.CustomFields {
		value := strings.TrimSpace(row.values[header])
		if value == "" {
			continue
		}
		definition, exists := definitionByKey[key]
		if !exists {
			return preparedImportContact{rowNumber: row.rowNumber, errorCode: "validation.custom_field.unknown", errorField: "customFields." + key}
		}
		converted, err := parseCSVCustomValue(definition, value)
		if err != nil {
			return preparedImportContact{rowNumber: row.rowNumber, errorCode: err.Error(), errorField: "customFields." + key}
		}
		input.CustomFields[key] = converted
	}
	validated, err := validateContact(input)
	if err != nil {
		return importValidationFailure(row.rowNumber, err)
	}
	normalized, err := validateTypedValues(definitions, validated.CustomFields)
	if err != nil {
		return importValidationFailure(row.rowNumber, err)
	}
	customJSON, _ := json.Marshal(normalized)
	id, err := ids.NewV7()
	if err != nil {
		return preparedImportContact{rowNumber: row.rowNumber, errorCode: "internal.id_unavailable"}
	}
	return preparedImportContact{
		rowNumber: row.rowNumber, id: id, firstName: validated.FirstName, lastName: validated.LastName,
		displayName: strings.TrimSpace(validated.FirstName + " " + validated.LastName),
		email:       validated.Email, phone: validated.Phone, jobTitle: validated.JobTitle,
		companyID: validated.CompanyID, ownerID: validated.OwnerID, status: validated.Status,
		source: validated.Source, customJSON: customJSON,
	}
}

func (service *Service) writePreparedImportContacts(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	sessionID ids.UUID,
	contacts []preparedImportContact,
	definitions []CustomFieldDefinition,
) error {
	batch := &pgx.Batch{}
	for _, contact := range contacts {
		if contact.errorCode != "" {
			batch.Queue(`WITH marked AS (
				UPDATE customers.import_rows SET state = 'failed', processed_at = now()
				WHERE workspace_id = $1 AND import_session_id = $2 AND row_number = $3 AND state = 'pending'
				RETURNING 1
			) INSERT INTO customers.import_errors (
				workspace_id, import_session_id, row_number, error_code, field_key
			) SELECT $1, $2, $3, $4, $5 FROM marked ON CONFLICT DO NOTHING`,
				metadata.WorkspaceID.PG(), sessionID.PG(), contact.rowNumber, contact.errorCode, nullableString(contact.errorField))
			continue
		}
		auditID, _ := ids.NewV7()
		outboxID, _ := ids.NewV7()
		sseID, _ := ids.NewV7()
		correlationID, _ := ids.NewV7()
		summary, _ := json.Marshal(map[string]any{"importSessionId": sessionID.String(), "rowNumber": contact.rowNumber})
		payload, _ := json.Marshal(map[string]any{"contactId": contact.id.String(), "version": 1, "importSessionId": sessionID.String()})
		batch.Queue(`
			WITH created AS (
			  INSERT INTO customers.contacts (
			    workspace_id, id, first_name, last_name, display_name, email, email_normalized,
			    phone, phone_normalized, job_title, company_id, owner_user_id, status, source, custom_fields
			  ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			  RETURNING id, version
			), indexed AS (
			  INSERT INTO search.documents (
			    workspace_id, entity_type, entity_id, title, subtitle, searchable_text, rank_boost, version
			  ) SELECT $1, 'contact', id, $5, $6, concat_ws(' ', $5, $6, $8), 1.2, version FROM created
			  ON CONFLICT (workspace_id, entity_type, entity_id) DO UPDATE
			    SET title = EXCLUDED.title, subtitle = EXCLUDED.subtitle, searchable_text = EXCLUDED.searchable_text,
			        rank_boost = EXCLUDED.rank_boost, version = EXCLUDED.version, updated_at = now()
			), field_values AS (
			  INSERT INTO customers.custom_field_values (
			    workspace_id, definition_id, entity_type, entity_id, value, schema_version
			  ) SELECT $1, definition.id, 'contact', $2, entry.value, definition.schema_version
			    FROM jsonb_each($15::jsonb) entry
			    JOIN customers.custom_field_definitions definition
			      ON definition.workspace_id = $1 AND definition.entity_type = 'contact' AND definition.field_key = entry.key
			), audited AS (
			  INSERT INTO audit.events (
			    workspace_id, id, actor_user_id, action, entity_type, entity_id, request_id, summary, user_agent
			  ) VALUES ($1,$16,$17,'contact.created','contact',$2,$18,$19,'import-worker')
			), emitted AS (
			  INSERT INTO platform.outbox_events (
			    workspace_id,id,event_type,schema_version,aggregate_type,aggregate_id,correlation_id,payload
			  ) VALUES ($1,$20,'customers.contact.created',1,'contact',$2,$21,$22)
			), replayed AS (
			  INSERT INTO notifications.sse_events (workspace_id,id,event_type,data,expires_at)
			  VALUES ($1,$23,'customers.contact.created',$22,now() + interval '24 hours')
			)
			UPDATE customers.import_rows SET state = 'created', created_entity_id = $2, processed_at = now()
			WHERE workspace_id = $1 AND import_session_id = $24 AND row_number = $25 AND state = 'pending'`,
			metadata.WorkspaceID.PG(), contact.id.PG(), contact.firstName, contact.lastName, contact.displayName,
			contact.email, normalizeEmail(contact.email), contact.phone, normalizePhone(contact.phone), contact.jobTitle,
			optionalUUID(contact.companyID), optionalUUID(contact.ownerID), contact.status, contact.source, contact.customJSON,
			auditID.PG(), metadata.ActorID.PG(), metadata.RequestID, summary, outboxID.PG(), correlationID.PG(), payload,
			sseID.PG(), sessionID.PG(), contact.rowNumber)
	}
	results := workspace.Tx.SendBatch(ctx, batch)
	defer results.Close()
	for range contacts {
		if _, err := results.Exec(); err != nil {
			return mapConstraintError(err)
		}
	}
	return results.Close()
}

func (service *Service) finishOrContinueImport(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	sessionID ids.UUID,
) error {
	var total, pending, created, failed int
	if err := workspace.Tx.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE state = 'pending'),
		       count(*) FILTER (WHERE state = 'created'), count(*) FILTER (WHERE state = 'failed')
		FROM customers.import_rows WHERE workspace_id = $1 AND import_session_id = $2`,
		metadata.WorkspaceID.PG(), sessionID.PG()).Scan(&total, &pending, &created, &failed); err != nil {
		return fmt.Errorf("count contact import progress: %w", err)
	}
	status := "running"
	if pending == 0 {
		status = "completed"
	}
	if _, err := workspace.Tx.Exec(ctx, `
		UPDATE customers.import_sessions
		SET status = $3, processed_rows = $4, created_rows = $5, error_rows = $6,
		    completed_at = CASE WHEN $3 = 'completed' THEN now() ELSE NULL END, updated_at = now()
		WHERE workspace_id = $1 AND id = $2`, metadata.WorkspaceID.PG(), sessionID.PG(), status, total-pending, created, failed); err != nil {
		return fmt.Errorf("update contact import progress: %w", err)
	}
	if pending > 0 {
		jobID, err := ids.NewV7()
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(ContactImportJobPayload{ImportSessionID: sessionID.String(), ActorUserID: metadata.ActorID.String()})
		if _, err := workspace.Tx.Exec(ctx, `
			INSERT INTO platform.jobs (
			  workspace_id,id,kind,schema_version,idempotency_key,payload,max_attempts
			) VALUES ($1,$2,'customers.import.contacts',1,$3,$4,20)
			ON CONFLICT (workspace_id,kind,idempotency_key) DO NOTHING`, metadata.WorkspaceID.PG(), jobID.PG(),
			sessionID.String()+":"+strconv.Itoa(total-pending), payload); err != nil {
			return fmt.Errorf("enqueue contact import continuation: %w", err)
		}
	}
	eventType := "customers.contact_import.progress"
	action := "contact_import.progressed"
	if status == "completed" {
		eventType = "customers.contact_import.completed"
		action = "contact_import.completed"
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: action, EventType: eventType, AggregateType: "contact_import", AggregateID: sessionID,
		Summary: map[string]any{"totalRows": total, "processedRows": total - pending, "createdRows": created, "errorRows": failed},
		Payload: map[string]any{"importSessionId": sessionID.String(), "status": status, "processedRows": total - pending, "totalRows": total},
	})
}

func preloadImportCompanies(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	rows []stagedImportRow,
	header string,
) (map[string]ids.UUID, error) {
	values := uniqueImportValues(rows, header)
	result := make(map[string]ids.UUID, len(values))
	if len(values) == 0 {
		return result, nil
	}
	dbRows, err := workspace.Tx.Query(ctx, `SELECT lower(name), id FROM customers.companies
		WHERE workspace_id = $1 AND deleted_at IS NULL AND lower(name) = ANY($2::text[])`, workspaceID.PG(), values)
	if err != nil {
		return nil, fmt.Errorf("preload import companies: %w", err)
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var name string
		var raw pgtype.UUID
		if err := dbRows.Scan(&name, &raw); err != nil {
			return nil, err
		}
		id, _ := ids.FromPG(raw)
		result[name] = id
	}
	return result, dbRows.Err()
}

func preloadImportOwners(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	rows []stagedImportRow,
	header string,
) (map[string]ids.UUID, error) {
	values := uniqueImportValues(rows, header)
	result := make(map[string]ids.UUID, len(values))
	if len(values) == 0 {
		return result, nil
	}
	dbRows, err := workspace.Tx.Query(ctx, `
		SELECT user_record.email_normalized, membership.user_id
		FROM tenancy.memberships membership
		JOIN identity.users user_record ON user_record.id = membership.user_id
		WHERE membership.workspace_id = $1 AND membership.status = 'active'
		  AND user_record.email_normalized = ANY($2::text[])`, workspaceID.PG(), values)
	if err != nil {
		return nil, fmt.Errorf("preload import owners: %w", err)
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var email string
		var raw pgtype.UUID
		if err := dbRows.Scan(&email, &raw); err != nil {
			return nil, err
		}
		id, _ := ids.FromPG(raw)
		result[email] = id
	}
	return result, dbRows.Err()
}

func uniqueImportValues(rows []stagedImportRow, header string) []string {
	if header == "" {
		return nil
	}
	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, row := range rows {
		value := strings.ToLower(strings.TrimSpace(row.values[header]))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func parseCSVCustomValue(definition CustomFieldDefinition, value string) (any, error) {
	switch definition.ValueType {
	case "text", "date", "single_select", "user_reference":
		return value, nil
	case "number":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("validation.custom_field.number")
		}
		return parsed, nil
	case "boolean":
		parsed, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return nil, errors.New("validation.custom_field.boolean")
		}
		return parsed, nil
	case "money":
		var money map[string]any
		if json.Unmarshal([]byte(value), &money) != nil {
			return nil, errors.New("validation.custom_field.money")
		}
		return money, nil
	case "multi_select":
		if strings.HasPrefix(value, "[") {
			var values []string
			if json.Unmarshal([]byte(value), &values) != nil {
				return nil, errors.New("validation.custom_field.options")
			}
			return values, nil
		}
		parts := strings.Split(value, ";")
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
		}
		return parts, nil
	default:
		return nil, errors.New("validation.custom_field.invalid")
	}
}

func importValidationFailure(rowNumber int32, err error) preparedImportContact {
	var validation *errx.ValidationError
	if errors.As(err, &validation) && len(validation.Fields) > 0 {
		return preparedImportContact{rowNumber: rowNumber, errorCode: validation.Fields[0].Code, errorField: validation.Fields[0].Pointer}
	}
	return preparedImportContact{rowNumber: rowNumber, errorCode: "validation.import.row_invalid"}
}

func importOptional(values map[string]string, header string) *string {
	value := strings.TrimSpace(importValue(values, header))
	if value == "" {
		return nil
	}
	return &value
}

func importValue(values map[string]string, header string) string {
	if header == "" {
		return ""
	}
	return values[header]
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	if len(value) > 160 {
		value = value[:160]
	}
	return &value
}

var _ = time.UTC
