package customers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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

var phoneCleanup = regexp.MustCompile(`[^0-9+]`)

type ContactInput struct {
	FirstName    string
	LastName     string
	Email        *string
	Phone        *string
	JobTitle     *string
	CompanyID    *ids.UUID
	OwnerID      *ids.UUID
	TeamID       *ids.UUID
	Status       string
	Source       *string
	Address      map[string]string
	CustomFields map[string]any
}

type ContactPage struct {
	Items      []dbgen.ListContactsRow
	NextCursor string
}

type CompanyPage struct {
	Items      []dbgen.ListCompaniesPageRow
	NextCursor string
}

type CompanyInput struct {
	Name         string
	Domain       *string
	Industry     *string
	OwnerID      *ids.UUID
	TeamID       *ids.UUID
	Status       string
	Address      map[string]string
	CustomFields map[string]any
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) ListContacts(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	query, status, cursor string,
	limit int,
) (ContactPage, error) {
	query = strings.TrimSpace(query)
	status = strings.TrimSpace(status)
	if len(query) > 120 || len(status) > 40 {
		return ContactPage{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/query", Code: "validation.length"}}}
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	filter := "q=" + query + "&status=" + status
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return ContactPage{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/cursor", Code: "validation.cursor.invalid"}}}
	}
	rows, err := workspace.Queries.ListContacts(ctx, dbgen.ListContactsParams{
		WorkspaceID:     workspaceID.PG(),
		SearchQuery:     query,
		StatusFilter:    status,
		CursorUpdatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true},
		CursorID:        cursorID.PG(),
		PageLimit:       int32(limit + 1),
	})
	if err != nil {
		return ContactPage{}, fmt.Errorf("list contacts: %w", err)
	}
	page := ContactPage{Items: rows}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.UpdatedAt.Time, lastID, filter)
		if err != nil {
			return ContactPage{}, err
		}
		page.Items = rows[:limit]
	}
	return page, nil
}

func (service *Service) GetContact(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, contactID ids.UUID) (dbgen.GetContactRow, error) {
	row, err := workspace.Queries.GetContact(ctx, dbgen.GetContactParams{WorkspaceID: workspaceID.PG(), ID: contactID.PG()})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.GetContactRow{}, errx.ErrNotFound
	}
	if err != nil {
		return dbgen.GetContactRow{}, fmt.Errorf("get contact: %w", err)
	}
	return row, nil
}

func (service *Service) CreateContact(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input ContactInput,
) (dbgen.CreateContactRow, error) {
	validated, err := validateContact(input)
	if err != nil {
		return dbgen.CreateContactRow{}, err
	}
	contactID, err := ids.NewV7()
	if err != nil {
		return dbgen.CreateContactRow{}, err
	}
	customFields, normalizedFields, err := service.prepareCustomFields(ctx, workspace, metadata.WorkspaceID, "contact", validated.CustomFields)
	if err != nil {
		return dbgen.CreateContactRow{}, err
	}
	address, _ := json.Marshal(validated.Address)
	row, err := workspace.Queries.CreateContact(ctx, dbgen.CreateContactParams{
		WorkspaceID:     metadata.WorkspaceID.PG(),
		ID:              contactID.PG(),
		FirstName:       validated.FirstName,
		LastName:        validated.LastName,
		DisplayName:     strings.TrimSpace(validated.FirstName + " " + validated.LastName),
		Email:           validated.Email,
		EmailNormalized: normalizeEmail(validated.Email),
		Phone:           validated.Phone,
		PhoneNormalized: normalizePhone(validated.Phone),
		JobTitle:        validated.JobTitle,
		CompanyID:       optionalUUID(validated.CompanyID),
		OwnerUserID:     optionalUUID(validated.OwnerID),
		TeamID:          optionalUUID(validated.TeamID),
		Status:          validated.Status,
		Source:          validated.Source,
		Address:         address,
		CustomFields:    customFields,
	})
	if err != nil {
		return dbgen.CreateContactRow{}, mapConstraintError(err)
	}
	if err := service.replaceCustomFieldValues(ctx, workspace, metadata.WorkspaceID, "contact", contactID, normalizedFields); err != nil {
		return dbgen.CreateContactRow{}, err
	}
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID:    metadata.WorkspaceID.PG(),
		EntityType:     "contact",
		EntityID:       contactID.PG(),
		Title:          row.DisplayName,
		Subtitle:       row.Email,
		SearchableText: strings.Join(nonEmpty(row.DisplayName, pointerValue(row.Email), pointerValue(row.Phone)), " "),
		RankBoost:      1.2,
		Version:        row.Version,
	}); err != nil {
		return dbgen.CreateContactRow{}, fmt.Errorf("index contact: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action:        "contact.created",
		EventType:     "customers.contact.created",
		AggregateType: "contact",
		AggregateID:   contactID,
		Summary:       map[string]any{"fields": []string{"firstName", "lastName", "email", "companyId"}},
		Payload:       map[string]any{"contactId": contactID.String(), "version": row.Version},
	}); err != nil {
		return dbgen.CreateContactRow{}, err
	}
	return row, nil
}

