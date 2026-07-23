package customers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) ListCustomFieldDefinitions(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
) ([]CustomFieldDefinition, error) {
	return service.listCustomFieldDefinitions(ctx, workspace, workspaceID, entityType, false)
}

func (service *Service) listCustomFieldDefinitions(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	lockForValueMutation bool,
) ([]CustomFieldDefinition, error) {
	if entityType != "" && !allowedEntityType(entityType) {
		return nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/entityType", Code: "validation.enum"}}}
	}
	query := `
		SELECT id, entity_type, field_key, label, value_type, validation, options,
		       schema_version, version, created_at, updated_at
		FROM customers.custom_field_definitions
		WHERE workspace_id = $1 AND ($2 = '' OR entity_type = $2)
		ORDER BY entity_type, field_key, id`
	if lockForValueMutation {
		// The row lock is held by the surrounding workspace transaction through
		// aggregate persistence, so a schema update cannot race validation.
		query += " FOR SHARE"
	}
	rows, err := workspace.Tx.Query(ctx, query, workspaceID.PG(), entityType)
	if err != nil {
		return nil, fmt.Errorf("list custom field definitions: %w", err)
	}
	defer rows.Close()
	definitions := make([]CustomFieldDefinition, 0)
	for rows.Next() {
		definition, err := scanCustomFieldDefinition(rows)
		if err != nil {
			return nil, fmt.Errorf("scan custom field definition: %w", err)
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom field definitions: %w", err)
	}
	return definitions, nil
}

func (service *Service) CreateCustomFieldDefinition(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input CustomFieldDefinitionInput,
) (CustomFieldDefinition, error) {
	validated, err := validateDefinitionInput(input)
	if err != nil {
		return CustomFieldDefinition{}, err
	}
	id, err := ids.NewV7()
	if err != nil {
		return CustomFieldDefinition{}, err
	}
	validationJSON, _ := json.Marshal(validated.Validation)
	optionsJSON, _ := json.Marshal(validated.Options)
	definition, err := scanCustomFieldDefinition(workspace.Tx.QueryRow(ctx, `
		INSERT INTO customers.custom_field_definitions (
		  workspace_id, id, entity_type, field_key, label, value_type, validation, options
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, entity_type, field_key, label, value_type, validation, options,
		          schema_version, version, created_at, updated_at`,
		metadata.WorkspaceID.PG(), id.PG(), validated.EntityType, validated.FieldKey,
		validated.Label, validated.ValueType, validationJSON, optionsJSON))
	if err != nil {
		return CustomFieldDefinition{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "custom_field.created", EventType: "customers.custom_field.created",
		AggregateType: "custom_field_definition", AggregateID: id,
		Summary: map[string]any{"entityType": validated.EntityType, "fieldKey": validated.FieldKey, "valueType": validated.ValueType},
		Payload: map[string]any{"definitionId": id.String(), "entityType": validated.EntityType, "schemaVersion": definition.SchemaVersion},
	}); err != nil {
		return CustomFieldDefinition{}, err
	}
	return definition, nil
}

func (service *Service) UpdateCustomFieldDefinition(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	definitionID ids.UUID,
	version int64,
	input CustomFieldDefinitionInput,
) (CustomFieldDefinition, error) {
	validated, err := validateDefinitionInput(input)
	if err != nil {
		return CustomFieldDefinition{}, err
	}
	current, err := scanCustomFieldDefinition(workspace.Tx.QueryRow(ctx, `
		SELECT id, entity_type, field_key, label, value_type, validation, options,
		       schema_version, version, created_at, updated_at
		FROM customers.custom_field_definitions
		WHERE workspace_id = $1 AND id = $2
		FOR UPDATE`, metadata.WorkspaceID.PG(), definitionID.PG()))
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomFieldDefinition{}, errx.ErrNotFound
	}
	if err != nil {
		return CustomFieldDefinition{}, fmt.Errorf("lock custom field definition: %w", err)
	}
	if current.Version != version {
		return CustomFieldDefinition{}, errx.ErrVersionConflict
	}
	if current.EntityType != validated.EntityType || current.FieldKey != validated.FieldKey {
		return CustomFieldDefinition{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/fieldKey", Code: "validation.immutable"}}}
	}
	validationJSON, _ := json.Marshal(validated.Validation)
	optionsJSON, _ := json.Marshal(validated.Options)
	currentOptions, _ := json.Marshal(current.Options)
	if current.ValueType != validated.ValueType || !bytes.Equal(currentOptions, optionsJSON) {
		var valueCount int64
		if err := workspace.Tx.QueryRow(ctx, `
			SELECT count(*) FROM customers.custom_field_values
			WHERE workspace_id = $1 AND definition_id = $2`, metadata.WorkspaceID.PG(), definitionID.PG()).Scan(&valueCount); err != nil {
			return CustomFieldDefinition{}, fmt.Errorf("count custom field values: %w", err)
		}
		if valueCount > 0 {
			return CustomFieldDefinition{}, &errx.ValidationError{Fields: []errx.FieldError{{
				Pointer: "/valueType", Code: "validation.custom_field.migration_required", Params: map[string]any{"valueCount": valueCount},
			}}}
		}
	}
	updated, err := scanCustomFieldDefinition(workspace.Tx.QueryRow(ctx, `
		UPDATE customers.custom_field_definitions
		SET label = $3, value_type = $4, validation = $5, options = $6,
		    schema_version = schema_version + 1, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $7
		RETURNING id, entity_type, field_key, label, value_type, validation, options,
		          schema_version, version, created_at, updated_at`,
		metadata.WorkspaceID.PG(), definitionID.PG(), validated.Label, validated.ValueType,
		validationJSON, optionsJSON, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomFieldDefinition{}, errx.ErrVersionConflict
	}
	if err != nil {
		return CustomFieldDefinition{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "custom_field.updated", EventType: "customers.custom_field.updated",
		AggregateType: "custom_field_definition", AggregateID: definitionID,
		Summary: map[string]any{"fieldKey": updated.FieldKey, "schemaVersion": updated.SchemaVersion},
		Payload: map[string]any{"definitionId": definitionID.String(), "entityType": updated.EntityType, "schemaVersion": updated.SchemaVersion},
	}); err != nil {
		return CustomFieldDefinition{}, err
	}
	return updated, nil
}

func (service *Service) DeleteCustomFieldDefinition(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	definitionID ids.UUID,
	version int64,
) error {
	var entityType, fieldKey string
	err := workspace.Tx.QueryRow(ctx, `
		DELETE FROM customers.custom_field_definitions
		WHERE workspace_id = $1 AND id = $2 AND version = $3
		RETURNING entity_type, field_key`, metadata.WorkspaceID.PG(), definitionID.PG(), version).Scan(&entityType, &fieldKey)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if scanErr := workspace.Tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM customers.custom_field_definitions WHERE workspace_id = $1 AND id = $2
		)`, metadata.WorkspaceID.PG(), definitionID.PG()).Scan(&exists); scanErr != nil {
			return fmt.Errorf("classify custom field deletion: %w", scanErr)
		}
		if exists {
			return errx.ErrVersionConflict
		}
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete custom field definition: %w", err)
	}
	// Remove the denormalized copy in the same transaction. jsonb - text is
	// parameterized; the table name is selected only from a closed enum.
	tables := map[string]string{
		"contact": "customers.contacts",
		"company": "customers.companies",
		"lead":    "sales.leads",
		"deal":    "sales.deals",
	}
	if table, ok := tables[entityType]; ok {
		if _, err := workspace.Tx.Exec(ctx, `UPDATE `+table+`
			SET custom_fields = custom_fields - $2, version = version + 1, updated_at = now()
			WHERE workspace_id = $1 AND custom_fields ? $2`, metadata.WorkspaceID.PG(), fieldKey); err != nil {
			return fmt.Errorf("remove denormalized custom field: %w", err)
		}
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "custom_field.deleted", EventType: "customers.custom_field.deleted",
		AggregateType: "custom_field_definition", AggregateID: definitionID,
		Summary: map[string]any{"entityType": entityType, "fieldKey": fieldKey},
		Payload: map[string]any{"definitionId": definitionID.String(), "entityType": entityType},
	})
}

func (service *Service) SetEntityCustomFields(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	entityType string,
	entityID ids.UUID,
	version int64,
	values map[string]any,
) (int64, error) {
	if entityType != "contact" && entityType != "company" {
		return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/entityType", Code: "validation.enum"}}}
	}
	aggregate, normalized, err := service.prepareCustomFields(ctx, workspace, metadata.WorkspaceID, entityType, values)
	if err != nil {
		return 0, err
	}
	table := "customers.contacts"
	if entityType == "company" {
		table = "customers.companies"
	}
	var newVersion int64
	err = workspace.Tx.QueryRow(ctx, `UPDATE `+table+`
		SET custom_fields = $3, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $4 AND deleted_at IS NULL
		RETURNING version`, metadata.WorkspaceID.PG(), entityID.PG(), aggregate, version).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, versionedRecordError(ctx, workspace, table, metadata.WorkspaceID, entityID, false)
	}
	if err != nil {
		return 0, fmt.Errorf("update custom fields: %w", err)
	}
	if err := service.replaceCustomFieldValues(ctx, workspace, metadata.WorkspaceID, entityType, entityID, normalized); err != nil {
		return 0, err
	}
	if _, err := workspace.Tx.Exec(ctx, `
		UPDATE search.documents SET version = $4, updated_at = now()
		WHERE workspace_id = $1 AND entity_type = $2 AND entity_id = $3`,
		metadata.WorkspaceID.PG(), entityType, entityID.PG(), newVersion); err != nil {
		return 0, fmt.Errorf("advance search document version: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: entityType + ".custom_fields.updated", EventType: "customers." + entityType + ".updated",
		AggregateType: entityType, AggregateID: entityID,
		Summary: map[string]any{"fields": sortedKeys(values)},
		Payload: map[string]any{entityType + "Id": entityID.String(), "version": newVersion},
	}); err != nil {
		return 0, err
	}
	return newVersion, nil
}

// ValidateCustomFields applies the workspace definitions to custom fields stored by another domain.
type ValidatedCustomFields struct {
	Values     map[string]any
	normalized map[string]json.RawMessage
}

func (service *Service) ValidateCustomFields(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	values map[string]any,
) (ValidatedCustomFields, error) {
	aggregate, normalizedValues, err := service.prepareCustomFields(ctx, workspace, workspaceID, entityType, values)
	if err != nil {
		return ValidatedCustomFields{}, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(aggregate, &normalized); err != nil {
		return ValidatedCustomFields{}, fmt.Errorf("decode validated custom fields: %w", err)
	}
	return ValidatedCustomFields{Values: normalized, normalized: normalizedValues}, nil
}

// PersistValidatedCustomFields keeps the normalized query/index representation
// transactionally aligned with another domain's JSON aggregate.
func (service *Service) PersistValidatedCustomFields(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	entityID ids.UUID,
	validated ValidatedCustomFields,
) error {
	return service.replaceCustomFieldValues(ctx, workspace, workspaceID, entityType, entityID, validated.normalized)
}

func (service *Service) prepareCustomFields(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	supplied map[string]any,
) ([]byte, map[string]json.RawMessage, error) {
	definitions, err := service.listCustomFieldDefinitions(ctx, workspace, workspaceID, entityType, true)
	if err != nil {
		return nil, nil, err
	}
	if supplied == nil {
		supplied = map[string]any{}
	}
	values, err := validateTypedValues(definitions, supplied)
	if err != nil {
		return nil, nil, err
	}
	if err := validateUserReferences(ctx, workspace, workspaceID, definitions, values); err != nil {
		return nil, nil, err
	}
	aggregate, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("encode custom fields: %w", err)
	}
	return aggregate, values, nil
}

func (service *Service) replaceCustomFieldValues(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	entityID ids.UUID,
	values map[string]json.RawMessage,
) error {
	definitions, err := service.listCustomFieldDefinitions(ctx, workspace, workspaceID, entityType, true)
	if err != nil {
		return err
	}
	byKey := make(map[string]CustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		byKey[definition.FieldKey] = definition
	}
	if _, err := workspace.Tx.Exec(ctx, `
		DELETE FROM customers.custom_field_values
		WHERE workspace_id = $1 AND entity_type = $2 AND entity_id = $3`,
		workspaceID.PG(), entityType, entityID.PG()); err != nil {
		return fmt.Errorf("clear custom field values: %w", err)
	}
	if len(values) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for key, value := range values {
		definition := byKey[key]
		definitionID, err := ids.Parse(definition.ID)
		if err != nil {
			return fmt.Errorf("invalid custom definition ID: %w", err)
		}
		batch.Queue(`
			INSERT INTO customers.custom_field_values (
			  workspace_id, definition_id, entity_type, entity_id, value, schema_version
			) VALUES ($1, $2, $3, $4, $5, $6)`,
			workspaceID.PG(), definitionID.PG(), entityType, entityID.PG(), []byte(value), definition.SchemaVersion)
	}
	results := workspace.Tx.SendBatch(ctx, batch)
	defer results.Close()
	for range values {
		if _, err := results.Exec(); err != nil {
			return mapConstraintError(err)
		}
	}
	return results.Close()
}

func validateUserReferences(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	definitions []CustomFieldDefinition,
	values map[string]json.RawMessage,
) error {
	definitionTypes := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		definitionTypes[definition.FieldKey] = definition.ValueType
	}
	unique := make(map[ids.UUID]struct{})
	for key, raw := range values {
		if definitionTypes[key] != "user_reference" {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		id, _ := ids.Parse(value)
		unique[id] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	identifiers := make([]pgtype.UUID, 0, len(unique))
	for id := range unique {
		identifiers = append(identifiers, id.PG())
	}
	var count int64
	if err := workspace.Tx.QueryRow(ctx, `
		SELECT count(*) FROM tenancy.memberships
		WHERE workspace_id = $1 AND user_id = ANY($2::uuid[]) AND status = 'active'`,
		workspaceID.PG(), identifiers).Scan(&count); err != nil {
		return fmt.Errorf("validate custom field user references: %w", err)
	}
	if count != int64(len(unique)) {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/customFields", Code: "validation.custom_field.user_reference"}}}
	}
	return nil
}

func scanCustomFieldDefinition(row rowScanner) (CustomFieldDefinition, error) {
	var id pgtype.UUID
	var definition CustomFieldDefinition
	var validationJSON, optionsJSON []byte
	var createdAt, updatedAt pgtype.Timestamptz
	err := row.Scan(&id, &definition.EntityType, &definition.FieldKey, &definition.Label,
		&definition.ValueType, &validationJSON, &optionsJSON, &definition.SchemaVersion,
		&definition.Version, &createdAt, &updatedAt)
	if err != nil {
		return CustomFieldDefinition{}, err
	}
	definition.ID = idString(id)
	definition.CreatedAt = createdAt.Time.UTC()
	definition.UpdatedAt = updatedAt.Time.UTC()
	if err := json.Unmarshal(validationJSON, &definition.Validation); err != nil {
		return CustomFieldDefinition{}, fmt.Errorf("decode custom field validation: %w", err)
	}
	if err := json.Unmarshal(optionsJSON, &definition.Options); err != nil {
		return CustomFieldDefinition{}, fmt.Errorf("decode custom field options: %w", err)
	}
	return definition, nil
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slicesSort(keys)
	return keys
}

func slicesSort(values []string) {
	// Tiny insertion sort avoids adding a helper dependency for typically fewer
	// than ten custom fields and keeps audit output deterministic.
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && strings.Compare(values[current], values[current-1]) < 0; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}
