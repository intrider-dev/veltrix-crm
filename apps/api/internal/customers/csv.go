package customers

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

var exportHeaders = []string{
	"id", "first_name", "last_name", "display_name", "email", "phone", "job_title",
	"company_name", "status", "source", "owner_id", "tags", "custom_fields", "created_at", "updated_at",
}

func ValidateContactExportFilter(filter ContactExportFilter) error {
	_, err := validateExportFilter(filter)
	return err
}

func (service *Service) ExportContactsCSV(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	filter ContactExportFilter,
	destination io.Writer,
) error {
	validated, err := validateExportFilter(filter)
	if err != nil {
		return err
	}
	order := "contact.updated_at DESC, contact.id DESC"
	if validated.Sort == "name" {
		order = "contact.display_name " + strings.ToUpper(validated.Order) + ", contact.id " + strings.ToUpper(validated.Order)
	} else if validated.Order == "asc" {
		order = "contact.updated_at ASC, contact.id ASC"
	}
	query := `
		SELECT contact.id, contact.first_name, contact.last_name, contact.display_name,
		       contact.email, contact.phone, contact.job_title, company.name, contact.status,
		       contact.source, contact.owner_user_id,
		       COALESCE((
		         SELECT string_agg(tag.name, ';' ORDER BY lower(tag.name), tag.id)
		         FROM customers.contact_tags relation
		         JOIN customers.tags tag ON tag.workspace_id = relation.workspace_id AND tag.id = relation.tag_id
		         WHERE relation.workspace_id = contact.workspace_id AND relation.contact_id = contact.id
		       ), '') AS tags,
		       contact.custom_fields::text, contact.created_at, contact.updated_at
		FROM customers.contacts contact
		LEFT JOIN customers.companies company
		  ON company.workspace_id = contact.workspace_id AND company.id = contact.company_id AND company.deleted_at IS NULL
		WHERE contact.workspace_id = $1 AND contact.deleted_at IS NULL
		  AND ($2 = '' OR contact.display_name ILIKE '%' || $2 || '%'
		       OR contact.email_normalized LIKE '%' || lower($2) || '%')
		  AND ($3 = '' OR contact.status = $3)
		  AND ($4::uuid IS NULL OR contact.owner_user_id = $4)
		  AND (cardinality($5::uuid[]) = 0 OR EXISTS (
		    SELECT 1 FROM customers.contact_tags relation
		    WHERE relation.workspace_id = contact.workspace_id AND relation.contact_id = contact.id
		      AND relation.tag_id = ANY($5::uuid[])
		  ))
		ORDER BY ` + order
	rows, err := workspace.Tx.Query(ctx, query, workspaceID.PG(), validated.Query, validated.Status,
		optionalUUID(validated.OwnerID), pgUUIDs(validated.TagIDs))
	if err != nil {
		return fmt.Errorf("export contacts: %w", err)
	}
	defer rows.Close()
	writer := csv.NewWriter(destination)
	if err := writer.Write(exportHeaders); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for rows.Next() {
		var id, ownerID pgtype.UUID
		var firstName, lastName, displayName, status, tags, customFields string
		var email, phone, jobTitle, companyName, source *string
		var createdAt, updatedAt pgtype.Timestamptz
		if err := rows.Scan(&id, &firstName, &lastName, &displayName, &email, &phone, &jobTitle,
			&companyName, &status, &source, &ownerID, &tags, &customFields, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scan contact export: %w", err)
		}
		record := []string{
			idString(id), firstName, lastName, displayName, pointerValue(email), pointerValue(phone),
			pointerValue(jobTitle), pointerValue(companyName), status, pointerValue(source),
			valueOrEmpty(idStringPointer(ownerID)), tags, customFields,
			createdAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			updatedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		}
		for index := range record {
			record[index] = spreadsheetSafe(record[index])
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write contact export row: %w", err)
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("flush contact export: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate contact export: %w", err)
	}
	writer.Flush()
	return writer.Error()
}

func (service *Service) StageContactCSV(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	source io.Reader,
) (ImportPreview, error) {
	limited := &io.LimitedReader{R: bufio.NewReaderSize(source, 64<<10), N: MaxCSVBytes + 1}
	reader := csv.NewReader(limited)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	reader.TrimLeadingSpace = false
	headers, err := readOwnedCSVRecord(reader)
	if errors.Is(err, io.EOF) {
		return ImportPreview{}, csvValidation("/file", "validation.csv.empty", nil)
	}
	if err != nil {
		return ImportPreview{}, csvValidation("/file", "validation.csv.invalid", nil)
	}
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}
	if err := validateCSVHeaders(headers); err != nil {
		return ImportPreview{}, err
	}
	sessionID, err := ids.NewV7()
	if err != nil {
		return ImportPreview{}, err
	}
	headerJSON, _ := json.Marshal(headers)
	if _, err := workspace.Tx.Exec(ctx, `
		INSERT INTO customers.import_sessions (
		  workspace_id, id, actor_user_id, entity_type, status, source_headers
		) VALUES ($1, $2, $3, 'contact', 'preview', $4)`,
		metadata.WorkspaceID.PG(), sessionID.PG(), metadata.ActorID.PG(), headerJSON); err != nil {
		return ImportPreview{}, fmt.Errorf("create contact import session: %w", err)
	}
	preview := ImportPreview{
		ID: sessionID.String(), EntityType: "contact", Headers: append([]string(nil), headers...),
		SampleRows: make([]map[string]string, 0, ImportPreviewRows), Status: "preview",
		SuggestedMapping: suggestedContactMapping(headers),
	}
	errorRows := 0
	batch := &pgx.Batch{}
	queued := 0
	flush := func() error {
		if queued == 0 {
			return nil
		}
		results := workspace.Tx.SendBatch(ctx, batch)
		defer results.Close()
		for index := 0; index < queued; index++ {
			if _, err := results.Exec(); err != nil {
				return err
			}
		}
		if err := results.Close(); err != nil {
			return err
		}
		batch = &pgx.Batch{}
		queued = 0
		return nil
	}
	for rowNumber := 2; ; rowNumber++ {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return ImportPreview{}, csvValidation("/file", "validation.csv.invalid", map[string]any{"row": rowNumber})
		}
		if preview.TotalRows >= MaxCSVRows {
			return ImportPreview{}, csvValidation("/file", "validation.csv.too_many_rows", map[string]any{"max": MaxCSVRows})
		}
		preview.TotalRows++
		row := make(map[string]string, len(headers))
		state := "pending"
		errorCode := ""
		if len(record) != len(headers) {
			state = "failed"
			errorCode = "validation.csv.column_count"
			errorRows++
		}
		for index := 0; index < min(len(record), len(headers)); index++ {
			if len(record[index]) > MaxCSVFieldBytes || !utf8.ValidString(record[index]) {
				state = "failed"
				errorCode = "validation.csv.field_invalid"
			}
			row[headers[index]] = record[index]
		}
		if state == "failed" && errorCode == "validation.csv.field_invalid" {
			errorRows++
		}
		if len(preview.SampleRows) < ImportPreviewRows {
			preview.SampleRows = append(preview.SampleRows, cloneStringMap(row))
		}
		rowJSON, _ := json.Marshal(row)
		batch.Queue(`INSERT INTO customers.import_rows (
			workspace_id, import_session_id, row_number, source_values, state, processed_at
		) VALUES ($1, $2, $3, $4, $5, CASE WHEN $5 = 'failed' THEN now() ELSE NULL END)`,
			metadata.WorkspaceID.PG(), sessionID.PG(), rowNumber, rowJSON, state)
		queued++
		if errorCode != "" {
			batch.Queue(`INSERT INTO customers.import_errors (
				workspace_id, import_session_id, row_number, error_code
			) VALUES ($1, $2, $3, $4)`, metadata.WorkspaceID.PG(), sessionID.PG(), rowNumber, errorCode)
			queued++
		}
		if queued >= ImportProcessingLot {
			if err := flush(); err != nil {
				return ImportPreview{}, fmt.Errorf("stage contact import rows: %w", err)
			}
		}
	}
	if limited.N <= 0 {
		return ImportPreview{}, csvValidation("/file", "validation.body.too_large", map[string]any{"maxBytes": MaxCSVBytes})
	}
	if err := flush(); err != nil {
		return ImportPreview{}, fmt.Errorf("stage contact import rows: %w", err)
	}
	if preview.TotalRows == 0 {
		return ImportPreview{}, csvValidation("/file", "validation.csv.no_data", nil)
	}
	if _, err := workspace.Tx.Exec(ctx, `
		UPDATE customers.import_sessions
		SET total_rows = $3, error_rows = $4, updated_at = now()
		WHERE workspace_id = $1 AND id = $2`, metadata.WorkspaceID.PG(), sessionID.PG(), preview.TotalRows, errorRows); err != nil {
		return ImportPreview{}, fmt.Errorf("finalize contact import preview: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "contact_import.previewed", EventType: "customers.contact_import.previewed",
		AggregateType: "contact_import", AggregateID: sessionID,
		Summary: map[string]any{"rows": preview.TotalRows, "columns": len(headers), "invalidRows": errorRows},
		Payload: map[string]any{"importSessionId": sessionID.String(), "status": "preview"},
	}); err != nil {
		return ImportPreview{}, err
	}
	return preview, nil
}

