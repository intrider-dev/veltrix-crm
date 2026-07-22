package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type createWorkspaceRequest struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	DefaultLocale   string `json:"defaultLocale"`
	Timezone        string `json:"timezone"`
	DefaultCurrency string `json:"defaultCurrency"`
}

type inviteMemberRequest struct {
	Email          string `json:"email"`
	Role           string `json:"role"`
	ExpiresInHours int    `json:"expiresInHours"`
}

type acceptInvitationRequest struct {
	Token string `json:"token"`
}

type memberRoleRequest struct {
	Role string `json:"role"`
}

type memberStatusRequest struct {
	Status string `json:"status"`
}

type workspaceDefaultsRequest struct {
	DefaultLocale   string `json:"defaultLocale"`
	Timezone        string `json:"timezone"`
	DefaultCurrency string `json:"defaultCurrency"`
}

type workspaceLocaleRequest struct {
	Locale *string `json:"locale"`
}

type createTeamRequest struct {
	Name string `json:"name"`
}

func (application *Application) createWorkspace(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[createWorkspaceRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	result, err := application.tenancy.CreateWorkspace(
		request.Context(), principal, httpx.RequestID(request.Context()), tenancy.CreateWorkspaceRequest{
			Name: body.Name, Slug: body.Slug, DefaultLocale: body.DefaultLocale,
			Timezone: body.Timezone, DefaultCurrency: body.DefaultCurrency,
		},
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, map[string]any{
		"id": apiID(result.Workspace.ID), "name": result.Workspace.Name,
		"slug": result.Workspace.Slug, "role": result.Membership.Role,
		"defaultLocale":    result.Workspace.DefaultLocale,
		"supportedLocales": result.Workspace.SupportedLocales,
		"timezone":         result.Workspace.Timezone,
		"defaultCurrency":  result.Workspace.DefaultCurrency,
		"version":          result.Workspace.Version,
	})
}

func (application *Application) inviteMember(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[inviteMemberRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	if body.ExpiresInHours == 0 {
		body.ExpiresInHours = 72
	}
	principal, _ := httpx.Principal(request.Context())
	invitation, err := application.tenancy.InviteMember(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()),
		body.Email, body.Role, time.Duration(body.ExpiresInHours)*time.Hour,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	// The raw token is returned exactly once. A configured delivery worker can
	// send it by email without ever persisting plaintext.
	httpx.WriteJSON(writer, http.StatusCreated, map[string]any{
		"id": invitation.ID.String(), "email": invitation.Email, "role": invitation.Role,
		"token": invitation.Token, "expiresAt": invitation.ExpiresAt,
	})
}

func (application *Application) acceptInvitation(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[acceptInvitationRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	membership, err := application.tenancy.AcceptInvitation(
		request.Context(), principal, body.Token, httpx.RequestID(request.Context()),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, membershipResponse(membership))
}

func (application *Application) listMembers(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	rows, err := application.tenancy.ListMembers(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": apiID(row.ID), "userId": apiID(row.UserID), "email": row.Email,
			"displayName": row.DisplayName, "role": row.Role, "roleId": apiID(row.RoleID),
			"roleName": row.RoleName, "status": row.Status,
			"preferredLocale": row.PreferredLocale, "localeOverride": row.LocaleOverride,
			"createdAt": row.CreatedAt.Time.UTC(), "updatedAt": row.UpdatedAt.Time.UTC(),
		})
	}
	httpx.WriteJSON(writer, http.StatusOK, items)
}

func (application *Application) updateMemberRole(writer http.ResponseWriter, request *http.Request) {
	workspaceID, membershipID, ok := application.workspaceAndPathID(writer, request, "membershipId")
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[memberRoleRequest](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.tenancy.UpdateMemberRole(
		request.Context(), principal, workspaceID, membershipID,
		httpx.RequestID(request.Context()), body.Role,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, membershipResponse(row))
}

func (application *Application) updateMemberStatus(writer http.ResponseWriter, request *http.Request) {
	workspaceID, membershipID, ok := application.workspaceAndPathID(writer, request, "membershipId")
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[memberStatusRequest](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.tenancy.SetMemberStatus(
		request.Context(), principal, workspaceID, membershipID,
		httpx.RequestID(request.Context()), body.Status,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, membershipResponse(row))
}

func (application *Application) updateWorkspaceDefaults(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[workspaceDefaultsRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.tenancy.UpdateWorkspaceDefaults(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()),
		body.DefaultLocale, body.Timezone, body.DefaultCurrency, version,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	setETag(writer, row.Version)
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{
		"id": apiID(row.ID), "name": row.Name, "slug": row.Slug,
		"defaultLocale": row.DefaultLocale, "supportedLocales": row.SupportedLocales,
		"timezone": row.Timezone, "defaultCurrency": row.DefaultCurrency,
		"version": row.Version, "updatedAt": row.UpdatedAt.Time.UTC(),
	})
}

func (application *Application) setMyWorkspaceLocale(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[workspaceLocaleRequest](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.tenancy.SetMyWorkspaceLocale(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), body.Locale,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, membershipResponse(row))
}

func (application *Application) createTeam(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[createTeamRequest](writer, request, 8<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	row, err := application.tenancy.CreateTeam(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), body.Name,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, teamResponse(row))
}

func (application *Application) listTeams(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	rows, err := application.tenancy.ListTeams(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, teamResponse(row))
	}
	httpx.WriteJSON(writer, http.StatusOK, items)
}

func (application *Application) listTeamMembers(writer http.ResponseWriter, request *http.Request) {
	workspaceID, teamID, ok := application.workspaceAndPathID(writer, request, "teamId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	rows, err := application.tenancy.ListTeamMembers(
		request.Context(), principal, workspaceID, teamID, httpx.RequestID(request.Context()),
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"membershipId": apiID(row.MembershipID), "userId": apiID(row.UserID),
			"email": row.Email, "displayName": row.DisplayName,
		})
	}
	httpx.WriteJSON(writer, http.StatusOK, items)
}

