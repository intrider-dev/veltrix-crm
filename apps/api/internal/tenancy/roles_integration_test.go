//go:build integration

package tenancy

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestCustomRoleEffectivePermissionsOnPostgreSQL(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, adminURL, appPassword); err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, adminURL, 1, "roles-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	pool, err := database.Open(ctx, appURL, 2, "roles-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	identityService, err := identity.NewServiceWithOptions(pool, identity.NewPasswordHasher(1), identity.ServiceOptions{
		RegistrationEnabled: true, SupportedLocales: []string{"en", "ru"},
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(pool)
	owner := registerPrincipal(t, ctx, identityService, "roles-owner")
	member := registerPrincipal(t, ctx, identityService, "roles-member")
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{owner.UserID.String(), member.UserID.String()})
	}()

	first := createRoleTestWorkspace(t, ctx, service, owner, "roles-first")
	second := createRoleTestWorkspace(t, ctx, service, owner, "roles-second")
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{first.String(), second.String()})
	}()

	invitation, err := service.InviteMember(ctx, owner, first, "roles-invite", member.Email, "sales", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	membership, err := service.AcceptInvitation(ctx, member, invitation.Token, "roles-accept")
	if err != nil {
		t.Fatal(err)
	}
	membershipID, _ := ids.FromPG(membership.ID)

	roles, err := service.ListRoles(ctx, owner, first, "roles-list")
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 5 {
		t.Fatalf("system roles=%d, want 5", len(roles))
	}
	allPermissions := []Permission{
		PermissionRecordsRead, PermissionRecordsCreate, PermissionRecordsUpdate, PermissionRecordsDelete,
		PermissionDataExport, PermissionReportsRead, PermissionAuditRead, PermissionSettingsWrite,
		PermissionMembersRead, PermissionMembersWrite, PermissionRolesWrite,
		PermissionLeadsRead, PermissionLeadsCreate, PermissionLeadsUpdate, PermissionLeadsDelete,
		PermissionDealsRead, PermissionDealsCreate, PermissionDealsUpdate, PermissionDealsDelete,
		PermissionLeadStagesManage, PermissionDealStagesManage,
	}
	for _, role := range roles {
		if !role.IsSystem {
			t.Fatalf("bootstrap role %s is not system", role.Key)
		}
		grants := make(map[Permission]struct{}, len(role.Permissions))
		for _, permission := range role.Permissions {
			grants[permission] = struct{}{}
		}
		for _, permission := range allPermissions {
			_, actual := grants[permission]
			if expected := RoleAllows(role.BaseRole, permission); actual != expected {
				t.Fatalf("role=%s permission=%s actual=%t expected=%t", role.Key, permission, actual, expected)
			}
		}
	}

	custom, err := service.CreateRole(ctx, owner, roleTestMetadata(first, owner.UserID, "roles-create"), WorkspaceRoleInput{
		Name: "Lead-only sales", BaseRole: "sales", Permissions: []Permission{PermissionLeadsRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := service.AssignRole(ctx, owner, roleTestMetadata(first, owner.UserID, "roles-assign"), membershipID, custom.ID)
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Role != "sales" {
		t.Fatalf("compatibility base role=%s, want sales", assigned.Role)
	}
	if err := service.WithWorkspace(ctx, member, first, "roles-leads-read", PermissionLeadsRead, func(workspace *WorkspaceTx) error {
		var actorSetting, workspaceSetting string
		if err := workspace.Tx.QueryRow(ctx, `SELECT current_setting('app.actor_id'), current_setting('app.workspace_id')`).Scan(
			&actorSetting, &workspaceSetting,
		); err != nil {
			return err
		}
		if actorSetting != member.UserID.String() || workspaceSetting != first.String() {
			t.Fatalf("transaction context actor=%q workspace=%q", actorSetting, workspaceSetting)
		}
		return nil
	}); err != nil {
		t.Fatalf("custom role lead read denied: %v", err)
	}
	var leakedActor, leakedWorkspace *string
	if err := pool.QueryRow(ctx, `
SELECT NULLIF(current_setting('app.actor_id', true), ''),
       NULLIF(current_setting('app.workspace_id', true), '')
`).Scan(&leakedActor, &leakedWorkspace); err != nil {
		t.Fatal(err)
	}
	if leakedActor != nil || leakedWorkspace != nil {
		t.Fatalf("transaction-local context leaked actor=%v workspace=%v", leakedActor, leakedWorkspace)
	}
	if err := service.WithWorkspace(ctx, member, first, "roles-deals-read", PermissionDealsRead, func(*WorkspaceTx) error { return nil }); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("lead-only role deal read error=%v, want forbidden", err)
	}
	if _, err := service.CreateRole(ctx, member, roleTestMetadata(first, member.UserID, "roles-escalate"), WorkspaceRoleInput{
		Name: "Escalation", BaseRole: "sales", Permissions: []Permission{PermissionRecordsRead},
	}); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("non-owner role create error=%v, want forbidden", err)
	}
	if _, err := service.CreateRole(ctx, owner, roleTestMetadata(first, owner.UserID, "roles-invalid"), WorkspaceRoleInput{
		Name: "Invalid", BaseRole: "sales", Permissions: []Permission{PermissionSettingsWrite},
	}); err == nil {
		t.Fatal("sales-based role accepted settings.write")
	}

	secondRoles, err := service.ListRoles(ctx, owner, second, "roles-second-list")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AssignRole(ctx, owner, roleTestMetadata(first, owner.UserID, "roles-cross-tenant"), membershipID, secondRoles[0].ID); !errors.Is(err, errx.ErrNotFound) {
		t.Fatalf("cross-workspace role assignment error=%v, want not found", err)
	}
	if err := service.DeleteRole(ctx, owner, roleTestMetadata(first, owner.UserID, "roles-delete-assigned"), custom.ID, custom.Version); !errors.Is(err, errx.ErrConflict) {
		t.Fatalf("delete assigned role error=%v, want conflict", err)
	}
	members, err := service.ListMembers(ctx, owner, first, "roles-owner-membership")
	if err != nil {
		t.Fatal(err)
	}
	var ownerMembershipID ids.UUID
	for _, candidate := range members {
		candidateUserID, _ := ids.FromPG(candidate.UserID)
		if candidateUserID == owner.UserID {
			ownerMembershipID, _ = ids.FromPG(candidate.ID)
		}
	}
	if ownerMembershipID == (ids.UUID{}) {
		t.Fatal("owner membership not found")
	}
	if _, err := service.AssignRole(ctx, owner, roleTestMetadata(first, owner.UserID, "roles-owner-protect"), ownerMembershipID, custom.ID); !errors.Is(err, ErrCannotManageOwner) {
		t.Fatalf("owner reassignment error=%v", err)
	}
}

func createRoleTestWorkspace(t *testing.T, ctx context.Context, service *Service, owner identity.Principal, label string) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateWorkspace(ctx, owner, label, CreateWorkspaceRequest{
		// A UUIDv7 prefix is time-derived and can repeat across test reruns in the
		// same database. Keep the full identifier so leaked fixtures cannot turn a
		// later authorization run into an unrelated slug-validation failure.
		Name: label, Slug: label + "-" + id.String(), DefaultLocale: "en", Timezone: "UTC", DefaultCurrency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, ok := ids.FromPG(created.Workspace.ID)
	if !ok {
		t.Fatal("invalid workspace ID")
	}
	return workspaceID
}

func roleTestMetadata(workspaceID, actorID ids.UUID, requestID string) events.Metadata {
	return events.Metadata{WorkspaceID: workspaceID, ActorID: actorID, RequestID: requestID}
}
