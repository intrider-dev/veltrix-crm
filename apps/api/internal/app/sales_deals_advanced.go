package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type dealUpdateRequest struct {
	Name              string         `json:"name"`
	PipelineID        string         `json:"pipelineId"`
	StageID           string         `json:"stageId"`
	ContactID         *string        `json:"contactId"`
	CompanyID         *string        `json:"companyId"`
	OwnerID           *string        `json:"ownerId"`
	AmountMinor       int64          `json:"amountMinor"`
	Currency          string         `json:"currency"`
	PlannedStartDate  *string        `json:"plannedStartDate"`
	ExpectedCloseDate *string        `json:"expectedCloseDate"`
	ForecastCategory  string         `json:"forecastCategory"`
	CustomFields      map[string]any `json:"customFields"`
}

type lineItemRequest struct {
	Name           string `json:"name"`
	Quantity       string `json:"quantity"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	Currency       string `json:"currency"`
	Position       int    `json:"position"`
	Version        int64  `json:"version,omitempty"`
}

type lineItemMutationResponse struct {
	Item    sales.LineItemRecord `json:"item"`
	Version int64                `json:"version"`
}

type dealParticipantRequest struct {
	ContactID string  `json:"contactId"`
	Role      *string `json:"role"`
}

type participantMutationResponse struct {
	Participant sales.DealParticipantRecord `json:"participant"`
	Version     int64                       `json:"version"`
}

func (application *Application) listDealsFull(writer http.ResponseWriter, request *http.Request) {
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
	ownerID, err := parseOptionalID(request.URL.Query().Get("ownerId"), "/query/ownerId")
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, sales.DefaultPageSize)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.DealPageAdvanced
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListDealsAdvanced(request.Context(), workspace, workspaceID, sales.DealListFilter{
				Query: request.URL.Query().Get("query"), PipelineID: pipelineID, StageID: stageID, OwnerID: ownerID,
				Status: request.URL.Query().Get("status"), Cursor: request.URL.Query().Get("cursor"), Limit: limit,
			})
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) getDealAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.DealRecord
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.GetDealRecord(request.Context(), workspace, workspaceID, dealID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) updateDealAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[dealUpdateRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	input, err := parseDealUpdateRequest(body)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.DealRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.UpdateDeal(request.Context(), workspace, metadata(request, workspaceID, principal), dealID, version, input)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) setDealOutcomeAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[sales.DealOutcomeInput](writer, request, 32<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.DealRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.SetDealOutcomeAdvanced(request.Context(), workspace, metadata(request, workspaceID, principal), dealID, version, body)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteDealAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.sales.DeleteDeal(request.Context(), workspace, metadata(request, workspaceID, principal), dealID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) restoreDealAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.DealRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.RestoreDeal(request.Context(), workspace, metadata(request, workspaceID, principal), dealID, version)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listDealTrash(writer http.ResponseWriter, request *http.Request) {
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
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListDealTrash(request.Context(), workspace, workspaceID, request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listKanbanStageDeals(writer http.ResponseWriter, request *http.Request) {
	workspaceID, pipelineID, ok := salesWorkspaceAndResourceID(application, writer, request, "pipelineId")
	if !ok {
		return
	}
	stageID, err := parsePathID(request, "stageId")
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, sales.MaxKanbanPageSize)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.KanbanPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListKanbanStage(request.Context(), workspace, workspaceID, pipelineID, stageID,
				request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listDealStageHistoryAdvanced(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	limit, err := parseLimit(request, sales.DefaultPageSize)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result sales.StageHistoryPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListDealStageHistory(request.Context(), workspace, workspaceID, dealID,
				request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listDealLineItems(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []sales.LineItemRecord
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListDealLineItems(request.Context(), workspace, workspaceID, dealID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createDealLineItem(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	dealVersion, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[lineItemRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionDealsUpdate,
		"deal-line-items.create."+dealID.String(), raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata) (any, int64, error) {
			item, newVersion, createErr := application.sales.CreateDealLineItem(request.Context(), workspace, mutationMetadata,
				dealID, dealVersion, sales.LineItemInput{Name: body.Name, Quantity: body.Quantity, UnitPriceMinor: body.UnitPriceMinor, Currency: body.Currency, Position: body.Position})
			return lineItemMutationResponse{Item: item, Version: newVersion}, newVersion, createErr
		})
}

func (application *Application) updateDealLineItem(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	itemID, err := parsePathID(request, "itemId")
	if writeError(application, writer, request, err) {
		return
	}
	dealVersion, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[lineItemRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result lineItemMutationResponse
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			item, newVersion, updateErr := application.sales.UpdateDealLineItem(request.Context(), workspace, metadata(request, workspaceID, principal),
				dealID, itemID, dealVersion, body.Version,
				sales.LineItemInput{Name: body.Name, Quantity: body.Quantity, UnitPriceMinor: body.UnitPriceMinor, Currency: body.Currency, Position: body.Position})
			result = lineItemMutationResponse{Item: item, Version: newVersion}
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteDealLineItem(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	itemID, err := parsePathID(request, "itemId")
	if writeError(application, writer, request, err) {
		return
	}
	dealVersion, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	itemVersion, err := parseResourceVersion(request, "itemVersion")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			newVersion, err = application.sales.DeleteDealLineItem(request.Context(), workspace, metadata(request, workspaceID, principal),
				dealID, itemID, dealVersion, itemVersion)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) listDealParticipants(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []sales.DealParticipantRecord
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.sales.ListDealParticipants(request.Context(), workspace, workspaceID, dealID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) upsertDealParticipant(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	dealVersion, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[dealParticipantRequest](writer, request, 32<<10)
	if writeError(application, writer, request, err) {
		return
	}
	contactID, err := ids.Parse(strings.TrimSpace(body.ContactID))
	if err != nil {
		writeError(application, writer, request, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/contactId", Code: "validation.uuid.invalid"}}})
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionDealsUpdate,
		"deal-participants.upsert."+dealID.String(), raw, http.StatusOK,
		func(workspace *tenancy.WorkspaceTx, mutationMetadata events.Metadata) (any, int64, error) {
			participant, newVersion, updateErr := application.sales.UpsertDealParticipant(request.Context(), workspace, mutationMetadata,
				dealID, dealVersion, sales.DealParticipantInput{ContactID: contactID, Role: body.Role})
			return participantMutationResponse{Participant: participant, Version: newVersion}, newVersion, updateErr
		})
}

func (application *Application) deleteDealParticipant(writer http.ResponseWriter, request *http.Request) {
	workspaceID, dealID, ok := salesWorkspaceAndResourceID(application, writer, request, "dealId")
	if !ok {
		return
	}
	contactID, err := parsePathID(request, "contactId")
	if writeError(application, writer, request, err) {
		return
	}
	dealVersion, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	participantVersion, err := parseResourceVersion(request, "participantVersion")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDealsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			newVersion, err = application.sales.DeleteDealParticipant(request.Context(), workspace, metadata(request, workspaceID, principal),
				dealID, contactID, dealVersion, participantVersion)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	writer.WriteHeader(http.StatusNoContent)
}

func parseDealUpdateRequest(body dealUpdateRequest) (sales.DealUpdateInput, error) {
	pipelineID, err := ids.Parse(strings.TrimSpace(body.PipelineID))
	if err != nil {
		return sales.DealUpdateInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/pipelineId", Code: "validation.uuid.invalid"}}}
	}
	stageID, err := ids.Parse(strings.TrimSpace(body.StageID))
	if err != nil {
		return sales.DealUpdateInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/stageId", Code: "validation.uuid.invalid"}}}
	}
	contactID, err := parseOptionalStringID(body.ContactID, "/contactId")
	if err != nil {
		return sales.DealUpdateInput{}, err
	}
	companyID, err := parseOptionalStringID(body.CompanyID, "/companyId")
	if err != nil {
		return sales.DealUpdateInput{}, err
	}
	ownerID, err := parseOptionalStringID(body.OwnerID, "/ownerId")
	if err != nil {
		return sales.DealUpdateInput{}, err
	}
	plannedStartDate, err := parseOptionalISODate(body.PlannedStartDate, "/plannedStartDate")
	if err != nil {
		return sales.DealUpdateInput{}, err
	}
	var closeDate *time.Time
	if body.ExpectedCloseDate != nil {
		parsed, parseErr := time.Parse("2006-01-02", strings.TrimSpace(*body.ExpectedCloseDate))
		if parseErr != nil || parsed.Format("2006-01-02") != strings.TrimSpace(*body.ExpectedCloseDate) {
			return sales.DealUpdateInput{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/expectedCloseDate", Code: "validation.date.invalid"}}}
		}
		closeDate = &parsed
	}
	return sales.DealUpdateInput{
		Name: body.Name, PipelineID: pipelineID, StageID: stageID, ContactID: contactID,
		CompanyID: companyID, OwnerID: ownerID, AmountMinor: body.AmountMinor, Currency: body.Currency,
		PlannedStartDate: plannedStartDate, ExpectedCloseDate: closeDate,
		ForecastCategory: body.ForecastCategory, CustomFields: body.CustomFields,
	}, nil
}

func parseOptionalISODate(value *string, pointer string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil || parsed.Format("2006-01-02") != trimmed {
		return nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: "validation.date.invalid"}}}
	}
	return &parsed, nil
}

func parseResourceVersion(request *http.Request, name string) (int64, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	version, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || version < 1 {
		return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/query/" + name, Code: "validation.version.invalid"}}}
	}
	return version, nil
}