func (application *Application) setTeamMember(writer http.ResponseWriter, request *http.Request, present bool) {
	workspaceID, teamID, ok := application.workspaceAndPathID(writer, request, "teamId")
	if !ok {
		return
	}
	membershipID, err := parsePathID(request, "membershipId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.SetTeamMember(
		request.Context(), principal, workspaceID, teamID, membershipID,
		httpx.RequestID(request.Context()), present,
	)
	if writeError(application, writer, request, mapTenancyError(err)) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) addTeamMember(writer http.ResponseWriter, request *http.Request) {
	application.setTeamMember(writer, request, true)
}

func (application *Application) removeTeamMember(writer http.ResponseWriter, request *http.Request) {
	application.setTeamMember(writer, request, false)
}

func (application *Application) workspaceAndPathID(
	writer http.ResponseWriter,
	request *http.Request,
	pathName string,
) (workspaceID, entityID ids.UUID, ok bool) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return workspaceID, entityID, false
	}
	entityID, err = parsePathID(request, pathName)
	if writeError(application, writer, request, err) {
		return workspaceID, entityID, false
	}
	return workspaceID, entityID, true
}

func membershipResponse(row dbgen.TenancyMembership) map[string]any {
	return map[string]any{
		"id": apiID(row.ID), "userId": apiID(row.UserID), "role": row.Role,
		"roleId": apiID(row.RoleID),
		"status": row.Status, "localeOverride": row.LocaleOverride,
		"timezoneOverride": row.TimezoneOverride,
		"createdAt":        row.CreatedAt.Time.UTC(), "updatedAt": row.UpdatedAt.Time.UTC(),
	}
}

func teamResponse(row dbgen.TenancyTeam) map[string]any {
	return map[string]any{
		"id": apiID(row.ID), "name": row.Name, "version": row.Version,
		"createdAt": row.CreatedAt.Time.UTC(), "updatedAt": row.UpdatedAt.Time.UTC(),
	}
}

func mapTenancyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tenancy.ErrInvalidInvitation):
		return errx.ErrNotFound
	case errors.Is(err, tenancy.ErrLastOwner):
		return errx.ErrConflict
	case errors.Is(err, tenancy.ErrCannotManageOwner):
		return errx.ErrForbidden
	default:
		return err
	}
}
