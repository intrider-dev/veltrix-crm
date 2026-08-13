package app

import (
	"net/http"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/activities"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) listActivities(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	entityID, err := parseOptionalID(request.URL.Query().Get("entityId"), "/query/entityId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []apigen.Activity
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			rows, loadErr := application.activities.List(
				request.Context(), workspace, workspaceID, request.URL.Query().Get("entityType"), entityID, 100,
			)
			if loadErr != nil {
				return loadErr
			}
			result = make([]apigen.Activity, 0, len(rows))
			for _, row := range rows {
				result = append(result, activityFromList(row))
			}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createActivity(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateActivity](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	var relatedType *string
	if body.RelatedType != nil {
		value := string(*body.RelatedType)
		relatedType = &value
	}
	priority := ""
	if body.Priority != nil {
		priority = string(*body.Priority)
	}
	var dueAt *time.Time
	if body.DueAt != nil {
		value := body.DueAt.UTC()
		dueAt = &value
	}
	input := activities.Input{
		Type: string(body.Type), Title: body.Title, Body: body.Body,
		RelatedType: relatedType, RelatedID: internalIDPointer(body.RelatedId),
		AssigneeID: internalIDPointer(body.AssigneeId), Priority: priority, DueAt: dueAt,
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate, "activities.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			row, createErr := application.activities.Create(request.Context(), workspace, metadata, input)
			if createErr != nil {
				return nil, 0, createErr
			}
			return activityFromCreate(row), row.Version, nil
		})
}

func (application *Application) completeActivity(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	activityID, err := parsePathID(request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result apigen.Activity
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			row, completeErr := application.activities.Complete(
				request.Context(), workspace, metadata(request, workspaceID, principal), activityID, version,
			)
			if completeErr == nil {
				result = activityFromComplete(row)
				newVersion = row.Version
			}
			return completeErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	httpx.WriteJSON(writer, http.StatusOK, result)
}
