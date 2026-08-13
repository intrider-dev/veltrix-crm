//go:build integration

package tenancy

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestWorkspaceInvitationOwnerAndTeamLifecycle(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "tenancy-advanced-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	pool, err := database.Open(ctx, appURL, 2, "tenancy-advanced-test-app")
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
	tenantService := NewServiceWithOptions(pool, ServiceOptions{
		SupportedLocales: []string{"en", "ru"}, DefaultLocale: "en",
	})
	owner := registerPrincipal(t, ctx, identityService, "owner")
	member := registerPrincipal(t, ctx, identityService, "member")
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{owner.UserID.String(), member.UserID.String()})
	}()

	suffix := strings.ReplaceAll(owner.UserID.String(), "-", "")[:16]
	created, err := tenantService.CreateWorkspace(ctx, owner, "tenancy-integration", CreateWorkspaceRequest{
		Name: "Integration Workspace", Slug: "integration-" + suffix,
		DefaultLocale: "en", Timezone: "UTC", DefaultCurrency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, ok := ids.FromPG(created.Workspace.ID)
	if !ok {
		t.Fatal("invalid workspace ID")
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
	}()

	invitation, err := tenantService.InviteMember(ctx, owner, workspaceID, "tenancy-integration", member.Email, "sales", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := tenantService.AcceptInvitation(ctx, member, invitation.Token, "tenancy-integration")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Role != "sales" || accepted.Status != "active" {
		t.Fatalf("unexpected invited membership: role=%s status=%s", accepted.Role, accepted.Status)
	}
	ownerMembershipID, ok := ids.FromPG(created.Membership.ID)
	if !ok {
		t.Fatal("invalid owner membership ID")
	}
	memberMembershipID, ok := ids.FromPG(accepted.ID)
	if !ok {
		t.Fatal("invalid member membership ID")
	}
	if _, err := tenantService.UpdateMemberRole(ctx, owner, workspaceID, ownerMembershipID, "tenancy-integration", "admin"); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("last owner demotion error = %v", err)
	}
	if _, err := tenantService.UpdateMemberRole(ctx, owner, workspaceID, memberMembershipID, "tenancy-integration", "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := tenantService.SetMemberStatus(ctx, owner, workspaceID, memberMembershipID, "tenancy-integration", "disabled"); err != nil {
		t.Fatal(err)
	}
	if _, err := tenantService.SetMemberStatus(ctx, owner, workspaceID, ownerMembershipID, "tenancy-integration", "disabled"); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("last owner disable error = %v", err)
	}
	team, err := tenantService.CreateTeam(ctx, owner, workspaceID, "tenancy-integration", "North")
	if err != nil {
		t.Fatal(err)
	}
	teamID, ok := ids.FromPG(team.ID)
	if !ok {
		t.Fatal("invalid team ID")
	}
	if err := tenantService.SetTeamMember(ctx, owner, workspaceID, teamID, ownerMembershipID, "tenancy-integration", true); err != nil {
		t.Fatal(err)
	}
	members, err := tenantService.ListMembers(ctx, owner, workspaceID, "tenancy-integration")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("got %d workspace members", len(members))
	}
}

func registerPrincipal(t *testing.T, ctx context.Context, service *identity.Service, label string) identity.Principal {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	email := label + "-" + strings.ReplaceAll(id.String(), "-", "") + "@example.invalid"
	user, err := service.RegisterDevelopmentUser(ctx, identity.DevelopmentRegistration{
		Email: email, DisplayName: label, Password: "Demo123!", Locale: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	userID, ok := ids.FromPG(user.ID)
	if !ok {
		t.Fatal("invalid user ID")
	}
	return identity.Principal{UserID: userID, Email: user.Email, DisplayName: user.DisplayName, PreferredLocale: user.PreferredLocale}
}
