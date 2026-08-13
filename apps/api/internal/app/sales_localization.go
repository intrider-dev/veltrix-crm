package app

import (
	"context"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const (
	pipelineNameNamespace      = "sales.pipeline.name"
	pipelineStageNameNamespace = "sales.pipeline_stage.name"
	leadStageNameNamespace     = "sales.lead_stage.name"
)

func preferredWorkspaceLocale(workspace *tenancy.WorkspaceTx, userPreference string) string {
	if workspace.Membership.LocaleOverride != nil {
		return *workspace.Membership.LocaleOverride
	}
	return userPreference
}

func (application *Application) registerSalesContent(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	preferredLocale, namespace, resourceKey, sourceText string,
) error {
	locale, err := application.translations.EffectiveLocale(
		ctx, workspace, metadata.WorkspaceID,
		preferredWorkspaceLocale(workspace, preferredLocale),
	)
	if err != nil {
		return err
	}
	_, err = application.translations.RegisterResource(
		ctx, workspace, metadata, namespace, resourceKey,
		localization.ContentResourceInput{SourceLocale: locale, SourceText: sourceText},
	)
	return err
}

func (application *Application) localizePipelines(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	preferredLocale string,
	pipelines []sales.PipelineRecord,
) error {
	preference := preferredWorkspaceLocale(workspace, preferredLocale)
	pipelineKeys := make([]string, 0, len(pipelines))
	stageKeys := make([]string, 0, len(pipelines)*4)
	for _, pipeline := range pipelines {
		pipelineKeys = append(pipelineKeys, pipeline.ID)
		for _, stage := range pipeline.Stages {
			stageKeys = append(stageKeys, stage.ID)
		}
	}
	pipelineNames, err := application.translations.ResolveBatch(
		ctx, workspace, workspaceID, preference, pipelineNameNamespace, pipelineKeys,
	)
	if err != nil {
		return err
	}
	stageNames, err := application.translations.ResolveBatch(
		ctx, workspace, workspaceID, preference, pipelineStageNameNamespace, stageKeys,
	)
	if err != nil {
		return err
	}
	for pipelineIndex := range pipelines {
		if resolved, ok := pipelineNames[pipelines[pipelineIndex].ID]; ok {
			pipelines[pipelineIndex].DisplayName = resolved.Text
		}
		for stageIndex := range pipelines[pipelineIndex].Stages {
			stage := &pipelines[pipelineIndex].Stages[stageIndex]
			if resolved, ok := stageNames[stage.ID]; ok {
				stage.DisplayName = resolved.Text
			}
		}
	}
	return nil
}

func (application *Application) localizePipelineStages(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	preferredLocale string,
	stages []sales.PipelineStageRecord,
) error {
	keys := make([]string, 0, len(stages))
	for _, stage := range stages {
		keys = append(keys, stage.ID)
	}
	resolved, err := application.translations.ResolveBatch(
		ctx, workspace, workspaceID, preferredWorkspaceLocale(workspace, preferredLocale),
		pipelineStageNameNamespace, keys,
	)
	if err != nil {
		return err
	}
	for index := range stages {
		if name, ok := resolved[stages[index].ID]; ok {
			stages[index].DisplayName = name.Text
		}
	}
	return nil
}

func (application *Application) localizeLeadStages(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	preferredLocale string,
	stages []sales.LeadStageRecord,
) error {
	keys := make([]string, 0, len(stages))
	for _, stage := range stages {
		if stage.SystemKey == nil {
			keys = append(keys, stage.ID)
		}
	}
	resolved, err := application.translations.ResolveBatch(
		ctx, workspace, workspaceID, preferredWorkspaceLocale(workspace, preferredLocale),
		leadStageNameNamespace, keys,
	)
	if err != nil {
		return err
	}
	for index := range stages {
		if name, ok := resolved[stages[index].ID]; ok {
			stages[index].DisplayName = name.Text
		}
	}
	return nil
}
