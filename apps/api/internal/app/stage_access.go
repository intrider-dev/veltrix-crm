package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type stageAccessRuleRequest struct {
	RoleID   string `json:"roleId"`
	CanView  bool   `json:"canView"`
	CanEnter bool   `json:"canEnter"`
	CanLeave bool   `json:"canLeave"`
}

type stageAccessRequest struct {
	Rules []stageAccessRuleRequest `json:"rules"`
}

type stageAccessRuleResponse struct {
	RoleID    string    `json:"roleId"`
	RoleKey   string    `json:"roleKey"`
	RoleName  string    `json:"roleName"`
	BaseRole  string    `json:"baseRole"`
	CanView   bool      `json:"canView"`
	CanEnter  bool      `json:"canEnter"`
	CanLeave  bool      `json:"canLeave"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (application *Application) listLeadStageAccess(writer http.ResponseWriter, request *http.Request) {
	application.listStageAccess(writer, request, tenancy.PermissionLeadStagesManage, true)
}

func (application *Application) replaceLeadStageAccess(writer http.ResponseWriter, request *http.Request) {
	application.replaceStageAccess(writer, request, tenancy.PermissionLeadStagesManage, true)
}

func (application *Application) listPipelineStageAccess(writer http.ResponseWriter, request *http.Request) {
	application.listStageAccess(writer, request, tenancy.PermissionDealStagesManage, false)
}

func (application *Application) replacePipelineStageAccess(writer http.ResponseWriter, request *http.Request) {
	application.replaceStageAccess(writer, request, tenancy.PermissionDealStagesManage, false)
}

func (application *Application) listStageAccess(
	writer http.ResponseWriter,
	request *http.Request,
	permission tenancy.Permission,
	leadStage bool,
) {
	workspaceID, stageID, ok := salesWorkspaceAndResourceID(application, writer, request, "stageId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var rules []sales.StageRoleAccessRule
	err := application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), permission,
		func(workspace *tenancy.WorkspaceTx) error {
			var listErr error
			if leadStage {
				rules, listErr = application.sales.ListLeadStageRoleAccess(request.Context(), workspace, workspaceID, stageID)
			} else {
				rules, listErr = application.sales.ListPipelineStageRoleAccess(request.Context(), workspace, workspaceID, stageID)
			}
			return listErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, stageAccessResponses(rules))
}

func (application *Application) replaceStageAccess(
	writer http.ResponseWriter,
	request *http.Request,
	permission tenancy.Permission,
	leadStage bool,
) {
	workspaceID, stageID, ok := salesWorkspaceAndResourceID(application, writer, request, "stageId")
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[stageAccessRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	inputs := make([]sales.StageRoleAccessInput, 0, len(body.Rules))
	for index, rule := range body.Rules {
		roleID, parseErr := ids.Parse(strings.TrimSpace(rule.RoleID))
		if parseErr != nil {
			writeError(application, writer, request, &errx.ValidationError{Fields: []errx.FieldError{{
				Pointer: "/rules/" + strconv.Itoa(index) + "/roleId", Code: "validation.uuid.invalid",
			}}})
			return
		}
		inputs = append(inputs, sales.StageRoleAccessInput{
			RoleID: roleID, CanView: rule.CanView, CanEnter: rule.CanEnter, CanLeave: rule.CanLeave,
		})
	}
	principal, _ := httpx.Principal(request.Context())
	var rules []sales.StageRoleAccessRule
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), permission,
		func(workspace *tenancy.WorkspaceTx) error {
			var replaceErr error
			if leadStage {
				rules, replaceErr = application.sales.ReplaceLeadStageRoleAccess(
					request.Context(), workspace, metadata(request, workspaceID, principal), stageID, inputs,
				)
			} else {
				rules, replaceErr = application.sales.ReplacePipelineStageRoleAccess(
					request.Context(), workspace, metadata(request, workspaceID, principal), stageID, inputs,
				)
			}
			return replaceErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, stageAccessResponses(rules))
}

func stageAccessResponses(rules []sales.StageRoleAccessRule) []stageAccessRuleResponse {
	result := make([]stageAccessRuleResponse, 0, len(rules))
	for _, rule := range rules {
		result = append(result, stageAccessRuleResponse{
			RoleID: rule.RoleID.String(), RoleKey: rule.RoleKey, RoleName: rule.RoleName, BaseRole: rule.BaseRole,
			CanView: rule.CanView, CanEnter: rule.CanEnter, CanLeave: rule.CanLeave,
			CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
		})
	}
	return result
}
