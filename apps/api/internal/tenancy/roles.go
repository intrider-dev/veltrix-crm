package tenancy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type WorkspaceRoleDefinition struct {
	ID          ids.UUID
	Key         string
	Name        string
	BaseRole    string
	IsSystem    bool
	Permissions []Permission
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorkspaceRoleInput struct {
	Name        string
	BaseRole    string
	Permissions []Permission
}

func (service *Service) ListRoles(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
) ([]WorkspaceRoleDefinition, error) {
	var result []WorkspaceRoleDefinition
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersRead,
		func(workspace *WorkspaceTx) error {
			rows, err := workspace.Queries.ListWorkspaceRoles(ctx, workspaceID.PG())
			if err != nil {
				return fmt.Errorf("list workspace roles: %w", err)
			}
			result = make([]WorkspaceRoleDefinition, 0, len(rows))
			for _, row := range rows {
				roleID, ok := ids.FromPG(row.ID)
				if !ok {
					return fmt.Errorf("workspace role has invalid identifier")
				}
				permissions := make([]Permission, 0, len(row.Permissions))
				for _, permission := range row.Permissions {
					permissions = append(permissions, Permission(permission))
				}
				result = append(result, WorkspaceRoleDefinition{
					ID: roleID, Key: row.RoleKey, Name: row.Name, BaseRole: row.BaseRole,
					IsSystem: row.IsSystem, Permissions: permissions, Version: row.Version,
					CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
				})
			}
			return nil
		})
	return result, err
}

func (service *Service) CreateRole(
	ctx context.Context,
	principal identity.Principal,
	metadata events.Metadata,
	input WorkspaceRoleInput,
) (WorkspaceRoleDefinition, error) {
	validated, err := validateWorkspaceRoleInput(input)
	if err != nil {
		return WorkspaceRoleDefinition{}, err
	}
	roleID, err := ids.NewV7()
	if err != nil {
		return WorkspaceRoleDefinition{}, err
	}
	var result WorkspaceRoleDefinition
	err = service.WithWorkspace(ctx, principal, metadata.WorkspaceID, metadata.RequestID, PermissionRolesWrite,
		func(workspace *WorkspaceTx) error {
			row, err := workspace.Queries.CreateWorkspaceRole(ctx, dbgen.CreateWorkspaceRoleParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: roleID.PG(),
				RoleKey: "custom_" + strings.ReplaceAll(roleID.String(), "-", ""),
				Name:    validated.Name, BaseRole: validated.BaseRole,
			})
			if err != nil {
				return mapWorkspaceRoleConstraint(err, "/name")
			}
			if err := replaceWorkspaceRolePermissions(ctx, workspace.Queries, metadata.WorkspaceID, roleID, validated.Permissions); err != nil {
				return err
			}
			result = workspaceRoleFromCreateRow(row, validated.Permissions)
			return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
				Action: "workspace_role.created", EventType: "tenancy.workspace_role.created",
				AggregateType: "workspace_role", AggregateID: roleID,
				Summary: map[string]any{"name": validated.Name, "baseRole": validated.BaseRole,
					"permissions": permissionStrings(validated.Permissions)},
				Payload: map[string]any{"roleId": roleID.String(), "version": row.Version},
			})
		})
	return result, err
}

