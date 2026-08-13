package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/assignment"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type pipelineRequest struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type pipelineStageRequest struct {
	Name             string `json:"name"`
	Probability      int    `json:"probability"`
	ForecastCategory string `json:"forecastCategory"`
}

type pipelineStageOrderRequest struct {
	Stages []versionedRecordRequest `json:"stages"`
}

type leadRequest struct {
	Name              string         `json:"name"`
	Email             *string        `json:"email"`
	Phone             *string        `json:"phone"`
	CompanyName       *string        `json:"companyName"`
	JobTitle          *string        `json:"jobTitle"`
	Source            *string        `json:"source"`
	Status            string         `json:"status"`
	StageID           *string        `json:"stageId"`
	OwnerID           *string        `json:"ownerId"`
	TeamID            *string        `json:"teamId"`
	PlannedStartDate  *string        `json:"plannedStartDate"`
	ExpectedCloseDate *string        `json:"expectedCloseDate"`
	CustomFields      map[string]any `json:"customFields"`
}

type leadStageRequest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Color    string `json:"color"`
}

type leadStageMoveRequest struct {
	StageID string `json:"stageId"`
}

type recordAssignmentRequest struct {
	Kind        string `json:"kind"`
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
	IsPrimary   bool   `json:"isPrimary"`
}

type recordAssignmentsRequest struct {
	Assignments []recordAssignmentRequest `json:"assignments"`
}

type leadConversionContactRequest struct {
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	JobTitle  *string `json:"jobTitle"`
}

type leadConversionCompanyRequest struct {
	Name     string  `json:"name"`
	Domain   *string `json:"domain"`
	Industry *string `json:"industry"`
}

type leadConversionDealRequest struct {
	Name              string  `json:"name"`
	PipelineID        string  `json:"pipelineId"`
	StageID           string  `json:"stageId"`
	AmountMinor       int64   `json:"amountMinor"`
	Currency          string  `json:"currency"`
	ExpectedCloseDate *string `json:"expectedCloseDate"`
}

type leadConversionRequest struct {
	ContactID     *string                       `json:"contactId"`
	CreateContact bool                          `json:"createContact"`
	Contact       *leadConversionContactRequest `json:"contact"`
	CompanyID     *string                       `json:"companyId"`
	CreateCompany bool                          `json:"createCompany"`
	Company       *leadConversionCompanyRequest `json:"company"`
	DealID        *string                       `json:"dealId"`
	Deal          *leadConversionDealRequest    `json:"deal"`
}

type leadConversionResponse struct {
	Lead      sales.LeadRecord `json:"lead"`
	ContactID string           `json:"contactId"`
	CompanyID *string          `json:"companyId,omitempty"`
	DealID    *string          `json:"dealId,omitempty"`
	Version   int64            `json:"version"`
}

func (application *Application) listPipelinesAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []sales.PipelineRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListPipelinesAdvanced(request.Context(), workspace, workspaceID)
			if err != nil {
				return err
			}
			return application.localizePipelines(request.Context(), workspace, workspaceID, principal.PreferredLocale, result)
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) getPipelineAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, pipelineID, ok := salesWorkspaceAndResourceID(application, writer, request, "pipelineId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.PipelineRecord
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.GetPipelineAdvanced(request.Context(), workspace, workspaceID, pipelineID)
			if err != nil {
				return err
			}
			pipelines := []sales.PipelineRecord{result}
			if err = application.localizePipelines(request.Context(), workspace, workspaceID, principal.PreferredLocale, pipelines); err == nil {
				result = pipelines[0]
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createPipelineAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[pipelineRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionDealStagesManage, "pipelines.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			pipeline, createErr := application.sales.CreatePipelineAdvanced(request.Context(), workspace, metadata,
				sales.PipelineInput{Name: body.Name, IsDefault: body.IsDefault})
			if createErr != nil {
				return pipeline, pipeline.Version, createErr
			}
			if createErr = application.registerSalesContent(request.Context(), workspace, metadata,
				principal.PreferredLocale, pipelineNameNamespace, pipeline.ID, pipeline.Name); createErr != nil {
				return pipeline, pipeline.Version, createErr
			}
			pipelines := []sales.PipelineRecord{pipeline}
			createErr = application.localizePipelines(request.Context(), workspace, workspaceID, principal.PreferredLocale, pipelines)
			return pipelines[0], pipeline.Version, createErr
		})
}

