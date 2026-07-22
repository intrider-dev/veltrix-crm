package sales

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

var stageColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (service *Service) ListLeadStages(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID,
) ([]LeadStageRecord, error) {
	rows, err := workspace.Queries.ListLeadStages(ctx, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("list lead stages: %w", err)
	}
	result := make([]LeadStageRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, leadStageRecord(row.ID, row.Name, row.Category, row.Color,
			row.Position, row.SystemKey, row.IsDefault, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time))
	}
	return result, nil
}

func (service *Service) CreateLeadStage(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, input LeadStageInput,
) (LeadStageRecord, error) {
	validated, err := validateLeadStageInput(input, true)
	if err != nil {
		return LeadStageRecord{}, err
	}
	id, err := ids.NewV7()
	if err != nil {
		return LeadStageRecord{}, err
	}
	position, err := workspace.Queries.NextLeadStagePosition(ctx, metadata.WorkspaceID.PG())
	if err != nil {
		return LeadStageRecord{}, fmt.Errorf("next lead stage position: %w", err)
	}
	row, err := workspace.Queries.CreateLeadStage(ctx, dbgen.CreateLeadStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: id.PG(), Name: validated.Name,
		Category: validated.Category, Color: validated.Color, Position: position,
	})
	if err != nil {
		return LeadStageRecord{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead_stage.created", EventType: "sales.lead_stage.created",
		AggregateType: "lead_stage", AggregateID: id,
		Summary: map[string]any{"name": row.Name, "category": row.Category, "position": row.Position},
		Payload: map[string]any{"stageId": id.String(), "version": row.Version},
	}); err != nil {
		return LeadStageRecord{}, err
	}
	return leadStageRecord(row.ID, row.Name, row.Category, row.Color, row.Position, row.SystemKey,
		row.IsDefault, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) UpdateLeadStage(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	stageID ids.UUID, version int64, input LeadStageInput,
) (LeadStageRecord, error) {
	validated, err := validateLeadStageInput(input, false)
	if err != nil {
		return LeadStageRecord{}, err
	}
	existing, err := workspace.Queries.GetLeadStage(ctx, dbgen.GetLeadStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadStageRecord{}, errx.ErrNotFound
	}
	if err != nil {
		return LeadStageRecord{}, fmt.Errorf("get lead stage: %w", err)
	}
	if existing.Version != version {
		return LeadStageRecord{}, errx.ErrVersionConflict
	}
	row, err := workspace.Queries.UpdateLeadStage(ctx, dbgen.UpdateLeadStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(),
		Name:  leadStageUpdateName(existing.Name, existing.SystemKey, validated.Name),
		Color: validated.Color, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LeadStageRecord{}, errx.ErrVersionConflict
	}
	if err != nil {
		return LeadStageRecord{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead_stage.updated", EventType: "sales.lead_stage.updated",
		AggregateType: "lead_stage", AggregateID: stageID,
		Summary: map[string]any{"fields": []string{"name", "color"}},
		Payload: map[string]any{"stageId": stageID.String(), "version": row.Version},
	}); err != nil {
		return LeadStageRecord{}, err
	}
	return leadStageRecord(row.ID, row.Name, row.Category, row.Color, row.Position, row.SystemKey,
		row.IsDefault, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) DeleteLeadStage(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	stageID ids.UUID, version int64,
) error {
	existing, err := workspace.Queries.GetLeadStage(ctx, dbgen.GetLeadStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get lead stage: %w", err)
	}
	if existing.SystemKey != nil {
		return errx.ErrForbidden
	}
	if existing.Version != version {
		return errx.ErrVersionConflict
	}
	changed, err := workspace.Queries.DeleteLeadStage(ctx, dbgen.DeleteLeadStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(), Version: version,
	})
	if err != nil {
		return mapConstraintError(err)
	}
	if changed == 0 {
		return errx.ErrConflict
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "lead_stage.deleted", EventType: "sales.lead_stage.deleted",
		AggregateType: "lead_stage", AggregateID: stageID,
		Summary: map[string]any{"deleted": true}, Payload: map[string]any{"stageId": stageID.String()},
	})
}

func validateLeadStageInput(input LeadStageInput, creating bool) (LeadStageInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(input.Category)
	input.Color = strings.TrimSpace(input.Color)
	fields := make([]errx.FieldError, 0, 3)
	if count := utf8.RuneCountInString(input.Name); count < 1 || count > 100 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if creating && input.Category != "new" && input.Category != "qualified" && input.Category != "disqualified" {
		fields = append(fields, errx.FieldError{Pointer: "/category", Code: "validation.enum"})
	}
	if !stageColorPattern.MatchString(input.Color) {
		fields = append(fields, errx.FieldError{Pointer: "/color", Code: "validation.color"})
	}
	if len(fields) > 0 {
		return LeadStageInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func leadStageRecord(
	id pgtype.UUID, name, category, color string, position int32,
	systemKey *string, isDefault bool, version int64, createdAt, updatedAt time.Time,
) LeadStageRecord {
	stageID, _ := ids.FromPG(id)
	return LeadStageRecord{
		ID: stageID.String(), Name: name, DisplayName: name,
		Category: category, Color: color, Position: int(position),
		SystemKey: systemKey, IsDefault: isDefault, Version: version,
		CreatedAt: createdAt.UTC(), UpdatedAt: updatedAt.UTC(),
	}
}

func leadStageUpdateName(existingName string, systemKey *string, requestedName string) string {
	if systemKey != nil {
		return existingName
	}
	return requestedName
}
