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
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestCalendarAudienceIsolationAndTargetedSSEOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "calendar-visibility-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 1, "calendar-visibility-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID := mustCalendarID(t)
	ownerID, targetID := mustCalendarID(t), mustCalendarID(t)
	departmentUserID, outsiderID := mustCalendarID(t), mustCalendarID(t)
	ownerMembershipID, targetMembershipID := mustCalendarID(t), mustCalendarID(t)
	departmentMembershipID, outsiderMembershipID := mustCalendarID(t), mustCalendarID(t)
	departmentID := mustCalendarID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	_, err = admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Calendar Owner','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($3,$4,$4,'Calendar Target','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($5,$6,$6,'Calendar Department','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
 ($7,$8,$8,'Calendar Outsider','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		ownerID.PG(), "calendar-owner-"+suffix+"@example.invalid",
		targetID.PG(), "calendar-target-"+suffix+"@example.invalid",
		departmentUserID.PG(), "calendar-department-"+suffix+"@example.invalid",
		outsiderID.PG(), "calendar-outsider-"+suffix+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{ownerID.String(), targetID.String(), departmentUserID.String(), outsiderID.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES($1,'Calendar Workspace',$2)`,
		workspaceID.PG(), "calendar-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'sales'), ($1,$6,$7,'sales'), ($1,$8,$9,'viewer')`,
		workspaceID.PG(), ownerMembershipID.PG(), ownerID.PG(), targetMembershipID.PG(), targetID.PG(),
		departmentMembershipID.PG(), departmentUserID.PG(), outsiderMembershipID.PG(), outsiderID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.teams(workspace_id,id,name) VALUES($1,$2,'Field Sales')`,
		workspaceID.PG(), departmentID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.team_memberships(workspace_id,team_id,membership_id) VALUES($1,$2,$3)`,
		workspaceID.PG(), departmentID.PG(), departmentMembershipID.PG()); err != nil {
		t.Fatal(err)
	}

	service := activities.NewService(nil)
	tenantService := tenancy.NewService(appPool)
	owner := identity.Principal{UserID: ownerID}
	now := time.Now().UTC().Truncate(time.Second)
	created := make(map[string]activities.Activity)
	err = tenantService.WithWorkspace(ctx, owner, workspaceID, "calendar-create", tenancy.PermissionRecordsCreate,
		func(workspace *tenancy.WorkspaceTx) error {
			inputs := []activities.Input{
				{Type: "meeting", Title: "Workspace planning", OccurredAt: now, VisibilityScope: "workspace"},
				{Type: "meeting", Title: "Private one-to-one", OccurredAt: now, VisibilityScope: "user", ScopeUserID: &targetID},
				{Type: "meeting", Title: "Department planning", OccurredAt: now, VisibilityScope: "department", ScopeDepartmentID: &departmentID},
			}
			for _, input := range inputs {
				value, createErr := service.CreateAdvanced(ctx, workspace, events.Metadata{
					WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "calendar-create",
				}, input)
				if createErr != nil {
					return createErr
				}
				created[input.VisibilityScope] = value
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	assertVisible := func(userID ids.UUID, expected ...string) {
		t.Helper()
		err := tenantService.WithWorkspace(ctx, identity.Principal{UserID: userID}, workspaceID,
			"calendar-read", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
				items, listErr := service.Calendar(ctx, workspace, workspaceID, now.Add(-time.Hour), now.Add(time.Hour))
				if listErr != nil {
					return listErr
				}
				got := make(map[string]bool, len(items))
				for _, item := range items {
					got[item.VisibilityScope] = true
				}
				if len(got) != len(expected) {
					t.Fatalf("user %s sees %v, expected %v", userID, got, expected)
				}
				for _, scope := range expected {
					if !got[scope] {
						t.Fatalf("user %s does not see %s event", userID, scope)
					}
				}
				return nil
			})
		if err != nil {
			t.Fatal(err)
		}
	}
	assertVisible(ownerID, "workspace", "user", "department")
	assertVisible(targetID, "workspace", "user")
	assertVisible(departmentUserID, "workspace", "department")
	assertVisible(outsiderID, "workspace")

	customAdminRoleID := mustCalendarID(t)
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.workspace_roles(workspace_id,id,role_key,name,base_role,is_system)
VALUES($1,$2,'calendar_restricted_admin','Calendar restricted admin','admin',false)
`, workspaceID.PG(), customAdminRoleID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.role_permissions(workspace_id,role_id,permission)
VALUES($1,$2,'records.read')
`, workspaceID.PG(), customAdminRoleID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
UPDATE tenancy.memberships SET role='admin', role_id=$3
WHERE workspace_id=$1 AND id=$2
`, workspaceID.PG(), outsiderMembershipID.PG(), customAdminRoleID.PG()); err != nil {
		t.Fatal(err)
	}
	assertVisible(outsiderID, "workspace")

	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: targetID}, workspaceID,
		"calendar-assignee-update-guard", tenancy.PermissionRecordsUpdate, func(workspace *tenancy.WorkspaceTx) error {
			_, updateErr := workspace.Tx.Exec(ctx, `
UPDATE activities.activities SET title='Unauthorized rewrite' WHERE workspace_id=$1 AND id=$2
`, workspaceID.PG(), created["user"].ID.PG())
			return updateErr
		})
	if err == nil {
		t.Fatal("non-creator changed protected activity fields through the runtime role")
	}
	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: targetID}, workspaceID,
		"calendar-assignee-complete", tenancy.PermissionRecordsUpdate, func(workspace *tenancy.WorkspaceTx) error {
			command, updateErr := workspace.Tx.Exec(ctx, `
UPDATE activities.activities
SET status='completed', completed_at=now(), version=version+1, updated_at=now()
WHERE workspace_id=$1 AND id=$2
`, workspaceID.PG(), created["user"].ID.PG())
			if updateErr != nil {
				return updateErr
			}
			if command.RowsAffected() != 1 {
				t.Fatal("non-creator could not complete an assigned/user-scoped activity")
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	err = tenantService.WithWorkspace(ctx, identity.Principal{UserID: outsiderID}, workspaceID,
		"calendar-exact-deny", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
			_, getErr := service.Get(ctx, workspace, workspaceID, created["user"].ID)
			if !errors.Is(getErr, errx.ErrNotFound) {
				t.Fatalf("private exact read error=%v, want not found", getErr)
			}
			var visiblePrivateSSE int
			if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM notifications.sse_events WHERE event_type LIKE 'activities.private.%'`).Scan(&visiblePrivateSSE); queryErr != nil {
				return queryErr
			}
			if visiblePrivateSSE != 0 {
				t.Fatalf("outsider sees %d restricted SSE events", visiblePrivateSSE)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	var privateEvents, untargeted int
	if err := admin.QueryRow(ctx, `
SELECT count(*), count(*) FILTER (WHERE recipient_user_id IS NULL)
FROM notifications.sse_events
WHERE workspace_id=$1 AND event_type LIKE 'activities.private.%'`, workspaceID.PG()).Scan(&privateEvents, &untargeted); err != nil {
		t.Fatal(err)
	}
	if privateEvents != 4 || untargeted != 0 {
		t.Fatalf("restricted SSE events=%d untargeted=%d, want 4 and 0", privateEvents, untargeted)
	}
	var restrictedOutbox int
	if err := admin.QueryRow(ctx, `
SELECT count(*) FROM platform.outbox_events
WHERE workspace_id=$1 AND aggregate_type='activity' AND aggregate_id IN ($2,$3)`,
		workspaceID.PG(), created["user"].ID.PG(), created["department"].ID.PG()).Scan(&restrictedOutbox); err != nil {
		t.Fatal(err)
	}
	if restrictedOutbox != 0 {
		t.Fatalf("restricted activities leaked to %d outbox events", restrictedOutbox)
	}
}

func mustCalendarID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