func (application *Application) updatePipelineAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, pipelineID, ok := salesWorkspaceAndResourceID(application, writer, request, "pipelineId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[pipelineRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.PipelineRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.UpdatePipelineAdvanced(request.Context(), workspace, metadata(request, workspaceID, principal),
				pipelineID, version, sales.PipelineInput{Name: body.Name, IsDefault: body.IsDefault})
			if err != nil {
				return err
			}
			if err = application.registerSalesContent(request.Context(), workspace, metadata(request, workspaceID, principal),
				principal.PreferredLocale, pipelineNameNamespace, result.ID, result.Name); err != nil {
				return err
			}
			pipelines := []sales.PipelineRecord{result}
			if err = application.localizePipelines(request.Context(), workspace, workspaceID, principal.PreferredLocale, pipelines); err == nil {
				result = pipelines[0]
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deletePipelineAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, pipelineID, ok := salesWorkspaceAndResourceID(application, writer, request, "pipelineId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			pipeline, loadErr := application.sales.GetPipelineAdvanced(request.Context(), workspace, workspaceID, pipelineID)
			if loadErr != nil {
				return loadErr
			}
			if deleteErr := application.sales.DeletePipelineAdvanced(request.Context(), workspace, metadata(request, workspaceID, principal), pipelineID, version); deleteErr != nil {
				return deleteErr
			}
			if deleteErr := application.translations.DeleteResources(request.Context(), workspace, workspaceID,
				pipelineNameNamespace, []string{pipeline.ID}); deleteErr != nil {
				return deleteErr
			}
			stageIDs := make([]string, 0, len(pipeline.Stages))
			for _, stage := range pipeline.Stages {
				stageIDs = append(stageIDs, stage.ID)
			}
			return application.translations.DeleteResources(request.Context(), workspace, workspaceID,
				pipelineStageNameNamespace, stageIDs)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) createPipelineStageAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, pipelineID, ok := salesWorkspaceAndResourceID(application, writer, request, "pipelineId")
	if !ok {
		return
	}
	body, raw, err := httpx.DecodeJSON[pipelineStageRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionDealStagesManage,
		"pipeline-stages.create."+pipelineID.String(), raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			stage, createErr := application.sales.CreatePipelineStageAdvanced(request.Context(), workspace, metadata, pipelineID,
				sales.PipelineStageInput{Name: body.Name, Probability: body.Probability, ForecastCategory: body.ForecastCategory})
			if createErr != nil {
				return stage, stage.Version, createErr
			}
			if createErr = application.registerSalesContent(request.Context(), workspace, metadata,
				principal.PreferredLocale, pipelineStageNameNamespace, stage.ID, stage.Name); createErr != nil {
				return stage, stage.Version, createErr
			}
			stages := []sales.PipelineStageRecord{stage}
			createErr = application.localizePipelineStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, stages)
			return stages[0], stage.Version, createErr
		})
}