// readOwnedCSVRecord detaches a record from csv.Reader's reusable buffer.
// StageContactCSV enables ReuseRecord to keep large imports allocation-aware,
// so retaining the slice returned by Read would otherwise let the next row
// overwrite the header values used as source-value map keys.
func readOwnedCSVRecord(reader *csv.Reader) ([]string, error) {
	record, err := reader.Read()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), record...), nil
}

func (service *Service) QueueContactImport(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	sessionID ids.UUID,
	mapping ContactImportMapping,
) (ImportStatus, error) {
	status, headers, err := service.lockImportSession(ctx, workspace, metadata.WorkspaceID, metadata.ActorID, sessionID)
	if err != nil {
		return ImportStatus{}, err
	}
	if status.Status != "preview" {
		return ImportStatus{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/status", Code: "validation.import.not_preview"}}}
	}
	if err := validateImportMapping(mapping, headers); err != nil {
		return ImportStatus{}, err
	}
	mappingJSON, _ := json.Marshal(mapping)
	if _, err := workspace.Tx.Exec(ctx, `
		UPDATE customers.import_sessions SET mapping = $3, status = 'queued', updated_at = now()
		WHERE workspace_id = $1 AND id = $2`, metadata.WorkspaceID.PG(), sessionID.PG(), mappingJSON); err != nil {
		return ImportStatus{}, fmt.Errorf("queue contact import: %w", err)
	}
	jobID, err := ids.NewV7()
	if err != nil {
		return ImportStatus{}, err
	}
	payload, _ := json.Marshal(map[string]string{"importSessionId": sessionID.String(), "actorUserId": metadata.ActorID.String()})
	if _, err := workspace.Tx.Exec(ctx, `
		INSERT INTO platform.jobs (
		  workspace_id, id, kind, schema_version, idempotency_key, payload, max_attempts
		) VALUES ($1, $2, 'customers.import.contacts', 1, $3, $4, 20)
		ON CONFLICT (workspace_id, kind, idempotency_key) DO NOTHING`,
		metadata.WorkspaceID.PG(), jobID.PG(), sessionID.String()+":0", payload); err != nil {
		return ImportStatus{}, fmt.Errorf("enqueue contact import: %w", err)
	}
	status.Status = "queued"
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "contact_import.queued", EventType: "customers.contact_import.queued",
		AggregateType: "contact_import", AggregateID: sessionID,
		Summary: map[string]any{"rows": status.TotalRows},
		Payload: map[string]any{"importSessionId": sessionID.String(), "status": "queued"},
	}); err != nil {
		return ImportStatus{}, err
	}
	return status, nil
}

