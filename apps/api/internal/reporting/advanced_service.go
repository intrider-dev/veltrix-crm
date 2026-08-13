package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) Preferences(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID ids.UUID,
) (Preferences, error) {
	row, err := workspace.Queries.GetDashboardPreferences(ctx, dbgen.GetDashboardPreferencesParams{
		UserID: userID.PG(), WorkspaceID: workspaceID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Preferences{}, errx.ErrNotFound
	}
	if err != nil {
		return Preferences{}, fmt.Errorf("get dashboard preferences: %w", err)
	}
	result := Preferences{
		Layout: json.RawMessage(`{}`), PeriodDays: 30,
		EffectiveTimezone: row.WorkspaceTimezone,
	}
	if len(row.Preferences) > 0 {
		result.Layout = append(json.RawMessage(nil), row.Preferences...)
	}
	if row.PeriodDays != nil {
		result.PeriodDays = *row.PeriodDays
	}
	if row.Timezone != nil {
		result.Timezone = row.Timezone
		result.EffectiveTimezone = *row.Timezone
	}
	if row.Version != nil {
		result.Version = *row.Version
	}
	if row.UpdatedAt.Valid {
		updated := row.UpdatedAt.Time.UTC()
		result.UpdatedAt = &updated
	}
	if _, err := time.LoadLocation(result.EffectiveTimezone); err != nil {
		return Preferences{}, fmt.Errorf("stored dashboard timezone is invalid: %w", err)
	}
	return result, nil
}

func (service *Service) SavePreferences(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, userID ids.UUID,
	expectedVersion int64,
	input PreferencesInput,
) (Preferences, error) {
	input, err := validatePreferences(input)
	if err != nil {
		return Preferences{}, err
	}
	current, err := service.Preferences(ctx, workspace, workspaceID, userID)
	if err != nil {
		return Preferences{}, err
	}
	if current.Version != expectedVersion {
		return Preferences{}, errx.ErrVersionConflict
	}
	row, err := workspace.Queries.SaveDashboardPreferences(ctx, dbgen.SaveDashboardPreferencesParams{
		WorkspaceID: workspaceID.PG(), UserID: userID.PG(), Preferences: input.Layout,
		PeriodDays: input.PeriodDays, Timezone: input.Timezone, ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Preferences{}, errx.ErrVersionConflict
	}
	if err != nil {
		return Preferences{}, fmt.Errorf("save dashboard preferences: %w", err)
	}
	effectiveTimezone := current.EffectiveTimezone
	if row.Timezone != nil {
		effectiveTimezone = *row.Timezone
	} else if current.Timezone != nil {
		// Preferences explicitly cleared their override; reload to select the
		// workspace timezone instead of carrying the prior override forward.
		reloaded, reloadErr := service.Preferences(ctx, workspace, workspaceID, userID)
		if reloadErr != nil {
			return Preferences{}, reloadErr
		}
		effectiveTimezone = reloaded.EffectiveTimezone
	}
	updated := row.UpdatedAt.Time.UTC()
	return Preferences{
		Layout: append(json.RawMessage(nil), row.Preferences...), PeriodDays: row.PeriodDays,
		Timezone: row.Timezone, EffectiveTimezone: effectiveTimezone,
		Version: row.Version, UpdatedAt: &updated,
	}, nil
}

func (service *Service) PeriodReport(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	period Period,
) (Report, error) {
	period, err := ValidatePeriod(period.Start, period.End, period.Timezone)
	if err != nil {
		return Report{}, err
	}
	start := pgtype.Timestamptz{Time: period.Start, Valid: true}
	end := pgtype.Timestamptz{Time: period.End, Valid: true}
	overviewRow, err := workspace.Queries.GetPeriodReportOverview(ctx, dbgen.GetPeriodReportOverviewParams{
		WorkspaceID: workspaceID.PG(), PeriodStart: start, PeriodEnd: end,
	})
	if err != nil {
		return Report{}, fmt.Errorf("load report overview: %w", err)
	}
	stageRows, err := workspace.Queries.ReportDealsByStage(ctx, dbgen.ReportDealsByStageParams{
		PeriodStart: start, PeriodEnd: end, WorkspaceID: workspaceID.PG(),
	})
	if err != nil {
		return Report{}, fmt.Errorf("load deals by stage: %w", err)
	}
	ownerRows, err := workspace.Queries.ReportDealsByOwner(ctx, dbgen.ReportDealsByOwnerParams{
		WorkspaceID: workspaceID.PG(), PeriodStart: start, PeriodEnd: end,
	})
	if err != nil {
		return Report{}, fmt.Errorf("load deals by owner: %w", err)
	}
	activityRows, err := workspace.Queries.ReportActivitiesByDay(ctx, dbgen.ReportActivitiesByDayParams{
		Timezone: period.Timezone, WorkspaceID: workspaceID.PG(), PeriodStart: start, PeriodEnd: end,
	})
	if err != nil {
		return Report{}, fmt.Errorf("load activities by day: %w", err)
	}
	sourceRows, err := workspace.Queries.ReportLeadSources(ctx, dbgen.ReportLeadSourcesParams{
		WorkspaceID: workspaceID.PG(), PeriodStart: start, PeriodEnd: end,
	})
	if err != nil {
		return Report{}, fmt.Errorf("load lead sources: %w", err)
	}
	result := Report{
		Period: period,
		Overview: Overview{
			WonCount: overviewRow.WonCount, LostCount: overviewRow.LostCount,
			WonValueMinor: overviewRow.WonValueMinor, LeadCount: overviewRow.LeadCount,
			ConvertedLeadCount: overviewRow.ConvertedLeadCount,
			ConversionRate:     conversionRate(overviewRow.ConvertedLeadCount, overviewRow.LeadCount),
			ActivityCount:      overviewRow.ActivityCount,
		},
		DealsByStage: make([]StageMetric, 0, len(stageRows)),
		DealsByOwner: make([]OwnerMetric, 0, len(ownerRows)),
		Activities:   make([]ActivityDayMetric, 0, len(activityRows)),
		LeadSources:  make([]LeadSourceMetric, 0, len(sourceRows)),
	}
	for _, row := range stageRows {
		result.DealsByStage = append(result.DealsByStage, StageMetric{
			StageID: requiredID(row.StageID), StageName: row.StageName, Position: row.Position,
			DealCount: row.DealCount, AmountMinor: row.AmountMinor,
			WeightedAmountMinor: row.WeightedAmountMinor,
		})
	}
	for _, row := range ownerRows {
		result.DealsByOwner = append(result.DealsByOwner, OwnerMetric{
			OwnerID: optionalID(row.OwnerUserID), OwnerName: row.OwnerName, DealCount: row.DealCount,
			WonCount: row.WonCount, LostCount: row.LostCount, AmountMinor: row.AmountMinor,
		})
	}
	for _, row := range activityRows {
		result.Activities = append(result.Activities, ActivityDayMetric{
			Date: row.ActivityDate.Time.Format("2006-01-02"), Count: row.ActivityCount,
			TaskCount: row.TaskCount, CallCount: row.CallCount,
			MeetingCount: row.MeetingCount, NoteCount: row.NoteCount,
		})
	}
	for _, row := range sourceRows {
		result.LeadSources = append(result.LeadSources, LeadSourceMetric{
			Source: row.Source, LeadCount: row.LeadCount, ConvertedCount: row.ConvertedCount,
		})
	}
	return result, nil
}

func (service *Service) RecentActivity(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	period Period,
	limit int,
) ([]RecentActivity, error) {
	if _, err := ValidatePeriod(period.Start, period.End, period.Timezone); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := workspace.Queries.ListDashboardRecentActivity(ctx, dbgen.ListDashboardRecentActivityParams{
		WorkspaceID: workspaceID.PG(), PeriodStart: pgtype.Timestamptz{Time: period.Start, Valid: true},
		PeriodEnd: pgtype.Timestamptz{Time: period.End, Valid: true}, ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent dashboard activity: %w", err)
	}
	result := make([]RecentActivity, 0, len(rows))
	for _, row := range rows {
		result = append(result, RecentActivity{
			ID: requiredID(row.ID), Type: row.ActivityType, Title: row.Title,
			RelatedType: row.RelatedType, RelatedID: optionalID(row.RelatedID),
			Status: row.Status, Priority: row.Priority, OccurredAt: row.OccurredAt.Time.UTC(),
			DueAt: optionalTime(row.DueAt), AssigneeID: optionalID(row.AssigneeUserID),
		})
	}
	return result, nil
}

func requiredID(value pgtype.UUID) ids.UUID {
	result, _ := ids.FromPG(value)
	return result
}

func optionalID(value pgtype.UUID) *ids.UUID {
	result, valid := ids.FromPG(value)
	if !valid {
		return nil
	}
	return &result
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