func (service *Service) UpdateContact(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	contactID ids.UUID,
	version int64,
	input ContactInput,
) (dbgen.UpdateContactRow, error) {
	validated, err := validateContact(input)
	if err != nil {
		return dbgen.UpdateContactRow{}, err
	}
	if _, err := service.GetContact(ctx, workspace, metadata.WorkspaceID, contactID); err != nil {
		return dbgen.UpdateContactRow{}, err
	}
	customFields, normalizedFields, err := service.prepareCustomFields(ctx, workspace, metadata.WorkspaceID, "contact", validated.CustomFields)
	if err != nil {
		return dbgen.UpdateContactRow{}, err
	}
	address, _ := json.Marshal(validated.Address)
	row, err := workspace.Queries.UpdateContact(ctx, dbgen.UpdateContactParams{
		WorkspaceID:     metadata.WorkspaceID.PG(),
		ID:              contactID.PG(),
		FirstName:       validated.FirstName,
		LastName:        validated.LastName,
		DisplayName:     strings.TrimSpace(validated.FirstName + " " + validated.LastName),
		Email:           validated.Email,
		EmailNormalized: normalizeEmail(validated.Email),
		Phone:           validated.Phone,
		PhoneNormalized: normalizePhone(validated.Phone),
		JobTitle:        validated.JobTitle,
		CompanyID:       optionalUUID(validated.CompanyID),
		OwnerUserID:     optionalUUID(validated.OwnerID),
		TeamID:          optionalUUID(validated.TeamID),
		Status:          validated.Status,
		Source:          validated.Source,
		Address:         address,
		CustomFields:    customFields,
		Version:         version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.UpdateContactRow{}, errx.ErrVersionConflict
	}
	if err != nil {
		return dbgen.UpdateContactRow{}, mapConstraintError(err)
	}
	if err := service.replaceCustomFieldValues(ctx, workspace, metadata.WorkspaceID, "contact", contactID, normalizedFields); err != nil {
		return dbgen.UpdateContactRow{}, err
	}
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "contact", EntityID: contactID.PG(),
		Title: row.DisplayName, Subtitle: row.Email,
		SearchableText: strings.Join(nonEmpty(row.DisplayName, pointerValue(row.Email), pointerValue(row.Phone)), " "),
		RankBoost:      1.2, Version: row.Version,
	}); err != nil {
		return dbgen.UpdateContactRow{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "contact.updated", EventType: "customers.contact.updated", AggregateType: "contact", AggregateID: contactID,
		Summary: map[string]any{"fields": []string{"firstName", "lastName", "email", "phone", "companyId", "status"}},
		Payload: map[string]any{"contactId": contactID.String(), "version": row.Version},
	}); err != nil {
		return dbgen.UpdateContactRow{}, err
	}
	return row, nil
}