func (service *Service) GetImportStatus(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID, sessionID ids.UUID,
) (ImportStatus, error) {
	status, _, err := service.loadImportSession(ctx, workspace, workspaceID, actorID, sessionID, false)
	return status, err
}

func (service *Service) WriteImportErrorsCSV(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID, sessionID ids.UUID,
	destination io.Writer,
) error {
	if _, _, err := service.loadImportSession(ctx, workspace, workspaceID, actorID, sessionID, false); err != nil {
		return err
	}
	rows, err := workspace.Tx.Query(ctx, `
		SELECT row_number, error_code, field_key, safe_value
		FROM customers.import_errors
		WHERE workspace_id = $1 AND import_session_id = $2
		ORDER BY row_number, error_code`, workspaceID.PG(), sessionID.PG())
	if err != nil {
		return fmt.Errorf("list import errors: %w", err)
	}
	defer rows.Close()
	writer := csv.NewWriter(destination)
	if err := writer.Write([]string{"row_number", "error_code", "field_key", "safe_value"}); err != nil {
		return err
	}
	for rows.Next() {
		var rowNumber int32
		var code string
		var field, value *string
		if err := rows.Scan(&rowNumber, &code, &field, &value); err != nil {
			return err
		}
		if err := writer.Write([]string{strconv.Itoa(int(rowNumber)), code, pointerValue(field), spreadsheetSafe(pointerValue(value))}); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return rows.Err()
}

func (service *Service) lockImportSession(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID, sessionID ids.UUID,
) (ImportStatus, []string, error) {
	return service.loadImportSession(ctx, workspace, workspaceID, actorID, sessionID, true)
}

func (service *Service) loadImportSession(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID, sessionID ids.UUID,
	lock bool,
) (ImportStatus, []string, error) {
	query := `SELECT id, entity_type, status, source_headers, total_rows, processed_rows,
		created_rows, error_rows, created_at, updated_at, started_at, completed_at
		FROM customers.import_sessions
		WHERE workspace_id = $1 AND id = $2 AND (actor_user_id = $3 OR $4::boolean)`
	if lock {
		query += " FOR UPDATE"
	}
	isAdmin := workspace.Membership.Role == "owner" || workspace.Membership.Role == "admin"
	var id pgtype.UUID
	var result ImportStatus
	var headersJSON []byte
	var createdAt, updatedAt, startedAt, completedAt pgtype.Timestamptz
	err := workspace.Tx.QueryRow(ctx, query, workspaceID.PG(), sessionID.PG(), actorID.PG(), isAdmin).Scan(
		&id, &result.EntityType, &result.Status, &headersJSON, &result.TotalRows, &result.ProcessedRows,
		&result.CreatedRows, &result.ErrorRows, &createdAt, &updatedAt, &startedAt, &completedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImportStatus{}, nil, errx.ErrNotFound
	}
	if err != nil {
		return ImportStatus{}, nil, fmt.Errorf("load import session: %w", err)
	}
	result.ID = idString(id)
	result.CreatedAt = createdAt.Time.UTC()
	result.UpdatedAt = updatedAt.Time.UTC()
	result.StartedAt = timePointer(startedAt)
	result.CompletedAt = timePointer(completedAt)
	var headers []string
	if err := json.Unmarshal(headersJSON, &headers); err != nil {
		return ImportStatus{}, nil, fmt.Errorf("decode import headers: %w", err)
	}
	return result, headers, nil
}

func validateExportFilter(filter ContactExportFilter) (ContactExportFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Sort = strings.TrimSpace(filter.Sort)
	filter.Order = strings.ToLower(strings.TrimSpace(filter.Order))
	if filter.Sort == "" {
		filter.Sort = "updatedAt"
	}
	if filter.Order == "" {
		filter.Order = "desc"
	}
	fields := make([]errx.FieldError, 0)
	if len(filter.Query) > 120 || len(filter.Status) > 40 {
		fields = append(fields, errx.FieldError{Pointer: "/query", Code: "validation.length"})
	}
	if filter.Sort != "updatedAt" && filter.Sort != "name" {
		fields = append(fields, errx.FieldError{Pointer: "/sort", Code: "validation.enum"})
	}
	if filter.Order != "asc" && filter.Order != "desc" {
		fields = append(fields, errx.FieldError{Pointer: "/order", Code: "validation.enum"})
	}
	if len(filter.TagIDs) > 100 {
		fields = append(fields, errx.FieldError{Pointer: "/tagIds", Code: "validation.max_items"})
	}
	if len(fields) > 0 {
		return ContactExportFilter{}, &errx.ValidationError{Fields: fields}
	}
	filter.TagIDs = uniqueUUIDs(filter.TagIDs)
	return filter, nil
}

func validateCSVHeaders(headers []string) error {
	if len(headers) < 1 || len(headers) > MaxCSVColumns {
		return csvValidation("/file", "validation.csv.column_count", map[string]any{"max": MaxCSVColumns})
	}
	seen := make(map[string]struct{}, len(headers))
	for index := range headers {
		headers[index] = strings.TrimSpace(headers[index])
		normalized := normalizeHeader(headers[index])
		if headers[index] == "" || len(headers[index]) > 120 || !utf8.ValidString(headers[index]) {
			return csvValidation("/file", "validation.csv.header_invalid", map[string]any{"column": index + 1})
		}
		if _, exists := seen[normalized]; exists {
			return csvValidation("/file", "validation.csv.header_duplicate", map[string]any{"column": index + 1})
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

func validateImportMapping(mapping ContactImportMapping, headers []string) error {
	headerSet := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		headerSet[header] = struct{}{}
	}
	fields := make([]errx.FieldError, 0)
	if mapping.FirstName == "" || mapping.LastName == "" {
		fields = append(fields, errx.FieldError{Pointer: "/mapping", Code: "validation.import.names_required"})
	}
	used := make(map[string]struct{})
	values := []string{mapping.FirstName, mapping.LastName, mapping.Email, mapping.Phone, mapping.JobTitle,
		mapping.CompanyName, mapping.OwnerEmail, mapping.Status, mapping.Source}
	for _, header := range values {
		if header == "" {
			continue
		}
		if _, exists := headerSet[header]; !exists {
			fields = append(fields, errx.FieldError{Pointer: "/mapping", Code: "validation.import.header_unknown", Params: map[string]any{"header": header}})
			continue
		}
		if _, exists := used[header]; exists {
			fields = append(fields, errx.FieldError{Pointer: "/mapping", Code: "validation.import.header_reused", Params: map[string]any{"header": header}})
		}
		used[header] = struct{}{}
	}
	for key, header := range mapping.CustomFields {
		if !fieldKeyPattern.MatchString(key) {
			fields = append(fields, errx.FieldError{Pointer: "/mapping/customFields", Code: "validation.custom_field.unknown"})
		}
		if _, exists := headerSet[header]; !exists {
			fields = append(fields, errx.FieldError{Pointer: "/mapping/customFields", Code: "validation.import.header_unknown", Params: map[string]any{"header": header}})
		}
		if _, exists := used[header]; exists {
			fields = append(fields, errx.FieldError{Pointer: "/mapping/customFields", Code: "validation.import.header_reused", Params: map[string]any{"header": header}})
		}
		used[header] = struct{}{}
	}
	if len(fields) > 0 {
		return &errx.ValidationError{Fields: fields}
	}
	return nil
}

func suggestedContactMapping(headers []string) map[string]string {
	aliases := map[string]string{
		"firstname": "firstName", "first_name": "firstName", "first name": "firstName",
		"имя": "firstName", "lastname": "lastName", "last_name": "lastName", "last name": "lastName",
		"фамилия": "lastName", "email": "email", "e-mail": "email", "почта": "email",
		"phone": "phone", "телефон": "phone", "job_title": "jobTitle", "job title": "jobTitle",
		"должность": "jobTitle", "company": "companyName", "company_name": "companyName", "компания": "companyName",
		"status": "status", "статус": "status", "source": "source", "источник": "source",
	}
	result := make(map[string]string)
	for _, header := range headers {
		if target, exists := aliases[normalizeHeader(header)]; exists {
			if _, already := result[target]; !already {
				result[target] = header
			}
		}
	}
	return result
}

func normalizeHeader(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func spreadsheetSafe(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func csvValidation(pointer, code string, params map[string]any) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code, Params: params}}}
}

func cloneStringMap(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