func (application *Application) updatePipelineStageAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, stageID, ok := salesWorkspaceAndResourceID(application, writer, request, "stageId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[pipelineStageRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.PipelineStageRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.UpdatePipelineStageAdvanced(request.Context(), workspace, metadata(request, workspaceID, principal),
				stageID, version, sales.PipelineStageInput{Name: body.Name, Probability: body.Probability, ForecastCategory: body.ForecastCategory})
			if err != nil {
				return err
			}
			if err = application.registerSalesContent(request.Context(), workspace, metadata(request, workspaceID, principal),
				principal.PreferredLocale, pipelineStageNameNamespace, result.ID, result.Name); err != nil {
				return err
			}
			stages := []sales.PipelineStageRecord{result}
			if err = application.localizePipelineStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, stages); err == nil {
				result = stages[0]
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) reorderPipelineStages(writer http.ResponseWriter, request *http.Request) {
	workspaceID, pipelineID, ok := salesWorkspaceAndResourceID(application, writer, request, "pipelineId")
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[pipelineStageOrderRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	order := make([]sales.StageOrderItem, 0, len(body.Stages))
	for _, item := range body.Stages {
		id, parseErr := ids.Parse(strings.TrimSpace(item.ID))
		if parseErr != nil {
			writeError(application, writer, request, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stages/id", Code: "validation.uuid.invalid"}}})
			return
		}
		order = append(order, sales.StageOrderItem{ID: id, Version: item.Version})
	}
	principal, _ := httpx.Principal(request.Context())
	var result []sales.PipelineStageRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ReorderPipelineStages(request.Context(), workspace, metadata(request, workspaceID, principal), pipelineID, order)
			if err != nil {
				return err
			}
			return application.localizePipelineStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, result)
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deletePipelineStageAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, stageID, ok := salesWorkspaceAndResourceID(application, writer, request, "stageId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			if deleteErr := application.sales.DeletePipelineStageAdvanced(request.Context(), workspace, metadata(request, workspaceID, principal), stageID, version); deleteErr != nil {
				return deleteErr
			}
			return application.translations.DeleteResources(request.Context(), workspace, workspaceID,
				pipelineStageNameNamespace, []string{stageID.String()})
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) listLeadsAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	ownerID, err := parseOptionalID(request.URL.Query().Get("ownerId"), "/query/ownerId")
	if writeError(application, writer, request, err) {
		return
	}
	stageID, err := parseOptionalID(request.URL.Query().Get("stageId"), "/query/stageId")
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, sales.DefaultPageSize)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.LeadPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListLeads(request.Context(), workspace, workspaceID, sales.LeadListFilter{
				Query: request.URL.Query().Get("query"), Status: request.URL.Query().Get("status"),
				OwnerID: ownerID, StageID: stageID, Cursor: request.URL.Query().Get("cursor"), Limit: limit,
			})
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listLeadStages(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []sales.LeadStageRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListLeadStages(request.Context(), workspace, workspaceID)
			if err != nil {
				return err
			}
			return application.localizeLeadStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, result)
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createLeadStage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[leadStageRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionLeadStagesManage, "lead-stages.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata) (any, int64, error) {
			stage, createErr := application.sales.CreateLeadStage(request.Context(), workspace, mutationMetadata,
				sales.LeadStageInput{Name: body.Name, Category: body.Category, Color: body.Color})
			if createErr != nil {
				return stage, stage.Version, createErr
			}
			if createErr = application.registerSalesContent(request.Context(), workspace, mutationMetadata,
				principal.PreferredLocale, leadStageNameNamespace, stage.ID, stage.Name); createErr != nil {
				return stage, stage.Version, createErr
			}
			stages := []sales.LeadStageRecord{stage}
			createErr = application.localizeLeadStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, stages)
			return stages[0], stage.Version, createErr
		})
}

