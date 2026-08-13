package app

import (
	"net/http"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) listDeals(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	pipelineID, err := parseOptionalID(request.URL.Query().Get("pipelineId"), "/query/pipelineId")
	if writeError(application, writer, request, err) {
		return
	}
	stageID, err := parseOptionalID(request.URL.Query().Get("stageId"), "/query/stageId")
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var page sales.DealPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			page, loadErr = application.sales.ListDeals(
				request.Context(), workspace, workspaceID, pipelineID, stageID, request.URL.Query().Get("cursor"), limit,
			)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	items := make([]apigen.Deal, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, dealFromList(row))
	}
	result := apigen.DealPage{Items: items}
	if page.NextCursor != "" {
		result.NextCursor = &page.NextCursor
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createDeal(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateDeal](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	var closeDate *time.Time
	if body.ExpectedCloseDate != nil {
		value := body.ExpectedCloseDate.Time.UTC()
		closeDate = &value
	}
	var plannedStartDate *time.Time
	if body.PlannedStartDate != nil {
		value := body.PlannedStartDate.Time.UTC()
		plannedStartDate = &value
	}
	input := sales.DealInput{
		Name: body.Name, PipelineID: ids.UUID(body.PipelineId), StageID: ids.UUID(body.StageId),
		ContactID: internalIDPointer(body.ContactId), CompanyID: internalIDPointer(body.CompanyId),
		OwnerID: internalIDPointer(body.OwnerId), AmountMinor: body.AmountMinor, Currency: body.Currency,
		PlannedStartDate: plannedStartDate, ExpectedCloseDate: closeDate,
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionDealsCreate, "deals.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			row, createErr := application.sales.CreateDeal(request.Context(), workspace, metadata, input)
			if createErr != nil {
				return nil, 0, createErr
			}
			return dealFromCreate(row), row.Version, nil
		})
}

func (application *Application) moveDeal(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	dealID, err := parsePathID(request, "dealId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[apigen.SalesMoveDealJSONBody](writer, request, 32<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result apigen.Deal
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			row, moveErr := application.sales.MoveDeal(
				request.Context(), workspace, metadata(request, workspaceID, principal), dealID, ids.UUID(body.StageId), body.Position, version,
			)
			if moveErr == nil {
				result = dealFromMove(row)
				newVersion = row.Version
			}
			return moveErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listPipelines(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []apigen.Pipeline
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			pipelines, loadErr := application.sales.ListPipelines(request.Context(), workspace, workspaceID)
			if loadErr != nil {
				return loadErr
			}
			result = make([]apigen.Pipeline, 0, len(pipelines))
			for _, pipeline := range pipelines {
				stages := make([]apigen.PipelineStage, 0, len(pipeline.Stages))
				for _, stage := range pipeline.Stages {
					stages = append(stages, apigen.PipelineStage{
						Id: apiID(stage.ID), Name: stage.Name, Position: int(stage.Position), Probability: int(stage.Probability),
					})
				}
				result = append(result, apigen.Pipeline{Id: apiID(pipeline.Row.ID), Name: pipeline.Row.Name, Stages: stages})
			}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}
