//go:build integration

package activities_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/activities"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/assignment"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestTaskAssignmentsAuthorizationIsolationAndConcurrencyOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "task-assignments-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "task-assignments-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, otherWorkspaceID := assignmentTestID(t), assignmentTestID(t)
	ownerID, responsibleID := assignmentTestID(t), assignmentTestID(t)
	departmentUserID, watcherID := assignmentTestID(t), assignmentTestID(t)
	outsiderID, otherUserID := assignmentTestID(t), assignmentTestID(t)
	ownerMembershipID, responsibleMembershipID := assignmentTestID(t), assignmentTestID(t)
	departmentMembershipID, watcherMembershipID := assignmentTestID(t), assignmentTestID(t)
	outsiderMembershipID, otherMembershipID := assignmentTestID(t), assignmentTestID(t)
	departmentID := assignmentTestID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	passwordHash := "$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g"
	_, err = admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Assignment Owner',$13),
 ($3,$4,$4,'Responsible User',$13),
 ($5,$6,$6,'Department User',$13),
 ($7,$8,$8,'Watcher User',$13),
 ($9,$10,$10,'Unassigned User',$13),
 ($11,$12,$12,'Other Workspace User',$13)`,
		ownerID.PG(), "assignment-owner-"+suffix+"@example.invalid",
		responsibleID.PG(), "assignment-responsible-"+suffix+"@example.invalid",
		departmentUserID.PG(), "assignment-department-"+suffix+"@example.invalid",
		watcherID.PG(), "assignment-watcher-"+suffix+"@example.invalid",
		outsiderID.PG(), "assignment-outsider-"+suffix+"@example.invalid",
		otherUserID.PG(), "assignment-other-"+suffix+"@example.invalid", passwordHash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{workspaceID.String(), otherWorkspaceID.String()})
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`, []string{
			ownerID.String(), responsibleID.String(), departmentUserID.String(), watcherID.String(),
			outsiderID.String(), otherUserID.String(),
		})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES
 ($1,'Assignment Workspace',$2), ($3,'Other Assignment Workspace',$4)`,
		workspaceID.PG(), "assignments-"+suffix, otherWorkspaceID.PG(), "other-assignments-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'sales'), ($1,$6,$7,'sales'),
 ($1,$8,$9,'sales'), ($1,$10,$11,'sales'), ($12,$13,$14,'owner')`,
		workspaceID.PG(), ownerMembershipID.PG(), ownerID.PG(),
		responsibleMembershipID.PG(), responsibleID.PG(), departmentMembershipID.PG(), departmentUserID.PG(),
		watcherMembershipID.PG(), watcherID.PG(), outsiderMembershipID.PG(), outsiderID.PG(),
		otherWorkspaceID.PG(), otherMembershipID.PG(), otherUserID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.teams(workspace_id,id,name) VALUES($1,$2,'Delivery')`,
		workspaceID.PG(), departmentID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.team_memberships(workspace_id,team_id,membership_id) VALUES($1,$2,$3)`,
		workspaceID.PG(), departmentID.PG(), departmentMembershipID.PG()); err != nil {
		t.Fatal(err)
	}

	tenantService := tenancy.NewService(appPool)
	service := activities.NewService(nil)
	owner := identity.Principal{UserID: ownerID}
	metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "task-assignments"}
	dueAt := time.Now().UTC().Add(24 * time.Hour)
	var task activities.Activity
	err = tenantService.WithWorkspace(ctx, owner, workspaceID, metadata.RequestID, tenancy.PermissionRecordsCreate,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			task, createErr = service.CreateAdvanced(ctx, workspace, metadata, activities.Input{
				Type: "task", Title: "Prepare synthetic proposal", DueAt: &dueAt,
				VisibilityScope: "user", ScopeUserID: &ownerID,
			})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}

	items := []assignment.Input{
		{Kind: "responsible", SubjectType: "user", SubjectID: responsibleID, IsPrimary: true},
		{Kind: "responsible", SubjectType: "department", SubjectID: departmentID},
		{Kind: "watcher", SubjectType: "user", SubjectID: watcherID},
	}
	var assigned assignment.Set
	err = tenantService.WithWorkspace(ctx, owner, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var replaceErr error
			assigned, replaceErr = service.ReplaceTaskAssignments(ctx, workspace, metadata, task.ID, task.Version, items)
			return replaceErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Version != task.Version+1 || len(assigned.Items) != len(items) {
		t.Fatalf("assigned version=%d items=%d", assigned.Version, len(assigned.Items))
	}

	err = tenantService.WithWorkspace(ctx, owner, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceTaskAssignments(ctx, workspace, metadata, task.ID, assigned.Version,
				[]assignment.Input{{Kind: "responsible", SubjectType: "user", SubjectID: otherUserID, IsPrimary: true}})
			return replaceErr
		})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("cross-workspace assignment error=%v, want validation error", err)
	}
	err = tenantService.WithWorkspace(ctx, owner, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceTaskAssignments(ctx, workspace, metadata, task.ID, assigned.Version-1, items[:1])
			return replaceErr
		})
	if !errors.Is(err, errx.ErrVersionConflict) {
		t.Fatalf("stale assignment error=%v, want version conflict", err)
	}
	err = tenantService.WithWorkspace(ctx, owner, workspaceID, metadata.RequestID, tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			current, listErr := service.ListTaskAssignments(ctx, workspace, workspaceID, task.ID)
			if listErr == nil && (current.Version != assigned.Version || len(current.Items) != len(items)) {
				t.Fatalf("failed replacements changed assignments: version=%d items=%d", current.Version, len(current.Items))
			}
			return listErr
		})
	if err != nil {
		t.Fatal(err)
	}

	assertCanRead := func(userID ids.UUID) {
		t.Helper()
		readErr := tenantService.WithWorkspace(ctx, identity.Principal{UserID: userID}, workspaceID,
			"task-assignment-read", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
				_, getErr := service.Get(ctx, workspace, workspaceID, task.ID)
				return getErr
			})
		if readErr != nil {
			t.Fatalf("assigned user %s cannot read task: %v", userID, readErr)
		}
	}
	assertCanRead(responsibleID)
	assertCanRead(departmentUserID)
	assertCanRead(watcherID)
	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: outsiderID}, workspaceID,
		"task-assignment-deny", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
			if _, getErr := service.Get(ctx, workspace, workspaceID, task.ID); !errors.Is(getErr, errx.ErrNotFound) {
				t.Fatalf("unassigned task read error=%v, want not found", getErr)
			}
			var visibleAssignments int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM activities.activity_assignments WHERE activity_id=$1`,
				task.ID.PG()).Scan(&visibleAssignments); queryErr != nil {
				return queryErr
			}
			if visibleAssignments != 0 {
				t.Fatalf("unassigned user sees %d assignment rows", visibleAssignments)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: watcherID}, workspaceID,
		"task-assignment-watcher-update", tenancy.PermissionRecordsUpdate, func(workspace *tenancy.WorkspaceTx) error {
			_, completeErr := service.Complete(ctx, workspace, events.Metadata{
				WorkspaceID: workspaceID, ActorID: watcherID, RequestID: "watcher-complete",
			}, task.ID, assigned.Version)
			return completeErr
		})
	if !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("watcher complete error=%v, want forbidden", err)
	}
	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: departmentUserID}, workspaceID,
		"task-assignment-responsible-update", tenancy.PermissionRecordsUpdate, func(workspace *tenancy.WorkspaceTx) error {
			_, completeErr := service.Complete(ctx, workspace, events.Metadata{
				WorkspaceID: workspaceID, ActorID: departmentUserID, RequestID: "department-complete",
			}, task.ID, assigned.Version)
			return completeErr
		})
	if err != nil {
		t.Fatal(err)
	}

	var recipients int
	if err := admin.QueryRow(ctx, `SELECT count(DISTINCT recipient_user_id)
FROM notifications.sse_events WHERE workspace_id=$1 AND data->>'activityId'=$2
  AND event_type='activities.private.task.assignments_replaced'`, workspaceID.PG(), task.ID.String()).Scan(&recipients); err != nil {
		t.Fatal(err)
	}
	if recipients != 4 {
		t.Fatalf("assignment SSE recipients=%d, want owner, user, department member, watcher", recipients)
	}
}

func assignmentTestID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