func (service *Service) DeleteContact(ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, contactID ids.UUID, version int64) error {
	if _, err := service.GetContact(ctx, workspace, metadata.WorkspaceID, contactID); err != nil {
		return err
	}
	newVersion, err := workspace.Queries.SoftDeleteContact(ctx, dbgen.SoftDeleteContactParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: contactID.PG(), DeletedBy: metadata.ActorID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrVersionConflict
	}
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if err := workspace.Queries.DeleteSearchDocument(ctx, dbgen.DeleteSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "contact", EntityID: contactID.PG(),
	}); err != nil {
		return err
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "contact.deleted", EventType: "customers.contact.deleted", AggregateType: "contact", AggregateID: contactID,
		Summary: map[string]any{"softDelete": true}, Payload: map[string]any{"contactId": contactID.String(), "version": newVersion},
	})
}

func (service *Service) ListCompanies(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, limit int) ([]dbgen.ListCompaniesRow, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := workspace.Queries.ListCompanies(ctx, dbgen.ListCompaniesParams{WorkspaceID: workspaceID.PG(), Limit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list companies: %w", err)
	}
	return rows, nil
}

func (service *Service) ListCompaniesPage(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	query, status, cursor string,
	limit int,
) (CompanyPage, error) {
	query = strings.TrimSpace(query)
	status = strings.TrimSpace(status)
	var fields []errx.FieldError
	if len(query) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/query", Code: "validation.length"})
	}
	if len(status) > 40 {
		fields = append(fields, errx.FieldError{Pointer: "/status", Code: "validation.length"})
	}
	if len(fields) > 0 {
		return CompanyPage{}, &errx.ValidationError{Fields: fields}
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	filter := "q=" + query + "&status=" + status
	cursorTime, cursorID, err := pagination.Decode(cursor, filter)
	if err != nil {
		return CompanyPage{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/cursor", Code: "validation.cursor.invalid"}}}
	}
	rows, err := workspace.Queries.ListCompaniesPage(ctx, dbgen.ListCompaniesPageParams{
		WorkspaceID:     workspaceID.PG(),
		SearchQuery:     query,
		StatusFilter:    status,
		CursorUpdatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true},
		CursorID:        cursorID.PG(),
		PageLimit:       int32(limit + 1),
	})
	if err != nil {
		return CompanyPage{}, fmt.Errorf("list companies page: %w", err)
	}
	page := CompanyPage{Items: rows}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.UpdatedAt.Time, lastID, filter)
		if err != nil {
			return CompanyPage{}, err
		}
		page.Items = rows[:limit]
	}
	return page, nil
}

func (service *Service) GetCompany(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, companyID ids.UUID) (dbgen.GetCompanyRow, error) {
	row, err := workspace.Queries.GetCompany(ctx, dbgen.GetCompanyParams{WorkspaceID: workspaceID.PG(), ID: companyID.PG()})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.GetCompanyRow{}, errx.ErrNotFound
	}
	return row, err
}