func (service *Service) UpdateRole(
	ctx context.Context,
	principal identity.Principal,
	metadata events.Metadata,
	roleID ids.UUID,
	expectedVersion int64,
	input WorkspaceRoleInput,
) (WorkspaceRoleDefinition, error) {
	validated, err := validateWorkspaceRoleInput(input)
	if err != nil {
		return WorkspaceRoleDefinition{}, err
	}
	var result WorkspaceRoleDefinition
	err = service.WithWorkspace(ctx, principal, metadata.WorkspaceID, metadata.RequestID, PermissionRolesWrite,
		func(workspace *WorkspaceTx) error {
			existing, err := workspace.Queries.GetWorkspaceRoleForUpdate(ctx, dbgen.GetWorkspaceRoleForUpdateParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: roleID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("lock workspace role: %w", err)
			}
			if existing.IsSystem {
				return errx.ErrForbidden
			}
			if existing.Version != expectedVersion {
				return errx.ErrVersionConflict
			}
			row, err := workspace.Queries.UpdateWorkspaceRole(ctx, dbgen.UpdateWorkspaceRoleParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: roleID.PG(), Name: validated.Name,
				BaseRole: validated.BaseRole, Version: expectedVersion,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrVersionConflict
			}
			if err != nil {
				return mapWorkspaceRoleConstraint(err, "/name")
			}
			if err := replaceWorkspaceRolePermissions(ctx, workspace.Queries, metadata.WorkspaceID, roleID, validated.Permissions); err != nil {
				return err
			}
			result = workspaceRoleFromUpdateRow(row, validated.Permissions)
			return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
				Action: "workspace_role.updated", EventType: "tenancy.workspace_role.updated",
				AggregateType: "workspace_role", AggregateID: roleID,
				Summary: map[string]any{"name": validated.Name, "baseRole": validated.BaseRole,
					"permissions": permissionStrings(validated.Permissions)},
				Payload: map[string]any{"roleId": roleID.String(), "version": row.Version},
			})
		})
	return result, err
}

func (service *Service) DeleteRole(
	ctx context.Context,
	principal identity.Principal,
	metadata events.Metadata,
	roleID ids.UUID,
	expectedVersion int64,
) error {
	return service.WithWorkspace(ctx, principal, metadata.WorkspaceID, metadata.RequestID, PermissionRolesWrite,
		func(workspace *WorkspaceTx) error {
			existing, err := workspace.Queries.GetWorkspaceRoleForUpdate(ctx, dbgen.GetWorkspaceRoleForUpdateParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: roleID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("lock workspace role: %w", err)
			}
			if existing.IsSystem {
				return errx.ErrForbidden
			}
			if existing.Version != expectedVersion {
				return errx.ErrVersionConflict
			}
			changed, err := workspace.Queries.DeleteWorkspaceRole(ctx, dbgen.DeleteWorkspaceRoleParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: roleID.PG(), Version: expectedVersion,
			})
			if err != nil {
				return mapWorkspaceRoleConstraint(err, "/roleId")
			}
			if changed == 0 {
				return errx.ErrVersionConflict
			}
			return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
				Action: "workspace_role.deleted", EventType: "tenancy.workspace_role.deleted",
				AggregateType: "workspace_role", AggregateID: roleID,
				Summary: map[string]any{"deleted": true},
				Payload: map[string]any{"roleId": roleID.String()},
			})
		})
}

func (service *Service) AssignRole(
	ctx context.Context,
	principal identity.Principal,
	metadata events.Metadata,
	membershipID, roleID ids.UUID,
) (dbgen.TenancyMembership, error) {
	var result dbgen.TenancyMembership
	err := service.WithWorkspace(ctx, principal, metadata.WorkspaceID, metadata.RequestID, PermissionRolesWrite,
		func(workspace *WorkspaceTx) error {
			if err := workspace.Queries.LockWorkspaceMembershipMutation(ctx, metadata.WorkspaceID.String()); err != nil {
				return fmt.Errorf("lock workspace membership set: %w", err)
			}
			target, err := workspace.Queries.GetMembershipByIDForUpdate(ctx, dbgen.GetMembershipByIDForUpdateParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: membershipID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("lock membership: %w", err)
			}
			if target.Role == "owner" {
				return ErrCannotManageOwner
			}
			if _, err := workspace.Queries.GetWorkspaceRoleForUpdate(ctx, dbgen.GetWorkspaceRoleForUpdateParams{
				WorkspaceID: metadata.WorkspaceID.PG(), ID: roleID.PG(),
			}); errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			} else if err != nil {
				return fmt.Errorf("lock target workspace role: %w", err)
			}
			result, err = workspace.Queries.AssignMembershipWorkspaceRole(ctx, dbgen.AssignMembershipWorkspaceRoleParams{
				WorkspaceID: metadata.WorkspaceID.PG(), MembershipID: membershipID.PG(), RoleID: roleID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrForbidden
			}
			if err != nil {
				return fmt.Errorf("assign workspace role: %w", err)
			}
			targetUserID, ok := ids.FromPG(result.UserID)
			if !ok {
				return fmt.Errorf("membership has invalid user identifier")
			}
			return events.RecordTargeted(ctx, workspace.Queries, metadata, events.Mutation{
				Action: "membership.role_assigned", EventType: "authorization.changed",
				AggregateType: "membership", AggregateID: membershipID,
				Summary: map[string]any{"roleId": roleID.String()},
				Payload: map[string]any{"membershipId": membershipID.String()},
			}, []ids.UUID{targetUserID})
		})
	return result, err
}

