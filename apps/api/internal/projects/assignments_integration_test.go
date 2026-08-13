//go:build integration

package projects_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/projects"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestProjectAssignmentReplacementUsesVersionAndTenantReferencesOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "project-assignments-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "project-assignments-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, otherWorkspaceID := projectAssignmentID(t), projectAssignmentID(t)
	ownerID, memberID, otherUserID := projectAssignmentID(t), projectAssignmentID(t), projectAssignmentID(t)
	ownerMembershipID, memberMembershipID, otherMembershipID := projectAssignmentID(t), projectAssignmentID(t), projectAssignmentID(t)
	departmentID := projectAssignmentID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	passwordHash := "$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g"
	if _, err = admin.Exec(ctx, `INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Project Owner',$7), ($3,$4,$4,'Project Member',$7),
 ($5,$6,$6,'Other Project Member',$7)`,
		ownerID.PG(), "project-owner-"+suffix+"@example.invalid",
		memberID.PG(), "project-member-"+suffix+"@example.invalid",
		otherUserID.PG(), "project-other-"+suffix+"@example.invalid", passwordHash); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{workspaceID.String(), otherWorkspaceID.String()})
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{ownerID.String(), memberID.String(), otherUserID.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES
 ($1,'Project Assignment Workspace',$2), ($3,'Other Project Assignment Workspace',$4)`,
		workspaceID.PG(), "project-assignments-"+suffix,
		otherWorkspaceID.PG(), "other-project-assignments-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'sales'), ($6,$7,$8,'owner')`,
		workspaceID.PG(), ownerMembershipID.PG(), ownerID.PG(), memberMembershipID.PG(), memberID.PG(),
		otherWorkspaceID.PG(), otherMembershipID.PG(), otherUserID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.teams(workspace_id,id,name) VALUES($1,$2,'Product')`,
		workspaceID.PG(), departmentID.PG()); err != nil {
		t.Fatal(err)
	}

	service := projects.NewService()
	tenantService := tenancy.NewService(appPool)
	principal := identity.Principal{UserID: ownerID}
	metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "project-assignments"}
	var project projects.Record
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsCreate,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			project, createErr = service.Create(ctx, workspace, metadata, projects.Input{
				Name: "Synthetic rollout", Status: "active", Visibility: "restricted",
			})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}
	items := []projects.AssignmentInput{
		{Kind: "responsible", SubjectType: "user", SubjectID: memberID},
		{Kind: "watcher", SubjectType: "department", SubjectID: departmentID},
	}
	var first projects.AssignmentSet
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var replaceErr error
			first, replaceErr = service.ReplaceAssignments(ctx, workspace, metadata,
				ids.MustParse(project.ID), project.Version, items)
			return replaceErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != project.Version+1 || len(first.Items) != len(items) {
		t.Fatalf("first project assignment version=%d items=%d", first.Version, len(first.Items))
	}

	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceAssignments(ctx, workspace, metadata,
				ids.MustParse(project.ID), project.Version, items[:1])
			return replaceErr
		})
	if !errors.Is(err, errx.ErrVersionConflict) {
		t.Fatalf("stale project assignment error=%v, want version conflict", err)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceAssignments(ctx, workspace, metadata,
				ids.MustParse(project.ID), first.Version,
				[]projects.AssignmentInput{{Kind: "responsible", SubjectType: "user", SubjectID: otherUserID}})
			return replaceErr
		})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("cross-tenant project assignment error=%v, want validation error", err)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			current, listErr := service.ListAssignments(ctx, workspace, workspaceID, ids.MustParse(project.ID))
			if listErr == nil && (current.Version != first.Version || len(current.Items) != len(items)) {
				t.Fatalf("failed writes changed project assignments: version=%d items=%d", current.Version, len(current.Items))
			}
			return listErr
		})
	if err != nil {
		t.Fatal(err)
	}
}

func projectAssignmentID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
