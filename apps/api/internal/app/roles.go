package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) registerRoleRoutes(router chi.Router) {
	router.Get("/roles", application.listWorkspaceRoles)
	router.Post("/roles", application.createWorkspaceRole)
	router.Patch("/roles/{roleId}", application.updateWorkspaceRole)
	router.Delete("/roles/{roleId}", application.deleteWorkspaceRole)
	router.Patch("/members/{membershipId}/role-assignment", application.assignWorkspaceRole)
}

func (application *Application) listWorkspaceRoles(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	roles, err := application.tenancy.ListRoles(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	result := make([]apigen.WorkspaceRoleDefinition, 0, len(roles))
	for _, role := range roles {
		result = append(result, workspaceRoleResponse(role))
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createWorkspaceRole(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[apigen.WorkspaceRoleInput](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	role, err := application.tenancy.CreateRole(
		request.Context(), principal, metadata(request, workspaceID, principal), workspaceRoleInput(body),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	setETag(writer, role.Version)
	httpx.WriteJSON(writer, http.StatusCreated, workspaceRoleResponse(role))
}

func (application *Application) updateWorkspaceRole(writer http.ResponseWriter, request *http.Request) {
	workspaceID, roleID, ok := application.workspaceAndPathID(writer, request, "roleId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[apigen.WorkspaceRoleInput](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	role, err := application.tenancy.UpdateRole(
		request.Context(), principal, metadata(request, workspaceID, principal), roleID, version,
		workspaceRoleInput(body),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	setETag(writer, role.Version)
	httpx.WriteJSON(writer, http.StatusOK, workspaceRoleResponse(role))
}

func (application *Application) deleteWorkspaceRole(writer http.ResponseWriter, request *http.Request) {
	workspaceID, roleID, ok := application.workspaceAndPathID(writer, request, "roleId")
	if !ok {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.DeleteRole(
		request.Context(), principal, metadata(request, workspaceID, principal), roleID, version,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) assignWorkspaceRole(writer http.ResponseWriter, request *http.Request) {
	workspaceID, membershipID, ok := application.workspaceAndPathID(writer, request, "membershipId")
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[apigen.WorkspaceRoleAssignmentInput](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.tenancy.AssignRole(
		request.Context(), principal, metadata(request, workspaceID, principal), membershipID,
		ids.UUID(body.RoleId),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, membershipResponse(row))
}

func workspaceRoleInput(body apigen.WorkspaceRoleInput) tenancy.WorkspaceRoleInput {
	permissions := make([]tenancy.Permission, len(body.Permissions))
	for index, permission := range body.Permissions {
		permissions[index] = tenancy.Permission(permission)
	}
	return tenancy.WorkspaceRoleInput{Name: body.Name, BaseRole: string(body.BaseRole), Permissions: permissions}
}

func workspaceRoleResponse(role tenancy.WorkspaceRoleDefinition) apigen.WorkspaceRoleDefinition {
	permissions := make([]apigen.Permission, len(role.Permissions))
	for index, permission := range role.Permissions {
		permissions[index] = apigen.Permission(permission)
	}
	return apigen.WorkspaceRoleDefinition{
		Id: uuid.UUID(role.ID), Key: role.Key, Name: role.Name, BaseRole: apigen.WorkspaceRole(role.BaseRole),
		System: role.IsSystem, Permissions: permissions, Version: role.Version,
		CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt,
	}
}