func validateWorkspaceRoleInput(input WorkspaceRoleInput) (WorkspaceRoleInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.BaseRole = strings.TrimSpace(input.BaseRole)
	fields := make([]errx.FieldError, 0, 3)
	if count := utf8.RuneCountInString(input.Name); count < 1 || count > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if input.BaseRole == "owner" || !validRole(input.BaseRole) {
		fields = append(fields, errx.FieldError{Pointer: "/baseRole", Code: "validation.role"})
	}
	unique := make(map[Permission]struct{}, len(input.Permissions))
	for index, permission := range input.Permissions {
		if !knownPermission(permission) || permission == PermissionRolesWrite || !RoleAllows(input.BaseRole, permission) {
			fields = append(fields, errx.FieldError{
				Pointer: fmt.Sprintf("/permissions/%d", index), Code: "validation.permission.notAllowed",
			})
			continue
		}
		unique[permission] = struct{}{}
	}
	if len(fields) > 0 {
		return WorkspaceRoleInput{}, &errx.ValidationError{Fields: fields}
	}
	input.Permissions = make([]Permission, 0, len(unique))
	for permission := range unique {
		input.Permissions = append(input.Permissions, permission)
	}
	sort.Slice(input.Permissions, func(i, j int) bool { return input.Permissions[i] < input.Permissions[j] })
	return input, nil
}

func replaceWorkspaceRolePermissions(
	ctx context.Context,
	queries *dbgen.Queries,
	workspaceID, roleID ids.UUID,
	permissions []Permission,
) error {
	if err := queries.DeleteWorkspaceRolePermissions(ctx, dbgen.DeleteWorkspaceRolePermissionsParams{
		WorkspaceID: workspaceID.PG(), RoleID: roleID.PG(),
	}); err != nil {
		return fmt.Errorf("delete workspace role permissions: %w", err)
	}
	if len(permissions) == 0 {
		return nil
	}
	if err := queries.InsertWorkspaceRolePermissions(ctx, dbgen.InsertWorkspaceRolePermissionsParams{
		WorkspaceID: workspaceID.PG(), RoleID: roleID.PG(), Permissions: permissionStrings(permissions),
	}); err != nil {
		return fmt.Errorf("insert workspace role permissions: %w", err)
	}
	return nil
}

func permissionStrings(permissions []Permission) []string {
	result := make([]string, len(permissions))
	for index, permission := range permissions {
		result[index] = string(permission)
	}
	return result
}

func workspaceRoleFromCreateRow(row dbgen.CreateWorkspaceRoleRow, permissions []Permission) WorkspaceRoleDefinition {
	roleID, _ := ids.FromPG(row.ID)
	return WorkspaceRoleDefinition{ID: roleID, Key: row.RoleKey, Name: row.Name, BaseRole: row.BaseRole,
		IsSystem: row.IsSystem, Permissions: append([]Permission(nil), permissions...), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

func workspaceRoleFromUpdateRow(row dbgen.UpdateWorkspaceRoleRow, permissions []Permission) WorkspaceRoleDefinition {
	roleID, _ := ids.FromPG(row.ID)
	return WorkspaceRoleDefinition{ID: roleID, Key: row.RoleKey, Name: row.Name, BaseRole: row.BaseRole,
		IsSystem: row.IsSystem, Permissions: append([]Permission(nil), permissions...), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

func mapWorkspaceRoleConstraint(err error, pointer string) error {
	var postgresError interface{ SQLState() string }
	if errors.As(err, &postgresError) {
		switch postgresError.SQLState() {
		case "23503", "23505":
			return errx.ErrConflict
		case "23514":
			return validation(pointer, "validation.constraint")
		}
	}
	return fmt.Errorf("workspace role constraint: %w", err)
}
