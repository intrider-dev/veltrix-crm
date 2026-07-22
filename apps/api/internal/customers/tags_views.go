package customers

import (
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

func (service *Service) ListTags(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID) ([]Tag, error) {
	rows, err := workspace.Tx.Query(ctx, `
		SELECT id, name, color, version, created_at, updated_at
		FROM customers.tags WHERE workspace_id = $1 ORDER BY lower(name), id`, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	tags := make([]Tag, 0)
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (service *Service) CreateTag(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input TagInput,
) (Tag, error) {
	validated, err := validateTagInput(input)
	if err != nil {
		return Tag{}, err
	}
	id, err := ids.NewV7()
	if err != nil {
		return Tag{}, err
	}
	tag, err := scanTag(workspace.Tx.QueryRow(ctx, `
		INSERT INTO customers.tags (workspace_id, id, name, color)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, color, version, created_at, updated_at`,
		metadata.WorkspaceID.PG(), id.PG(), validated.Name, validated.Color))
	if err != nil {
		return Tag{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "tag.created", EventType: "customers.tag.created", AggregateType: "tag", AggregateID: id,
		Summary: map[string]any{"name": validated.Name, "color": validated.Color}, Payload: map[string]any{"tagId": id.String(), "version": tag.Version},
	}); err != nil {
		return Tag{}, err
	}
	return tag, nil
}

func (service *Service) UpdateTag(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	tagID ids.UUID,
	version int64,
	input TagInput,
) (Tag, error) {
	validated, err := validateTagInput(input)
	if err != nil {
		return Tag{}, err
	}
	tag, err := scanTag(workspace.Tx.QueryRow(ctx, `
		UPDATE customers.tags
		SET name = $3, color = $4, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $5
		RETURNING id, name, color, version, created_at, updated_at`,
		metadata.WorkspaceID.PG(), tagID.PG(), validated.Name, validated.Color, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return Tag{}, classifyVersionFailure(ctx, workspace, "customers.tags", metadata.WorkspaceID, tagID)
	}
	if err != nil {
		return Tag{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "tag.updated", EventType: "customers.tag.updated", AggregateType: "tag", AggregateID: tagID,
		Summary: map[string]any{"fields": []string{"name", "color"}}, Payload: map[string]any{"tagId": tagID.String(), "version": tag.Version},
	}); err != nil {
		return Tag{}, err
	}
	return tag, nil
}

func (service *Service) DeleteTag(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	tagID ids.UUID,
	version int64,
) error {
	var name string
	err := workspace.Tx.QueryRow(ctx, `
		DELETE FROM customers.tags
		WHERE workspace_id = $1 AND id = $2 AND version = $3
		RETURNING name`, metadata.WorkspaceID.PG(), tagID.PG(), version).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return classifyVersionFailure(ctx, workspace, "customers.tags", metadata.WorkspaceID, tagID)
	}
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "tag.deleted", EventType: "customers.tag.deleted", AggregateType: "tag", AggregateID: tagID,
		Summary: map[string]any{"name": name}, Payload: map[string]any{"tagId": tagID.String()},
	})
}

func (service *Service) ReplaceContactTags(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	contactID ids.UUID,
	version int64,
	tagIDs []ids.UUID,
) (int64, error) {
	if len(tagIDs) > 100 {
		return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/tagIds", Code: "validation.max_items", Params: map[string]any{"max": 100}}}}
	}
	unique := make(map[ids.UUID]struct{}, len(tagIDs))
	pgIDs := make([]pgtype.UUID, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if tagID == (ids.UUID{}) {
			return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/tagIds", Code: "validation.uuid.invalid"}}}
		}
		if _, exists := unique[tagID]; exists {
			continue
		}
		unique[tagID] = struct{}{}
		pgIDs = append(pgIDs, tagID.PG())
	}
	if len(pgIDs) > 0 {
		var count int64
		if err := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM customers.tags
			WHERE workspace_id = $1 AND id = ANY($2::uuid[])`, metadata.WorkspaceID.PG(), pgIDs).Scan(&count); err != nil {
			return 0, fmt.Errorf("validate contact tags: %w", err)
		}
		if count != int64(len(pgIDs)) {
			return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/tagIds", Code: "validation.reference.invalid"}}}
		}
	}
	var newVersion int64
	err := workspace.Tx.QueryRow(ctx, `
		UPDATE customers.contacts SET version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NULL
		RETURNING version`, metadata.WorkspaceID.PG(), contactID.PG(), version).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, versionedRecordError(ctx, workspace, "customers.contacts", metadata.WorkspaceID, contactID, false)
	}
	if err != nil {
		return 0, fmt.Errorf("lock contact tag version: %w", err)
	}
	if _, err := workspace.Tx.Exec(ctx, `DELETE FROM customers.contact_tags
		WHERE workspace_id = $1 AND contact_id = $2`, metadata.WorkspaceID.PG(), contactID.PG()); err != nil {
		return 0, fmt.Errorf("clear contact tags: %w", err)
	}
	if len(pgIDs) > 0 {
		if _, err := workspace.Tx.Exec(ctx, `
			INSERT INTO customers.contact_tags (workspace_id, contact_id, tag_id)
			SELECT $1, $2, value FROM unnest($3::uuid[]) AS value`, metadata.WorkspaceID.PG(), contactID.PG(), pgIDs); err != nil {
			return 0, fmt.Errorf("replace contact tags: %w", err)
		}
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "contact.tags.updated", EventType: "customers.contact.updated", AggregateType: "contact", AggregateID: contactID,
		Summary: map[string]any{"tagIds": uuidStrings(tagIDs)}, Payload: map[string]any{"contactId": contactID.String(), "version": newVersion},
	}); err != nil {
		return 0, err
	}
	return newVersion, nil
}

func (service *Service) ListSavedViews(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID ids.UUID,
	entityType string,
) ([]SavedView, error) {
	if !allowedEntityType(entityType) && entityType != "activity" {
		return nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/entityType", Code: "validation.enum"}}}
	}
	rows, err := workspace.Tx.Query(ctx, `
		SELECT id, owner_user_id, entity_type, name, definition, is_shared,
		       version, created_at, updated_at
		FROM customers.saved_views
		WHERE workspace_id = $1 AND entity_type = $2 AND (owner_user_id = $3 OR is_shared)
		ORDER BY is_shared DESC, lower(name), id`, workspaceID.PG(), entityType, actorID.PG())
	if err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}
	defer rows.Close()
	views := make([]SavedView, 0)
	for rows.Next() {
		view, err := scanSavedView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved view: %w", err)
		}
		views = append(views, view)
	}
	return views, rows.Err()
}

func (service *Service) CreateSavedView(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input SavedViewInput,
) (SavedView, error) {
	validated, err := validateSavedView(input)
	if err != nil {
		return SavedView{}, err
	}
	id, err := ids.NewV7()
	if err != nil {
		return SavedView{}, err
	}
	definition, _ := json.Marshal(validated.Definition)
	view, err := scanSavedView(workspace.Tx.QueryRow(ctx, `
		INSERT INTO customers.saved_views (
		  workspace_id, id, owner_user_id, entity_type, name, definition, is_shared
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, owner_user_id, entity_type, name, definition, is_shared,
		          version, created_at, updated_at`, metadata.WorkspaceID.PG(), id.PG(), metadata.ActorID.PG(),
		validated.EntityType, validated.Name, definition, validated.IsShared))
	if err != nil {
		return SavedView{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "saved_view.created", EventType: "customers.saved_view.created", AggregateType: "saved_view", AggregateID: id,
		Summary: map[string]any{"entityType": view.EntityType, "isShared": view.IsShared}, Payload: map[string]any{"savedViewId": id.String(), "version": view.Version},
	}); err != nil {
		return SavedView{}, err
	}
	return view, nil
}

func (service *Service) UpdateSavedView(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	viewID ids.UUID,
	version int64,
	input SavedViewInput,
) (SavedView, error) {
	validated, err := validateSavedView(input)
	if err != nil {
		return SavedView{}, err
	}
	definition, _ := json.Marshal(validated.Definition)
	isAdmin := workspace.Membership.Role == "owner" || workspace.Membership.Role == "admin"
	view, err := scanSavedView(workspace.Tx.QueryRow(ctx, `
		UPDATE customers.saved_views
		SET name = $4, definition = $5, is_shared = $6, version = version + 1, updated_at = now()
		WHERE workspace_id = $1 AND id = $2 AND version = $3 AND entity_type = $7
		  AND (owner_user_id = $8 OR $9::boolean)
		RETURNING id, owner_user_id, entity_type, name, definition, is_shared,
		          version, created_at, updated_at`, metadata.WorkspaceID.PG(), viewID.PG(), version,
		validated.Name, definition, validated.IsShared, validated.EntityType, metadata.ActorID.PG(), isAdmin))
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedView{}, service.classifySavedViewMutation(ctx, workspace, metadata.WorkspaceID, metadata.ActorID, viewID, version, isAdmin)
	}
	if err != nil {
		return SavedView{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "saved_view.updated", EventType: "customers.saved_view.updated", AggregateType: "saved_view", AggregateID: viewID,
		Summary: map[string]any{"entityType": view.EntityType, "isShared": view.IsShared}, Payload: map[string]any{"savedViewId": viewID.String(), "version": view.Version},
	}); err != nil {
		return SavedView{}, err
	}
	return view, nil
}

func (service *Service) DeleteSavedView(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	viewID ids.UUID,
	version int64,
) error {
	isAdmin := workspace.Membership.Role == "owner" || workspace.Membership.Role == "admin"
	var entityType string
	err := workspace.Tx.QueryRow(ctx, `
		DELETE FROM customers.saved_views
		WHERE workspace_id = $1 AND id = $2 AND version = $3
		  AND (owner_user_id = $4 OR $5::boolean)
		RETURNING entity_type`, metadata.WorkspaceID.PG(), viewID.PG(), version, metadata.ActorID.PG(), isAdmin).Scan(&entityType)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.classifySavedViewMutation(ctx, workspace, metadata.WorkspaceID, metadata.ActorID, viewID, version, isAdmin)
	}
	if err != nil {
		return fmt.Errorf("delete saved view: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "saved_view.deleted", EventType: "customers.saved_view.deleted", AggregateType: "saved_view", AggregateID: viewID,
		Summary: map[string]any{"entityType": entityType}, Payload: map[string]any{"savedViewId": viewID.String()},
	})
}

func (service *Service) classifySavedViewMutation(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID, viewID ids.UUID,
	version int64,
	isAdmin bool,
) error {
	var ownerID pgtype.UUID
	var currentVersion int64
	err := workspace.Tx.QueryRow(ctx, `SELECT owner_user_id, version FROM customers.saved_views
		WHERE workspace_id = $1 AND id = $2`, workspaceID.PG(), viewID.PG()).Scan(&ownerID, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify saved view mutation: %w", err)
	}
	owner, _ := ids.FromPG(ownerID)
	if !isAdmin && owner != actorID {
		return errx.ErrForbidden
	}
	if currentVersion != version {
		return errx.ErrVersionConflict
	}
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/entityType", Code: "validation.immutable"}}}
}

func classifyVersionFailure(ctx context.Context, workspace *tenancy.WorkspaceTx, table string, workspaceID, id ids.UUID) error {
	if table != "customers.tags" {
		return errx.ErrNotFound
	}
	var exists bool
	if err := workspace.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM customers.tags
		WHERE workspace_id = $1 AND id = $2)`, workspaceID.PG(), id.PG()).Scan(&exists); err != nil {
		return fmt.Errorf("classify tag mutation: %w", err)
	}
	if !exists {
		return errx.ErrNotFound
	}
	return errx.ErrVersionConflict
}

func scanTag(row rowScanner) (Tag, error) {
	var id pgtype.UUID
	var tag Tag
	var createdAt, updatedAt pgtype.Timestamptz
	if err := row.Scan(&id, &tag.Name, &tag.Color, &tag.Version, &createdAt, &updatedAt); err != nil {
		return Tag{}, err
	}
	tag.ID = idString(id)
	tag.CreatedAt = createdAt.Time.UTC()
	tag.UpdatedAt = updatedAt.Time.UTC()
	return tag, nil
}

func scanSavedView(row rowScanner) (SavedView, error) {
	var id, ownerID pgtype.UUID
	var definition []byte
	var view SavedView
	var createdAt, updatedAt pgtype.Timestamptz
	if err := row.Scan(&id, &ownerID, &view.EntityType, &view.Name, &definition, &view.IsShared,
		&view.Version, &createdAt, &updatedAt); err != nil {
		return SavedView{}, err
	}
	view.ID = idString(id)
	view.OwnerID = idString(ownerID)
	view.CreatedAt = createdAt.Time.UTC()
	view.UpdatedAt = updatedAt.Time.UTC()
	if err := json.Unmarshal(definition, &view.Definition); err != nil {
		return SavedView{}, fmt.Errorf("decode saved view: %w", err)
	}
	return view, nil
}

func uuidStrings(values []ids.UUID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

var _ = strings.Compare