func (service *Service) CreateCompany(ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, input CompanyInput) (dbgen.CreateCompanyRow, error) {
	validated, err := validateCompanyUpdate(CompanyUpdateInput{
		Name: input.Name, Domain: input.Domain, Industry: input.Industry, OwnerID: input.OwnerID,
		TeamID: input.TeamID, Status: input.Status, Address: input.Address, CustomFields: input.CustomFields,
	})
	if err != nil {
		return dbgen.CreateCompanyRow{}, err
	}
	companyID, err := ids.NewV7()
	if err != nil {
		return dbgen.CreateCompanyRow{}, err
	}
	var domainNormalized *string
	if validated.Domain != nil {
		value := strings.ToLower(strings.TrimSpace(*validated.Domain))
		validated.Domain = &value
		if value != "" {
			domainNormalized = &value
		}
	}
	customFields, normalizedFields, err := service.prepareCustomFields(ctx, workspace, metadata.WorkspaceID, "company", validated.CustomFields)
	if err != nil {
		return dbgen.CreateCompanyRow{}, err
	}
	address, _ := json.Marshal(validated.Address)
	row, err := workspace.Queries.CreateCompany(ctx, dbgen.CreateCompanyParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: companyID.PG(), Name: validated.Name,
		Domain: validated.Domain, DomainNormalized: domainNormalized, Industry: validated.Industry,
		OwnerUserID: optionalUUID(validated.OwnerID), TeamID: optionalUUID(validated.TeamID), Status: validated.Status,
		Address: address, CustomFields: customFields,
	})
	if err != nil {
		return dbgen.CreateCompanyRow{}, mapConstraintError(err)
	}
	if err := service.replaceCustomFieldValues(ctx, workspace, metadata.WorkspaceID, "company", companyID, normalizedFields); err != nil {
		return dbgen.CreateCompanyRow{}, err
	}
	if err := workspace.Queries.UpsertSearchDocument(ctx, dbgen.UpsertSearchDocumentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), EntityType: "company", EntityID: companyID.PG(),
		Title: row.Name, Subtitle: row.Domain, SearchableText: strings.Join(nonEmpty(row.Name, pointerValue(row.Domain), pointerValue(row.Industry)), " "),
		RankBoost: 1.1, Version: row.Version,
	}); err != nil {
		return dbgen.CreateCompanyRow{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "company.created", EventType: "customers.company.created", AggregateType: "company", AggregateID: companyID,
		Summary: map[string]any{"fields": []string{"name", "domain", "industry"}},
		Payload: map[string]any{"companyId": companyID.String(), "version": row.Version},
	}); err != nil {
		return dbgen.CreateCompanyRow{}, err
	}
	return row, nil
}

func validateContact(input ContactInput) (ContactInput, error) {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = trimPointer(input.Email)
	input.Phone = trimPointer(input.Phone)
	input.JobTitle = trimPointer(input.JobTitle)
	input.Source = trimPointer(input.Source)
	if input.Address == nil {
		input.Address = map[string]string{}
	}
	if input.Status == "" {
		input.Status = "active"
	}
	var fields []errx.FieldError
	if len(input.FirstName) < 1 || len(input.FirstName) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/firstName", Code: "validation.length"})
	}
	if len(input.LastName) < 1 || len(input.LastName) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/lastName", Code: "validation.length"})
	}
	if input.Email != nil && len(*input.Email) > 254 {
		fields = append(fields, errx.FieldError{Pointer: "/email", Code: "validation.email.invalid"})
	}
	if len(input.Status) < 1 || len(input.Status) > 40 {
		fields = append(fields, errx.FieldError{Pointer: "/status", Code: "validation.length"})
	}
	customFields, err := json.Marshal(input.CustomFields)
	if err != nil || len(customFields) > 65536 {
		fields = append(fields, errx.FieldError{Pointer: "/customFields", Code: "validation.custom_fields.invalid"})
	}
	if len(fields) > 0 {
		return ContactInput{}, &errx.ValidationError{Fields: fields}
	}
	if input.CustomFields == nil {
		input.CustomFields = map[string]any{}
	}
	address, addressErr := json.Marshal(input.Address)
	if addressErr != nil || len(address) > 8192 {
		return ContactInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/address", Code: "validation.address.invalid"}}}
	}
	return input, nil
}

func optionalUUID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func normalizeEmail(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	normalized := strings.ToLower(*value)
	return &normalized
}

func normalizePhone(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	normalized := phoneCleanup.ReplaceAllString(*value, "")
	return &normalized
}

func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func mapConstraintError(err error) error {
	var pgError interface{ SQLState() string }
	if errors.As(err, &pgError) {
		switch pgError.SQLState() {
		case "23503":
			return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/relation", Code: "validation.reference.invalid"}}}
		case "23505":
			return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/value", Code: "validation.duplicate"}}}
		}
	}
	return fmt.Errorf("database constraint: %w", err)
}

var _ = time.UTC
