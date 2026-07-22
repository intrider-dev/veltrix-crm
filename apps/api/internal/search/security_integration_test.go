//go:build integration

package search_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	searchservice "github.com/veltrixcrm/veltrix-crm/apps/api/internal/search"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestGlobalSearchEnforcesEntityPermissionsVisibilityAndSystemRoleStageBypass(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, adminURL, appPassword); err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, adminURL, 1, "search-security-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "search-security-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, ownerID, memberID := searchSecurityID(t), searchSecurityID(t), searchSecurityID(t)
	ownerMembershipID, memberMembershipID := searchSecurityID(t), searchSecurityID(t)
	roleID, pipelineID, stageID := searchSecurityID(t), searchSecurityID(t), searchSecurityID(t)
	leadID, dealID := searchSecurityID(t), searchSecurityID(t)
	publicNoteID, privateNoteID := searchSecurityID(t), searchSecurityID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	passwordHash := "$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g"

	if _, err := admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
  ($1,$2,$2,'Search owner',$5),
  ($3,$4,$4,'Restricted search member',$5)
`, ownerID.PG(), "search-owner-"+suffix+"@example.invalid",
		memberID.PG(), "search-member-"+suffix+"@example.invalid", passwordHash); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{ownerID.String(), memberID.String()})
	}()
	if _, err := admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES($1,'Search security',$2)`,
		workspaceID.PG(), "search-security-"+suffix); err != nil {
		t.Fatal(err)
	}
	var ownerRoleID ids.UUID
	if err := admin.QueryRow(ctx, `
SELECT id FROM tenancy.workspace_roles WHERE workspace_id=$1 AND role_key='owner'
`, workspaceID.PG()).Scan(&ownerRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.workspace_roles(workspace_id,id,role_key,name,base_role,is_system)
VALUES($1,$2,'restricted_admin','Restricted admin','admin',false)
`, workspaceID.PG(), roleID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.role_permissions(workspace_id,role_id,permission) VALUES
  ($1,$2,'records.read'), ($1,$2,'leads.read')
`, workspaceID.PG(), roleID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role,role_id,status) VALUES
  ($1,$3,$4,'owner',$5,'active'),
  ($1,$6,$7,'admin',$2,'active')
`, workspaceID.PG(), roleID.PG(), ownerMembershipID.PG(), ownerID.PG(), ownerRoleID.PG(),
		memberMembershipID.PG(), memberID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO sales.pipelines(workspace_id,id,name,is_default) VALUES($1,$2,'Permission pipeline',true)
`, workspaceID.PG(), pipelineID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO sales.pipeline_stages(workspace_id,id,pipeline_id,name,probability,position)
VALUES($1,$3,$2,'Permission stage',25,0)
`, workspaceID.PG(), pipelineID.PG(), stageID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO sales.pipeline_stage_role_access(workspace_id,stage_id,role_id,can_view,can_enter,can_leave)
VALUES($1,$2,$3,true,true,true)
`, workspaceID.PG(), stageID.PG(), ownerRoleID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO sales.leads(workspace_id,id,name,status) VALUES($1,$2,'Permission lead','new')
`, workspaceID.PG(), leadID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO sales.deals(workspace_id,id,pipeline_id,stage_id,name,amount_minor,currency,position)
VALUES($1,$2,$3,$4,'Permission deal',10000,'USD',0)
`, workspaceID.PG(), dealID.PG(), pipelineID.PG(), stageID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO activities.activities(
  workspace_id,id,activity_type,title,body,created_by,visibility_scope,scope_user_id
) VALUES
  ($1,$2,'note','Permission public note','Visible body',$4,'workspace',NULL),
  ($1,$3,'note','Permission private note','Private body',$4,'user',$4)
`, workspaceID.PG(), publicNoteID.PG(), privateNoteID.PG(), ownerID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO search.documents(workspace_id,entity_type,entity_id,title,subtitle,searchable_text) VALUES
  ($1,'lead',$2,'Permission lead',NULL,'Permission lead'),
  ($1,'deal',$3,'Permission deal',NULL,'Permission deal'),
  ($1,'note',$4,'Permission public note',NULL,'Permission public note Visible body'),
  ($1,'note',$5,'Permission private note',NULL,'Permission private note Private body')
`, workspaceID.PG(), leadID.PG(), dealID.PG(), publicNoteID.PG(), privateNoteID.PG()); err != nil {
		t.Fatal(err)
	}

	service := tenancy.NewService(appPool)
	search := searchservice.NewService()
	err = service.WithWorkspace(ctx, identity.Principal{UserID: memberID}, workspaceID,
		"search-permission-visibility", tenancy.PermissionRecordsRead, func(workspace *tenancy.WorkspaceTx) error {
			var stageAllowed bool
			if queryErr := workspace.Tx.QueryRow(ctx, `
SELECT sales.pipeline_stage_access_allowed($1,$2,'view')
`, workspaceID.PG(), stageID.PG()).Scan(&stageAllowed); queryErr != nil {
				return queryErr
			}
			if stageAllowed {
				t.Fatal("custom admin-envelope role bypassed system-only stage access")
			}
			rows, searchErr := search.Global(ctx, workspace, workspaceID, "Permission")
			if searchErr != nil {
				return searchErr
			}
			seen := make(map[string]bool, len(rows))
			for _, row := range rows {
				seen[row.EntityType+":"+row.Title] = true
			}
			if !seen["lead:Permission lead"] {
				t.Fatal("lead allowed by leads.read was missing")
			}
			if !seen["note:Permission public note"] {
				t.Fatal("workspace-visible note was missing")
			}
			if seen["deal:Permission deal"] {
				t.Fatal("deal was exposed without deals.read")
			}
			if seen["note:Permission private note"] {
				t.Fatal("user-scoped note was exposed to another member")
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func searchSecurityID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
