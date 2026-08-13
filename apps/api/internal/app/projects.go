package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/projects"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) registerProjectRoutes(router chi.Router) {
	router.Get("/projects", application.listProjects)
	router.Post("/projects", application.createProject)
	router.Get("/projects/{projectId}", application.getProject)
	router.Put("/projects/{projectId}", application.updateProject)
	router.Delete("/projects/{projectId}", application.deleteProject)
	router.Get("/projects/{projectId}/assignments", application.listProjectAssignments)
	router.Put("/projects/{projectId}/assignments", application.replaceProjectAssignments)
}

func (application *Application) listProjects(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 25)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result projects.Page
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.projects.List(request.Context(), workspace, workspaceID,
				request.URL.Query().Get("status"), request.URL.Query().Get("cursor"), limit)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createProject(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.ProjectInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	input := projectInput(body)
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate,
		"projects.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			created, createErr := application.projects.Create(request.Context(), workspace, metadata, input)
			return created, created.Version, createErr
		})
}

func (application *Application) getProject(writer http.ResponseWriter, request *http.Request) {
	workspaceID, projectID, ok := application.workspaceAndPathID(writer, request, "projectId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result projects.Record
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.projects.Get(request.Context(), workspace, workspaceID, projectID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) updateProject(writer http.ResponseWriter, request *http.Request) {
	workspaceID, projectID, ok := application.workspaceAndPathID(writer, request, "projectId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[apigen.ProjectInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result projects.Record
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.projects.Update(request.Context(), workspace,
				metadata(request, workspaceID, principal), projectID, version, projectInput(body))
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteProject(writer http.ResponseWriter, request *http.Request) {
	workspaceID, projectID, ok := application.workspaceAndPathID(writer, request, "projectId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.projects.Delete(request.Context(), workspace,
				metadata(request, workspaceID, principal), projectID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) listProjectAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, projectID, ok := application.workspaceAndPathID(writer, request, "projectId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result projects.AssignmentSet
	var err error
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.projects.ListAssignments(request.Context(), workspace, workspaceID, projectID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) replaceProjectAssignments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, projectID, ok := application.workspaceAndPathID(writer, request, "projectId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[apigen.ProjectAssignmentsInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	items := make([]projects.AssignmentInput, 0, len(body.Assignments))
	for _, item := range body.Assignments {
		subjectID := ids.UUID(item.SubjectId)
		items = append(items, projects.AssignmentInput{Kind: string(item.Kind), SubjectType: string(item.SubjectType), SubjectID: subjectID})
	}
	principal, _ := httpx.Principal(request.Context())
	var result projects.AssignmentSet
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = application.projects.ReplaceAssignments(request.Context(), workspace,
				metadata(request, workspaceID, principal), projectID, version, items)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func projectInput(body apigen.ProjectInput) projects.Input {
	return projects.Input{Name: body.Name, Description: body.Description, Status: string(body.Status),
		Visibility: string(body.Visibility), PlannedStartDate: apiDateTime(body.PlannedStartDate),
		TargetEndDate: apiDateTime(body.TargetEndDate), OwnerUserID: internalIDPointer(body.OwnerUserId)}
}

func apiDateTime(value *openapi_types.Date) *time.Time {
	if value == nil {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
