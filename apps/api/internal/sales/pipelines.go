package sales

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) ListPipelinesAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
) ([]PipelineRecord, error) {
	pipelines, err := workspace.Queries.ListPipelines(ctx, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	stages, err := workspace.Queries.ListPipelineStagesForWorkspace(ctx, workspaceID.PG())
	if err != nil {
		return nil, fmt.Errorf("list pipeline stages: %w", err)
	}
	byPipeline := make(map[ids.UUID][]PipelineStageRecord, len(pipelines))
	for _, row := range stages {
		pipelineID, ok := ids.FromPG(row.PipelineID)
		if !ok {
			return nil, fmt.Errorf("list pipeline stages: invalid pipeline id")
		}
		byPipeline[pipelineID] = append(byPipeline[pipelineID], pipelineStageRecord(
			row.ID, row.PipelineID, row.Name, row.Probability, row.ForecastCategory,
			row.Position, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time,
		))
	}
	result := make([]PipelineRecord, 0, len(pipelines))
	for _, row := range pipelines {
		id, ok := ids.FromPG(row.ID)
		if !ok {
			return nil, fmt.Errorf("list pipelines: invalid id")
		}
		pipelineStages := byPipeline[id]
		if pipelineStages == nil {
			pipelineStages = []PipelineStageRecord{}
		}
		result = append(result, PipelineRecord{
			ID: id.String(), Name: row.Name, DisplayName: row.Name, IsDefault: row.IsDefault, Version: row.Version,
			Stages: pipelineStages, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
		})
	}
	return result, nil
}

func (service *Service) GetPipelineAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, pipelineID ids.UUID,
) (PipelineRecord, error) {
	row, err := workspace.Queries.GetPipelineAdvanced(ctx, dbgen.GetPipelineAdvancedParams{
		WorkspaceID: workspaceID.PG(), ID: pipelineID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PipelineRecord{}, errx.ErrNotFound
	}
	if err != nil {
		return PipelineRecord{}, fmt.Errorf("get pipeline: %w", err)
	}
	stages, err := workspace.Queries.ListPipelineStages(ctx, dbgen.ListPipelineStagesParams{
		WorkspaceID: workspaceID.PG(), PipelineID: pipelineID.PG(),
	})
	if err != nil {
		return PipelineRecord{}, fmt.Errorf("list pipeline stages: %w", err)
	}
	stageVersions, err := workspace.Queries.ListPipelineStagesForWorkspace(ctx, workspaceID.PG())
	if err != nil {
		return PipelineRecord{}, fmt.Errorf("list pipeline stage versions: %w", err)
	}
	versions := make(map[ids.UUID]int64, len(stageVersions))
	for _, stage := range stageVersions {
		if currentPipeline, ok := ids.FromPG(stage.PipelineID); !ok || currentPipeline != pipelineID {
			continue
		}
		if id, ok := ids.FromPG(stage.ID); ok {
			versions[id] = stage.Version
		}
	}
	resultStages := make([]PipelineStageRecord, 0, len(stages))
	for _, stage := range stages {
		stageID, _ := ids.FromPG(stage.ID)
		resultStages = append(resultStages, pipelineStageRecord(
			stage.ID, stage.PipelineID, stage.Name, stage.Probability, stage.ForecastCategory,
			stage.Position, versions[stageID], stage.CreatedAt.Time, stage.UpdatedAt.Time,
		))
	}
	return PipelineRecord{
		ID: pipelineID.String(), Name: row.Name, DisplayName: row.Name, IsDefault: row.IsDefault, Version: row.Version,
		Stages: resultStages, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}

func (service *Service) CreatePipelineAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input PipelineInput,
) (PipelineRecord, error) {
	validated, err := validatePipelineInput(input)
	if err != nil {
		return PipelineRecord{}, err
	}
	if err := lockSalesConfiguration(ctx, workspace, metadata.WorkspaceID, "pipelines"); err != nil {
		return PipelineRecord{}, err
	}
	count, err := workspace.Queries.CountPipelines(ctx, metadata.WorkspaceID.PG())
	if err != nil {
		return PipelineRecord{}, fmt.Errorf("count pipelines: %w", err)
	}
	if count == 0 {
		validated.IsDefault = true
	}
	pipelineID, err := ids.NewV7()
	if err != nil {
		return PipelineRecord{}, err
	}
	if validated.IsDefault {
		if err := workspace.Queries.UnsetPipelineDefaults(ctx, dbgen.UnsetPipelineDefaultsParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(),
		}); err != nil {
			return PipelineRecord{}, fmt.Errorf("unset default pipeline: %w", err)
		}
	}
	row, err := workspace.Queries.CreatePipeline(ctx, dbgen.CreatePipelineParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(), Name: validated.Name, IsDefault: validated.IsDefault,
	})
	if err != nil {
		return PipelineRecord{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline.created", EventType: "sales.pipeline.created", AggregateType: "pipeline", AggregateID: pipelineID,
		Summary: map[string]any{"fields": []string{"name", "isDefault"}},
		Payload: map[string]any{"pipelineId": pipelineID.String(), "version": row.Version},
	}); err != nil {
		return PipelineRecord{}, err
	}
	return PipelineRecord{
		ID: pipelineID.String(), Name: row.Name, DisplayName: row.Name, IsDefault: row.IsDefault, Version: row.Version,
		Stages: []PipelineStageRecord{}, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}