func (application *Application) updateLeadStage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, stageID, ok := salesWorkspaceAndResourceID(application, writer, request, "stageId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[leadStageRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.LeadStageRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.UpdateLeadStage(request.Context(), workspace, metadata(request, workspaceID, principal),
				stageID, version, sales.LeadStageInput{Name: body.Name, Category: body.Category, Color: body.Color})
			if err != nil || result.SystemKey != nil {
				return err
			}
			if err = application.registerSalesContent(request.Context(), workspace, metadata(request, workspaceID, principal),
				principal.PreferredLocale, leadStageNameNamespace, result.ID, result.Name); err != nil {
				return err
			}
			stages := []sales.LeadStageRecord{result}
			if err = application.localizeLeadStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, stages); err == nil {
				result = stages[0]
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) reorderLeadStages(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[pipelineStageOrderRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	order := make([]sales.StageOrderItem, 0, len(body.Stages))
	for _, item := range body.Stages {
		id, parseErr := ids.Parse(strings.TrimSpace(item.ID))
		if parseErr != nil {
			writeError(application, writer, request, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stages/id", Code: "validation.uuid.invalid"}}})
			return
		}
		order = append(order, sales.StageOrderItem{ID: id, Version: item.Version})
	}
	principal, _ := httpx.Principal(request.Context())
	var result []sales.LeadStageRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ReorderLeadStages(request.Context(), workspace, metadata(request, workspaceID, principal), order)
			if err != nil {
				return err
			}
			return application.localizeLeadStages(request.Context(), workspace, workspaceID, principal.PreferredLocale, result)
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteLeadStage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, stageID, ok := salesWorkspaceAndResourceID(application, writer, request, "stageId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			if deleteErr := application.sales.DeleteLeadStage(request.Context(), workspace, metadata(request, workspaceID, principal), stageID, version); deleteErr != nil {
				return deleteErr
			}
			return application.translations.DeleteResources(request.Context(), workspace, workspaceID,
				leadStageNameNamespace, []string{stageID.String()})
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) moveLeadStage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[leadStageMoveRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	stageID, err := ids.Parse(strings.TrimSpace(body.StageID))
	if err != nil {
		writeError(application, writer, request, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stageId", Code: "validation.uuid.invalid"}}})
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.LeadRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.MoveLeadStage(request.Context(), workspace, metadata(request, workspaceID, principal), leadID, stageID, version)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listLeadAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result assignment.Set
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListLeadAssignments(request.Context(), workspace, workspaceID, leadID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) replaceLeadAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[recordAssignmentsRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	items, err := parseRecordAssignments(body.Assignments)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result assignment.Set
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ReplaceLeadAssignments(request.Context(), workspace,
				metadata(request, workspaceID, principal), leadID, version, items)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listDealAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result assignment.Set
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListDealAssignments(request.Context(), workspace, workspaceID, dealID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) replaceDealAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[recordAssignmentsRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	items, err := parseRecordAssignments(body.Assignments)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result assignment.Set
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ReplaceDealAssignments(request.Context(), workspace,
				metadata(request, workspaceID, principal), dealID, version, items)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func parseRecordAssignments(values []recordAssignmentRequest) ([]assignment.Input, error) {
	result := make([]assignment.Input, 0, len(values))
	for index, value := range values {
		subjectID, err := ids.Parse(strings.TrimSpace(value.SubjectID))
		if err != nil {
			return nil, &errx.ValidationError{Fields: []errx.FieldError{{
				Pointer: fmt.Sprintf("/assignments/%d/subjectId", index), Code: "validation.uuid.invalid",
			}}}
		}
		result = append(result, assignment.Input{Kind: value.Kind, SubjectType: value.SubjectType,
			SubjectID: subjectID, IsPrimary: value.IsPrimary})
	}
	return result, nil
}

func (application *Application) getLeadAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.LeadRecord
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.GetLead(request.Context(), workspace, workspaceID, leadID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createLeadAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[leadRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	input, err := parseLeadRequest(body)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionLeadsCreate, "leads.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			validatedFields, validationErr := application.customers.ValidateCustomFields(request.Context(), workspace, workspaceID, "lead", input.CustomFields)
			if validationErr != nil {
				return nil, 0, validationErr
			}
			input.CustomFields = validatedFields.Values
			lead, createErr := application.sales.CreateLead(request.Context(), workspace, metadata, input)
			if createErr != nil {
				return nil, 0, createErr
			}
			leadID, parseErr := ids.Parse(lead.ID)
			if parseErr != nil {
				return nil, 0, parseErr
			}
			if persistErr := application.customers.PersistValidatedCustomFields(request.Context(), workspace,
				workspaceID, "lead", leadID, validatedFields); persistErr != nil {
				return nil, 0, persistErr
			}
			return lead, lead.Version, nil
		})
}

func (application *Application) updateLeadAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[leadRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	input, err := parseLeadRequest(body)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.LeadRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			validatedFields, validationErr := application.customers.ValidateCustomFields(request.Context(), workspace, workspaceID, "lead", input.CustomFields)
			if validationErr != nil {
				return validationErr
			}
			input.CustomFields = validatedFields.Values
			result, err = application.sales.UpdateLead(request.Context(), workspace, metadata(request, workspaceID, principal), leadID, version, input)
			if err == nil {
				err = application.customers.PersistValidatedCustomFields(request.Context(), workspace,
					workspaceID, "lead", leadID, validatedFields)
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteLeadAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.sales.DeleteLead(request.Context(), workspace, metadata(request, workspaceID, principal), leadID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) listLeadTrash(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, sales.DefaultPageSize)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.DeletedSalesPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListLeadTrash(request.Context(), workspace, workspaceID, request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) restoreLeadAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.LeadRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionLeadsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.RestoreLead(request.Context(), workspace, metadata(request, workspaceID, principal), leadID, version)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) convertLeadAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, leadID, ok := salesWorkspaceAndResourceID(application, writer, request, "leadId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[leadConversionRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionLeadsUpdate,
		"leads.convert."+leadID.String(), raw, http.StatusOK,
		func(workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata) (any, int64, error) {
			result, convertErr := application.convertLeadInWorkspace(request, workspace, mutationMetadata, leadID, version, body)
			return result, result.Version, convertErr
		})
}

func (application *Application) convertLeadInWorkspace(
	request *http.Request,
	workspace *tenancy.WorkspaceTx,
	mutationMetadata events.Metadata,
	leadID ids.UUID,
	version int64,
	body leadConversionRequest,
) (leadConversionResponse, error) {
	if !workspace.Allows(tenancy.PermissionRecordsCreate) {
		return leadConversionResponse{}, errx.ErrForbidden
	}
	lead, err := application.sales.GetLead(request.Context(), workspace, mutationMetadata.WorkspaceID, leadID)
	if err != nil {
		return leadConversionResponse{}, err
	}
	contactID, err := parseOptionalStringID(body.ContactID, "/contactId")
	if err != nil {
		return leadConversionResponse{}, err
	}
	companyID, err := parseOptionalStringID(body.CompanyID, "/companyId")
	if err != nil {
		return leadConversionResponse{}, err
	}
	dealID, err := parseOptionalStringID(body.DealID, "/dealId")
	if err != nil {
		return leadConversionResponse{}, err
	}
	if body.CreateCompany || body.Company != nil {
		if companyID != nil {
			return leadConversionResponse{}, mutuallyExclusive("/companyId", "/company")
		}
		companyInput := customers.CompanyInput{Name: dereferenceString(lead.CompanyName), Status: "active", Address: map[string]string{}, CustomFields: map[string]any{}}
		if body.Company != nil {
			companyInput.Name = body.Company.Name
			companyInput.Domain = body.Company.Domain
			companyInput.Industry = body.Company.Industry
		}
		company, createErr := application.customers.CreateCompany(request.Context(), workspace, mutationMetadata, companyInput)
		if createErr != nil {
			return leadConversionResponse{}, createErr
		}
		createdID, ok := ids.FromPG(company.ID)
		if !ok {
			return leadConversionResponse{}, errx.ErrUnavailable
		}
		companyID = &createdID
	}
	if body.CreateContact || body.Contact != nil || contactID == nil {
		if contactID != nil {
			return leadConversionResponse{}, mutuallyExclusive("/contactId", "/contact")
		}
		contact, prepareErr := conversionContact(lead, body.Contact, companyID)
		if prepareErr != nil {
			return leadConversionResponse{}, prepareErr
		}
		created, createErr := application.customers.CreateContact(request.Context(), workspace, mutationMetadata, contact)
		if createErr != nil {
			return leadConversionResponse{}, createErr
		}
		createdID, ok := ids.FromPG(created.ID)
		if !ok {
			return leadConversionResponse{}, errx.ErrUnavailable
		}
		contactID = &createdID
	}
	if body.Deal != nil {
		if dealID != nil {
			return leadConversionResponse{}, mutuallyExclusive("/dealId", "/deal")
		}
		dealInput, parseErr := conversionDeal(body.Deal, contactID, companyID)
		if parseErr != nil {
			return leadConversionResponse{}, parseErr
		}
		created, createErr := application.sales.CreateDeal(request.Context(), workspace, mutationMetadata, dealInput)
		if createErr != nil {
			return leadConversionResponse{}, createErr
		}
		createdID, ok := ids.FromPG(created.ID)
		if !ok {
			return leadConversionResponse{}, errx.ErrUnavailable
		}
		dealID = &createdID
	}
	converted, err := application.sales.ConvertLead(request.Context(), workspace, mutationMetadata, leadID, version,
		sales.LeadConversionReferences{ContactID: contactID, CompanyID: companyID, DealID: dealID})
	if err != nil {
		return leadConversionResponse{}, err
	}
	return leadConversionResponse{
		Lead: converted, ContactID: contactID.String(), CompanyID: stringIDPointer(companyID),
		DealID: stringIDPointer(dealID), Version: converted.Version,
	}, nil
}

func parseLeadRequest(body leadRequest) (sales.LeadInput, error) {
	ownerID, err := parseOptionalStringID(body.OwnerID, "/ownerId")
	if err != nil {
		return sales.LeadInput{}, err
	}
	teamID, err := parseOptionalStringID(body.TeamID, "/teamId")
	if err != nil {
		return sales.LeadInput{}, err
	}
	stageID, err := parseOptionalStringID(body.StageID, "/stageId")
	if err != nil {
		return sales.LeadInput{}, err
	}
	plannedStartDate, err := parseOptionalISODate(body.PlannedStartDate, "/plannedStartDate")
	if err != nil {
		return sales.LeadInput{}, err
	}
	expectedCloseDate, err := parseOptionalISODate(body.ExpectedCloseDate, "/expectedCloseDate")
	if err != nil {
		return sales.LeadInput{}, err
	}
	return sales.LeadInput{
		Name: body.Name, Email: body.Email, Phone: body.Phone, CompanyName: body.CompanyName,
		JobTitle: body.JobTitle, Source: body.Source, Status: body.Status, StageID: stageID,
		OwnerID: ownerID, TeamID: teamID, PlannedStartDate: plannedStartDate,
		ExpectedCloseDate: expectedCloseDate, CustomFields: body.CustomFields,
	}, nil
}

func conversionContact(lead sales.LeadRecord, request *leadConversionContactRequest, companyID *ids.UUID) (customers.ContactInput, error) {
	firstName, lastName := splitContactName(lead.Name)
	result := customers.ContactInput{
		FirstName: firstName, LastName: lastName, Email: lead.Email, Phone: lead.Phone,
		JobTitle: lead.JobTitle, CompanyID: companyID, Status: "active", Source: lead.Source,
		Address: map[string]string{}, CustomFields: map[string]any{},
	}
	if request != nil {
		result.FirstName = request.FirstName
		result.LastName = request.LastName
		if request.Email != nil {
			result.Email = request.Email
		}
		if request.Phone != nil {
			result.Phone = request.Phone
		}
		if request.JobTitle != nil {
			result.JobTitle = request.JobTitle
		}
	}
	if strings.TrimSpace(result.FirstName) == "" || strings.TrimSpace(result.LastName) == "" {
		return customers.ContactInput{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/contact", Code: "validation.lead_conversion.contact_name_required",
		}}}
	}
	return result, nil
}

func conversionDeal(request *leadConversionDealRequest, contactID, companyID *ids.UUID) (sales.DealInput, error) {
	pipelineID, err := ids.Parse(strings.TrimSpace(request.PipelineID))
	if err != nil {
		return sales.DealInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/deal/pipelineId", Code: "validation.uuid.invalid"}}}
	}
	stageID, err := ids.Parse(strings.TrimSpace(request.StageID))
	if err != nil {
		return sales.DealInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/deal/stageId", Code: "validation.uuid.invalid"}}}
	}
	var closeDate *time.Time
	if request.ExpectedCloseDate != nil {
		value, parseErr := time.Parse("2006-01-02", *request.ExpectedCloseDate)
		if parseErr != nil || value.Format("2006-01-02") != *request.ExpectedCloseDate {
			return sales.DealInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/deal/expectedCloseDate", Code: "validation.date.invalid"}}}
		}
		closeDate = &value
	}
	return sales.DealInput{
		Name: request.Name, PipelineID: pipelineID, StageID: stageID, ContactID: contactID, CompanyID: companyID,
		AmountMinor: request.AmountMinor, Currency: request.Currency, ExpectedCloseDate: closeDate,
	}, nil
}

func splitContactName(name string) (string, string) {
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return strings.TrimSpace(name), ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func salesWorkspaceAndResourceID(application *Application, writer http.ResponseWriter, request *http.Request, pathKey string) (ids.UUID, ids.UUID, bool) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, false
	}
	resourceID, err := parsePathID(request, pathKey)
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, false
	}
	return workspaceID, resourceID, true
}

func mutuallyExclusive(first, second string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: first, Code: "validation.mutually_exclusive", Params: map[string]any{"other": second}}}}
}

func stringIDPointer(value *ids.UUID) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
