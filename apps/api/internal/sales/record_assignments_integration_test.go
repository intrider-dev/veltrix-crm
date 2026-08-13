//go:build integration

package sales_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/assignment"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestLeadAndDealAssignmentsAreAtomicAndTenantScopedOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "sales-assignments-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "sales-assignments-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, otherWorkspaceID := salesAssignmentID(t), salesAssignmentID(t)
	ownerID, assigneeID, otherUserID := salesAssignmentID(t), salesAssignmentID(t), salesAssignmentID(t)
	ownerMembershipID, assigneeMembershipID, otherMembershipID := salesAssignmentID(t), salesAssignmentID(t), salesAssignmentID(t)
	departmentID, leadID, dealID := salesAssignmentID(t), salesAssignmentID(t), salesAssignmentID(t)
	pipelineID, pipelineStageID := salesAssignmentID(t), salesAssignmentID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	passwordHash := "$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g"
	if _, err = admin.Exec(ctx, `INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Sales Assignment Owner',$7), ($3,$4,$4,'Sales Assignee',$7),
 ($5,$6,$6,'Other Tenant Assignee',$7)`,
		ownerID.PG(), "sales-assignment-owner-"+suffix+"@example.invalid",
		assigneeID.PG(), "sales-assignee-"+suffix+"@example.invalid",
		otherUserID.PG(), "sales-other-"+suffix+"@example.invalid", passwordHash); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{workspaceID.String(), otherWorkspaceID.String()})
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{ownerID.String(), assigneeID.String(), otherUserID.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES
 ($1,'Sales Assignment Workspace',$2), ($3,'Other Sales Assignment Workspace',$4)`,
		workspaceID.PG(), "sales-assignments-"+suffix,
		otherWorkspaceID.PG(), "other-sales-assignments-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'sales'), ($6,$7,$8,'owner')`,
		workspaceID.PG(), ownerMembershipID.PG(), ownerID.PG(), assigneeMembershipID.PG(), assigneeID.PG(),
		otherWorkspaceID.PG(), otherMembershipID.PG(), otherUserID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.teams(workspace_id,id,name) VALUES($1,$2,'Enterprise Sales')`,
		workspaceID.PG(), departmentID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO sales.pipelines(workspace_id,id,name,is_default) VALUES($1,$2,'Primary',true)`,
		workspaceID.PG(), pipelineID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO sales.pipeline_stages(workspace_id,id,pipeline_id,name,probability,position)
VALUES($1,$2,$3,'Discovery',20,0)`, workspaceID.PG(), pipelineStageID.PG(), pipelineID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO sales.leads(workspace_id,id,name,status) VALUES($1,$2,'Synthetic lead','new')`,
		workspaceID.PG(), leadID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO sales.deals(workspace_id,id,pipeline_id,stage_id,name,currency)
VALUES($1,$2,$3,$4,'Synthetic deal','USD')`, workspaceID.PG(), dealID.PG(), pipelineID.PG(), pipelineStageID.PG()); err != nil {
		t.Fatal(err)
	}

	service := sales.NewService()
	tenantService := tenancy.NewService(appPool)
	principal := identity.Principal{UserID: ownerID}
	metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "sales-assignments"}
	leadItems := []assignment.Input{
		{Kind: "responsible", SubjectType: "user", SubjectID: assigneeID, IsPrimary: true},
		{Kind: "watcher", SubjectType: "department", SubjectID: departmentID},
	}
	var leadSet assignment.Set
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var replaceErr error
			leadSet, replaceErr = service.ReplaceLeadAssignments(ctx, workspace, metadata, leadID, 1, leadItems)
			return replaceErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if leadSet.Version != 2 || len(leadSet.Items) != 2 {
		t.Fatalf("lead assignment set version=%d items=%d", leadSet.Version, len(leadSet.Items))
	}
	var leadOwner, leadTeam *string
	if err := admin.QueryRow(ctx, `SELECT owner_user_id::text, team_id::text FROM sales.leads WHERE workspace_id=$1 AND id=$2`,
		workspaceID.PG(), leadID.PG()).Scan(&leadOwner, &leadTeam); err != nil {
		t.Fatal(err)
	}
	if leadOwner == nil || *leadOwner != assigneeID.String() || leadTeam != nil {
		t.Fatalf("lead legacy projection owner=%v team=%v", leadOwner, leadTeam)
	}

	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceLeadAssignments(ctx, workspace, metadata, leadID, leadSet.Version,
				[]assignment.Input{{Kind: "responsible", SubjectType: "user", SubjectID: otherUserID, IsPrimary: true}})
			return replaceErr
		})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("cross-tenant lead assignment error=%v, want validation error", err)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceLeadAssignments(ctx, workspace, metadata, leadID, 1, leadItems[:1])
			return replaceErr
		})
	if !errors.Is(err, errx.ErrVersionConflict) {
		t.Fatalf("stale lead assignment error=%v, want version conflict", err)
	}

	dealItems := []assignment.Input{
		{Kind: "responsible", SubjectType: "user", SubjectID: assigneeID, IsPrimary: true},
		{Kind: "watcher", SubjectType: "department", SubjectID: departmentID},
	}
	var dealSet assignment.Set
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var replaceErr error
			dealSet, replaceErr = service.ReplaceDealAssignments(ctx, workspace, metadata, dealID, 1, dealItems)
			return replaceErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if dealSet.Version != 2 || len(dealSet.Items) != 2 {
		t.Fatalf("deal assignment set version=%d items=%d", dealSet.Version, len(dealSet.Items))
	}
	var dealOwner string
	if err := admin.QueryRow(ctx, `SELECT owner_user_id::text FROM sales.deals WHERE workspace_id=$1 AND id=$2`,
		workspaceID.PG(), dealID.PG()).Scan(&dealOwner); err != nil {
		t.Fatal(err)
	}
	if dealOwner != assigneeID.String() {
		t.Fatalf("deal legacy owner=%s, want %s", dealOwner, assigneeID)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := service.ReplaceDealAssignments(ctx, workspace, metadata, dealID, dealSet.Version,
				[]assignment.Input{{Kind: "responsible", SubjectType: "user", SubjectID: otherUserID, IsPrimary: true}})
			return replaceErr
		})
	if !errors.As(err, &validationError) {
		t.Fatalf("cross-tenant deal assignment error=%v, want validation error", err)
	}

	var leadCount, dealCount int
	if err := admin.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM sales.lead_assignments WHERE workspace_id=$1 AND lead_id=$2),
 (SELECT count(*) FROM sales.deal_assignments WHERE workspace_id=$1 AND deal_id=$3)`,
		workspaceID.PG(), leadID.PG(), dealID.PG()).Scan(&leadCount, &dealCount); err != nil {
		t.Fatal(err)
	}
	if leadCount != len(leadItems) || dealCount != len(dealItems) {
		t.Fatalf("rolled-back assignment counts lead=%d deal=%d", leadCount, dealCount)
	}
}

func salesAssignmentID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