func (service *Service) UpdatePipelineAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	pipelineID ids.UUID,
	version int64,
	input PipelineInput,
) (PipelineRecord, error) {
	validated, err := validatePipelineInput(input)
	if err != nil {
		return PipelineRecord{}, err
	}
	if err := lockSalesConfiguration(ctx, workspace, metadata.WorkspaceID, "pipelines"); err != nil {
		return PipelineRecord{}, err
	}
	existing, err := workspace.Queries.GetPipelineAdvanced(ctx, dbgen.GetPipelineAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PipelineRecord{}, errx.ErrNotFound
	}
	if err != nil {
		return PipelineRecord{}, fmt.Errorf("get pipeline: %w", err)
	}
	if existing.Version != version {
		return PipelineRecord{}, errx.ErrVersionConflict
	}
	if existing.IsDefault && !validated.IsDefault {
		return PipelineRecord{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/isDefault", Code: "validation.pipeline.default_required",
		}}}
	}
	if validated.IsDefault {
		if err := workspace.Queries.UnsetPipelineDefaults(ctx, dbgen.UnsetPipelineDefaultsParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(),
		}); err != nil {
			return PipelineRecord{}, fmt.Errorf("unset default pipeline: %w", err)
		}
	}
	row, err := workspace.Queries.UpdatePipelineAdvanced(ctx, dbgen.UpdatePipelineAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(), Name: validated.Name,
		IsDefault: validated.IsDefault, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PipelineRecord{}, errx.ErrVersionConflict
	}
	if err != nil {
		return PipelineRecord{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline.updated", EventType: "sales.pipeline.updated", AggregateType: "pipeline", AggregateID: pipelineID,
		Summary: map[string]any{"fields": []string{"name", "isDefault"}},
		Payload: map[string]any{"pipelineId": pipelineID.String(), "version": row.Version},
	}); err != nil {
		return PipelineRecord{}, err
	}
	return PipelineRecord{
		ID: pipelineID.String(), Name: row.Name, DisplayName: row.Name, IsDefault: row.IsDefault, Version: row.Version,
		Stages: []PipelineStageRecord{}, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}, nil
}

func (service *Service) DeletePipelineAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	pipelineID ids.UUID,
	version int64,
) error {
	existing, err := workspace.Queries.GetPipelineAdvanced(ctx, dbgen.GetPipelineAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get pipeline: %w", err)
	}
	if existing.Version != version {
		return errx.ErrVersionConflict
	}
	if existing.IsDefault {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/pipelineId", Code: "validation.pipeline.default_delete"}}}
	}
	count, err := workspace.Queries.CountPipelines(ctx, metadata.WorkspaceID.PG())
	if err != nil {
		return fmt.Errorf("count pipelines: %w", err)
	}
	if count <= 1 {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/pipelineId", Code: "validation.pipeline.last_delete"}}}
	}
	_, err = workspace.Queries.DeletePipelineAdvanced(ctx, dbgen.DeletePipelineAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/pipelineId", Code: "validation.pipeline.not_empty"}}}
	}
	if err != nil {
		return fmt.Errorf("delete pipeline: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline.deleted", EventType: "sales.pipeline.deleted", AggregateType: "pipeline", AggregateID: pipelineID,
		Summary: map[string]any{"name": existing.Name}, Payload: map[string]any{"pipelineId": pipelineID.String()},
	})
}

