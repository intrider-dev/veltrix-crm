package app

import (
	"net/http"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/assignment"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) listAssignmentSubjects(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	result := make([]assignment.SubjectOption, 0)
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			users, loadErr := workspace.Queries.ListAssignmentUserOptions(request.Context(), workspaceID.PG())
			if loadErr != nil {
				return loadErr
			}
			departments, loadErr := workspace.Queries.ListAssignmentDepartmentOptions(request.Context(), workspaceID.PG())
			if loadErr != nil {
				return loadErr
			}
			result = make([]assignment.SubjectOption, 0, len(users)+len(departments))
			for _, user := range users {
				id, _ := ids.FromPG(user.ID)
				result = append(result, assignment.SubjectOption{Type: "user", ID: id.String(), Name: user.Name})
			}
			for _, department := range departments {
				id, _ := ids.FromPG(department.ID)
				result = append(result, assignment.SubjectOption{Type: "department", ID: id.String(), Name: department.Name})
			}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listTaskAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, ok := application.workspaceAndPathID(writer, request, "activityId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result assignment.Set
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.activities.ListTaskAssignments(request.Context(), workspace, workspaceID, activityID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) replaceTaskAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, ok := application.workspaceAndPathID(writer, request, "activityId")
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
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.activities.ReplaceTaskAssignments(request.Context(), workspace,
				metadata(request, workspaceID, principal), activityID, version, items)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}