func (service *Service) CreatePipelineStageAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	pipelineID ids.UUID,
	input PipelineStageInput,
) (PipelineStageRecord, error) {
	validated, err := validateStageInput(input)
	if err != nil {
		return PipelineStageRecord{}, err
	}
	if err := lockSalesConfiguration(ctx, workspace, metadata.WorkspaceID, "pipeline-stages:"+pipelineID.String()); err != nil {
		return PipelineStageRecord{}, err
	}
	if _, err := workspace.Queries.GetPipelineAdvanced(ctx, dbgen.GetPipelineAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: pipelineID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return PipelineStageRecord{}, errx.ErrNotFound
	} else if err != nil {
		return PipelineStageRecord{}, fmt.Errorf("get pipeline: %w", err)
	}
	count, err := workspace.Queries.CountPipelineStages(ctx, dbgen.CountPipelineStagesParams{
		WorkspaceID: metadata.WorkspaceID.PG(), PipelineID: pipelineID.PG(),
	})
	if err != nil {
		return PipelineStageRecord{}, fmt.Errorf("count pipeline stages: %w", err)
	}
	if count >= MaxPipelineStages {
		return PipelineStageRecord{}, validation("/stages", "validation.max_items")
	}
	position, err := workspace.Queries.NextPipelineStagePositionAdvanced(ctx, dbgen.NextPipelineStagePositionAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), PipelineID: pipelineID.PG(),
	})
	if err != nil {
		return PipelineStageRecord{}, fmt.Errorf("next stage position: %w", err)
	}
	stageID, err := ids.NewV7()
	if err != nil {
		return PipelineStageRecord{}, err
	}
	row, err := workspace.Queries.CreatePipelineStage(ctx, dbgen.CreatePipelineStageParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(), PipelineID: pipelineID.PG(), Name: validated.Name,
		Probability: int16(validated.Probability), ForecastCategory: validated.ForecastCategory, Position: position,
	})
	if err != nil {
		return PipelineStageRecord{}, mapConstraintError(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline_stage.created", EventType: "sales.pipeline_stage.created", AggregateType: "pipeline_stage", AggregateID: stageID,
		Summary: map[string]any{"pipelineId": pipelineID.String(), "fields": []string{"name", "probability", "forecastCategory"}},
		Payload: map[string]any{"pipelineStageId": stageID.String(), "pipelineId": pipelineID.String(), "version": 1},
	}); err != nil {
		return PipelineStageRecord{}, err
	}
	return pipelineStageRecord(row.ID, row.PipelineID, row.Name, row.Probability, row.ForecastCategory,
		row.Position, 1, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) UpdatePipelineStageAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	stageID ids.UUID,
	version int64,
	input PipelineStageInput,
) (PipelineStageRecord, error) {
	validated, err := validateStageInput(input)
	if err != nil {
		return PipelineStageRecord{}, err
	}
	existing, err := service.getStage(ctx, workspace, metadata.WorkspaceID, stageID)
	if err != nil {
		return PipelineStageRecord{}, err
	}
	if existing.Version != version {
		return PipelineStageRecord{}, errx.ErrVersionConflict
	}
	row, err := workspace.Queries.UpdatePipelineStageAdvanced(ctx, dbgen.UpdatePipelineStageAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(), Name: validated.Name,
		Probability: int16(validated.Probability), ForecastCategory: validated.ForecastCategory, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PipelineStageRecord{}, errx.ErrVersionConflict
	}
	if err != nil {
		return PipelineStageRecord{}, mapConstraintError(err)
	}
	if err := workspace.Queries.RefreshDashboardSummary(ctx, metadata.WorkspaceID.PG()); err != nil {
		return PipelineStageRecord{}, fmt.Errorf("refresh dashboard: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline_stage.updated", EventType: "sales.pipeline_stage.updated", AggregateType: "pipeline_stage", AggregateID: stageID,
		Summary: map[string]any{"fields": []string{"name", "probability", "forecastCategory"}},
		Payload: map[string]any{"pipelineStageId": stageID.String(), "pipelineId": pgUUIDString(row.PipelineID), "version": row.Version},
	}); err != nil {
		return PipelineStageRecord{}, err
	}
	return pipelineStageRecord(row.ID, row.PipelineID, row.Name, row.Probability, row.ForecastCategory,
		row.Position, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) ReorderPipelineStages(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	pipelineID ids.UUID,
	order []StageOrderItem,
) ([]PipelineStageRecord, error) {
	if err := validateStageOrder(order); err != nil {
		return nil, err
	}
	rows, err := workspace.Tx.Query(ctx, `
		SELECT id, version FROM sales.pipeline_stages
		WHERE workspace_id = $1 AND pipeline_id = $2
		ORDER BY position, id
		FOR UPDATE`, metadata.WorkspaceID.PG(), pipelineID.PG())
	if err != nil {
		return nil, fmt.Errorf("lock pipeline stages: %w", err)
	}
	defer rows.Close()
	current := make(map[ids.UUID]int64, len(order))
	for rows.Next() {
		var stageIDValue pgtype.UUID
		var stageVersion int64
		if err := rows.Scan(&stageIDValue, &stageVersion); err != nil {
			return nil, fmt.Errorf("scan pipeline stage lock: %w", err)
		}
		stageID, ok := ids.FromPG(stageIDValue)
		if !ok {
			return nil, fmt.Errorf("lock pipeline stages: invalid id")
		}
		current[stageID] = stageVersion
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lock pipeline stages: %w", err)
	}
	if len(current) != len(order) {
		return nil, validation("/stages", "validation.pipeline_stage.complete_order_required")
	}
	for _, stage := range order {
		currentVersion, exists := current[stage.ID]
		if !exists {
			return nil, validation("/stages", "validation.reference.invalid")
		}
		if currentVersion != stage.Version {
			return nil, errx.ErrVersionConflict
		}
	}
	if err := workspace.Queries.OffsetPipelineStagePositions(ctx, dbgen.OffsetPipelineStagePositionsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), PipelineID: pipelineID.PG(),
	}); err != nil {
		return nil, fmt.Errorf("offset stage positions: %w", err)
	}
	result := make([]PipelineStageRecord, 0, len(order))
	for position, stage := range order {
		row, err := workspace.Queries.ApplyPipelineStagePosition(ctx, dbgen.ApplyPipelineStagePositionParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: stage.ID.PG(), Position: int32(position),
		})
		if err != nil {
			return nil, fmt.Errorf("apply stage position: %w", err)
		}
		result = append(result, pipelineStageRecord(row.ID, row.PipelineID, row.Name, row.Probability,
			row.ForecastCategory, row.Position, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time))
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline_stages.reordered", EventType: "sales.pipeline_stages.reordered", AggregateType: "pipeline", AggregateID: pipelineID,
		Summary: map[string]any{"stageCount": len(order)}, Payload: map[string]any{"pipelineId": pipelineID.String(), "stageCount": len(order)},
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (service *Service) DeletePipelineStageAdvanced(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	stageID ids.UUID,
	version int64,
) error {
	existing, err := service.getStage(ctx, workspace, metadata.WorkspaceID, stageID)
	if err != nil {
		return err
	}
	if existing.Version != version {
		return errx.ErrVersionConflict
	}
	_, err = workspace.Queries.DeletePipelineStageAdvanced(ctx, dbgen.DeletePipelineStageAdvancedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: stageID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stageId", Code: "validation.pipeline_stage.in_use_or_last"}}}
	}
	if err != nil {
		return fmt.Errorf("delete pipeline stage: %w", err)
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "pipeline_stage.deleted", EventType: "sales.pipeline_stage.deleted", AggregateType: "pipeline_stage", AggregateID: stageID,
		Summary: map[string]any{"pipelineId": existing.PipelineID}, Payload: map[string]any{"pipelineStageId": stageID.String(), "pipelineId": existing.PipelineID},
	})
}

func (service *Service) getStage(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, stageID ids.UUID,
) (PipelineStageRecord, error) {
	row, err := workspace.Queries.GetPipelineStageAdvanced(ctx, dbgen.GetPipelineStageAdvancedParams{
		WorkspaceID: workspaceID.PG(), ID: stageID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PipelineStageRecord{}, errx.ErrNotFound
	}
	if err != nil {
		return PipelineStageRecord{}, fmt.Errorf("get pipeline stage: %w", err)
	}
	return pipelineStageRecord(row.ID, row.PipelineID, row.Name, row.Probability, row.ForecastCategory,
		row.Position, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func pipelineStageRecord(
	id, pipelineID pgtype.UUID,
	name string,
	probability int16,
	forecastCategory string,
	position int32,
	version int64,
	createdAt, updatedAt time.Time,
) PipelineStageRecord {
	stageID, _ := ids.FromPG(id)
	parentID, _ := ids.FromPG(pipelineID)
	return PipelineStageRecord{
		ID: stageID.String(), PipelineID: parentID.String(), Name: name, DisplayName: name,
		Probability: int(probability), ForecastCategory: forecastCategory,
		Position: int(position), Version: version,
		CreatedAt: createdAt.UTC(), UpdatedAt: updatedAt.UTC(),
	}
}
