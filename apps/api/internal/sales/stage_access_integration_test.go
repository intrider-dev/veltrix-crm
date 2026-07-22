//go:build integration

package sales_test

import (
	"context"
	"encoding/json"
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
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/reporting"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestStageRoleAccessIsFailClosedAndTenantScopedOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "stage-access-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 4, "stage-access-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID, otherWorkspaceID := stageAccessID(t), stageAccessID(t)
	ownerID, adminID, salesID, viewerID, otherOwnerID := stageAccessID(t), stageAccessID(t), stageAccessID(t), stageAccessID(t), stageAccessID(t)
	ownerMembershipID, adminMembershipID := stageAccessID(t), stageAccessID(t)
	salesMembershipID, viewerMembershipID, otherMembershipID := stageAccessID(t), stageAccessID(t), stageAccessID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	passwordHash := "$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g"
	if _, err = admin.Exec(ctx, `INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
 ($1,$2,$2,'Stage Owner',$11), ($3,$4,$4,'Stage Admin',$11),
 ($5,$6,$6,'Stage Sales',$11), ($7,$8,$8,'Stage Viewer',$11),
 ($9,$10,$10,'Other Stage Owner',$11)`,
		ownerID.PG(), "stage-owner-"+suffix+"@example.invalid",
		adminID.PG(), "stage-admin-"+suffix+"@example.invalid",
		salesID.PG(), "stage-sales-"+suffix+"@example.invalid",
		viewerID.PG(), "stage-viewer-"+suffix+"@example.invalid",
		otherOwnerID.PG(), "stage-other-"+suffix+"@example.invalid", passwordHash); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{workspaceID.String(), otherWorkspaceID.String()})
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id = ANY($1::uuid[])`,
			[]string{ownerID.String(), adminID.String(), salesID.String(), viewerID.String(), otherOwnerID.String()})
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES
 ($1,'Stage Access',$2), ($3,'Other Stage Access',$4)`, workspaceID.PG(), "stage-access-"+suffix,
		otherWorkspaceID.PG(), "other-stage-access-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
 ($1,$2,$3,'owner'), ($1,$4,$5,'admin'), ($1,$6,$7,'sales'), ($1,$8,$9,'viewer'),
 ($10,$11,$12,'owner')`, workspaceID.PG(), ownerMembershipID.PG(), ownerID.PG(),
		adminMembershipID.PG(), adminID.PG(), salesMembershipID.PG(), salesID.PG(),
		viewerMembershipID.PG(), viewerID.PG(), otherWorkspaceID.PG(), otherMembershipID.PG(), otherOwnerID.PG()); err != nil {
		t.Fatal(err)
	}

	var salesRoleID, otherOwnerRoleID ids.UUID
	if err := admin.QueryRow(ctx, `SELECT id FROM tenancy.workspace_roles WHERE workspace_id=$1 AND role_key='sales'`,
		workspaceID.PG()).Scan(&salesRoleID); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `SELECT id FROM tenancy.workspace_roles WHERE workspace_id=$1 AND role_key='owner'`,
		otherWorkspaceID.PG()).Scan(&otherOwnerRoleID); err != nil {
		t.Fatal(err)
	}
	var leadNewID, leadQualifiedID, otherLeadID ids.UUID
	if err := admin.QueryRow(ctx, `SELECT id FROM sales.lead_stages WHERE workspace_id=$1 AND system_key='new'`, workspaceID.PG()).Scan(&leadNewID); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `SELECT id FROM sales.lead_stages WHERE workspace_id=$1 AND system_key='qualified'`, workspaceID.PG()).Scan(&leadQualifiedID); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `SELECT id FROM sales.lead_stages WHERE workspace_id=$1 AND system_key='new'`, otherWorkspaceID.PG()).Scan(&otherLeadID); err != nil {
		t.Fatal(err)
	}

	pipelineID, pipelineFirstID, pipelineSecondID := stageAccessID(t), stageAccessID(t), stageAccessID(t)
	otherPipelineID, otherPipelineStageID := stageAccessID(t), stageAccessID(t)
	if _, err = admin.Exec(ctx, `INSERT INTO sales.pipelines(workspace_id,id,name,is_default) VALUES
 ($1,$2,'Stage Access Pipeline',true), ($3,$4,'Other Stage Access Pipeline',true)`,
		workspaceID.PG(), pipelineID.PG(), otherWorkspaceID.PG(), otherPipelineID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO sales.pipeline_stages(workspace_id,id,pipeline_id,name,probability,position) VALUES
 ($1,$2,$3,'Discovery',10,0), ($1,$4,$3,'Proposal',50,1), ($5,$6,$7,'Other Discovery',10,0)`,
		workspaceID.PG(), pipelineFirstID.PG(), pipelineID.PG(), pipelineSecondID.PG(),
		otherWorkspaceID.PG(), otherPipelineStageID.PG(), otherPipelineID.PG()); err != nil {
		t.Fatal(err)
	}
	leadInRestrictedStageID, leadInOpenStageID := stageAccessID(t), stageAccessID(t)
	dealInRestrictedStageID, dealInOpenStageID := stageAccessID(t), stageAccessID(t)
	if _, err = admin.Exec(ctx, `INSERT INTO sales.leads(workspace_id,id,name,status,stage_id) VALUES
 ($1,$2,'Restricted lead','new',$3), ($1,$4,'Visible lead','qualified',$5)`,
		workspaceID.PG(), leadInRestrictedStageID.PG(), leadNewID.PG(), leadInOpenStageID.PG(), leadQualifiedID.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO sales.deals
 (workspace_id,id,pipeline_id,stage_id,name,amount_minor,currency,position) VALUES
 ($1,$2,$3,$4,'Restricted deal',10000,'USD',0),
 ($1,$5,$3,$6,'Visible deal',20000,'USD',0)`,
		workspaceID.PG(), dealInRestrictedStageID.PG(), pipelineID.PG(), pipelineFirstID.PG(),
		dealInOpenStageID.PG(), pipelineSecondID.PG()); err != nil {
		t.Fatal(err)
	}

	tenantService := tenancy.NewService(appPool)
	salesService := sales.NewService()
	reportingService := reporting.NewService()
	withWorkspace := func(userID, targetWorkspaceID ids.UUID, permission tenancy.Permission, requestID string, fn func(*tenancy.WorkspaceTx) error) error {
		return tenantService.WithWorkspace(ctx, identity.Principal{UserID: userID}, targetWorkspaceID, requestID, permission, fn)
	}
	requireLead := func(userID, stageID ids.UUID, action sales.StageAccessAction) error {
		return withWorkspace(userID, workspaceID, tenancy.PermissionLeadsRead, "stage-access-check", func(workspace *tenancy.WorkspaceTx) error {
			return salesService.RequireLeadStageAccess(ctx, workspace, workspaceID, stageID, action)
		})
	}
	requirePipeline := func(userID, stageID ids.UUID, action sales.StageAccessAction) error {
		return withWorkspace(userID, workspaceID, tenancy.PermissionDealsRead, "pipeline-stage-access-check", func(workspace *tenancy.WorkspaceTx) error {
			return salesService.RequirePipelineStageAccess(ctx, workspace, workspaceID, stageID, action)
		})
	}
	replaceLead := func(targetStageID ids.UUID, rules []sales.StageRoleAccessInput) error {
		metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "replace-lead-stage-access"}
		return withWorkspace(ownerID, workspaceID, tenancy.PermissionLeadStagesManage, metadata.RequestID, func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := salesService.ReplaceLeadStageRoleAccess(ctx, workspace, metadata, targetStageID, rules)
			return replaceErr
		})
	}
	replacePipeline := func(targetStageID ids.UUID, rules []sales.StageRoleAccessInput) error {
		metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "replace-pipeline-stage-access"}
		return withWorkspace(ownerID, workspaceID, tenancy.PermissionDealStagesManage, metadata.RequestID, func(workspace *tenancy.WorkspaceTx) error {
			_, replaceErr := salesService.ReplacePipelineStageRoleAccess(ctx, workspace, metadata, targetStageID, rules)
			return replaceErr
		})
	}

	// Without explicit rules the coarse resource permission remains authoritative.
	for _, action := range []sales.StageAccessAction{sales.StageAccessView, sales.StageAccessEnter, sales.StageAccessLeave} {
		if err := requireLead(salesID, leadNewID, action); err != nil {
			t.Fatalf("unrestricted lead stage action %s: %v", action, err)
		}
	}
	if err := requirePipeline(viewerID, pipelineFirstID, sales.StageAccessView); err != nil {
		t.Fatalf("unrestricted pipeline stage view: %v", err)
	}

	leadRules := []sales.StageRoleAccessInput{{RoleID: salesRoleID, CanView: true, CanEnter: true}}
	if err := replaceLead(leadNewID, leadRules); err != nil {
		t.Fatal(err)
	}
	if err := requireLead(salesID, leadNewID, sales.StageAccessView); err != nil {
		t.Fatalf("explicit sales view: %v", err)
	}
	if err := requireLead(salesID, leadNewID, sales.StageAccessEnter); err != nil {
		t.Fatalf("explicit sales enter: %v", err)
	}
	if err := requireLead(salesID, leadNewID, sales.StageAccessLeave); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("sales leave error=%v, want forbidden", err)
	}
	if err := requireLead(viewerID, leadNewID, sales.StageAccessView); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("missing viewer rule error=%v, want fail-closed forbidden", err)
	}
	err = withWorkspace(viewerID, workspaceID, tenancy.PermissionLeadsRead, "lead-list-stage-filter", func(workspace *tenancy.WorkspaceTx) error {
		page, listErr := salesService.ListLeads(ctx, workspace, workspaceID, sales.LeadListFilter{Limit: 50})
		if listErr != nil {
			return listErr
		}
		if len(page.Items) != 1 || page.Items[0].ID != leadInOpenStageID.String() {
			return errors.New("lead list exposed a record in a hidden stage")
		}
		stages, listErr := salesService.ListLeadStages(ctx, workspace, workspaceID)
		if listErr != nil {
			return listErr
		}
		for _, stage := range stages {
			if stage.ID == leadNewID.String() {
				return errors.New("lead stage list exposed a hidden stage")
			}
		}
		if _, getErr := salesService.GetLead(ctx, workspace, workspaceID, leadInRestrictedStageID); !errors.Is(getErr, errx.ErrNotFound) {
			return errors.New("direct lead read exposed a record in a hidden stage")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, bypassUserID := range []ids.UUID{ownerID, adminID} {
		if err := requireLead(bypassUserID, leadNewID, sales.StageAccessLeave); err != nil {
			t.Fatalf("owner/admin bypass for %s: %v", bypassUserID, err)
		}
	}

	// A transition requires leave on its source and enter on its destination.
	err = withWorkspace(salesID, workspaceID, tenancy.PermissionLeadsUpdate, "lead-transition-denied", func(workspace *tenancy.WorkspaceTx) error {
		return salesService.RequireLeadStageTransition(ctx, workspace, workspaceID, leadNewID, leadQualifiedID)
	})
	if !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("transition without leave error=%v, want forbidden", err)
	}
	if err := replaceLead(leadNewID, []sales.StageRoleAccessInput{{RoleID: salesRoleID, CanView: true, CanEnter: true, CanLeave: true}}); err != nil {
		t.Fatal(err)
	}
	if err := replaceLead(leadQualifiedID, []sales.StageRoleAccessInput{{RoleID: salesRoleID, CanView: true, CanLeave: true}}); err != nil {
		t.Fatal(err)
	}
	err = withWorkspace(salesID, workspaceID, tenancy.PermissionLeadsUpdate, "lead-transition-target-denied", func(workspace *tenancy.WorkspaceTx) error {
		return salesService.RequireLeadStageTransition(ctx, workspace, workspaceID, leadNewID, leadQualifiedID)
	})
	if !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("transition without enter error=%v, want forbidden", err)
	}
	if err := replaceLead(leadQualifiedID, []sales.StageRoleAccessInput{{RoleID: salesRoleID, CanView: true, CanEnter: true, CanLeave: true}}); err != nil {
		t.Fatal(err)
	}
	if err := withWorkspace(salesID, workspaceID, tenancy.PermissionLeadsUpdate, "lead-transition-allowed", func(workspace *tenancy.WorkspaceTx) error {
		return salesService.RequireLeadStageTransition(ctx, workspace, workspaceID, leadNewID, leadQualifiedID)
	}); err != nil {
		t.Fatalf("fully granted transition: %v", err)
	}

	if err := replacePipeline(pipelineFirstID, []sales.StageRoleAccessInput{{
		RoleID: salesRoleID, CanEnter: true, CanLeave: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := requirePipeline(salesID, pipelineFirstID, sales.StageAccessView); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("independent pipeline view error=%v, want forbidden", err)
	}
	if err := requirePipeline(salesID, pipelineFirstID, sales.StageAccessEnter); err != nil {
		t.Fatalf("independent pipeline enter: %v", err)
	}
	err = withWorkspace(salesID, workspaceID, tenancy.PermissionDealsRead, "deal-list-stage-filter", func(workspace *tenancy.WorkspaceTx) error {
		page, listErr := salesService.ListDeals(ctx, workspace, workspaceID, nil, nil, "", 50)
		if listErr != nil {
			return listErr
		}
		if len(page.Items) != 1 {
			return errors.New("deal list did not filter the hidden pipeline stage")
		}
		visibleID, valid := ids.FromPG(page.Items[0].ID)
		if !valid || visibleID != dealInOpenStageID {
			return errors.New("deal list exposed a record in a hidden pipeline stage")
		}
		if _, getErr := salesService.GetDeal(ctx, workspace, workspaceID, dealInRestrictedStageID); !errors.Is(getErr, errx.ErrNotFound) {
			return errors.New("direct deal read exposed a record in a hidden pipeline stage")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = withWorkspace(viewerID, workspaceID, tenancy.PermissionReportsRead, "dashboard-stage-filter", func(workspace *tenancy.WorkspaceTx) error {
		dashboard, loadErr := reportingService.Dashboard(ctx, workspace, workspaceID)
		if loadErr != nil {
			return loadErr
		}
		if dashboard.OpenPipelineMinor != 20000 {
			return errors.New("dashboard aggregate included a deal in a hidden stage")
		}
		var stages []struct {
			StageID string `json:"stageId"`
		}
		if unmarshalErr := json.Unmarshal(dashboard.DealsByStage, &stages); unmarshalErr != nil {
			return unmarshalErr
		}
		if len(stages) != 1 || stages[0].StageID != pipelineSecondID.String() {
			return errors.New("dashboard stage breakdown exposed a hidden stage")
		}
		report, reportErr := reportingService.PeriodReport(ctx, workspace, workspaceID, reporting.Period{
			Start: time.Now().UTC().Add(-time.Hour), End: time.Now().UTC().Add(time.Hour), Timezone: "UTC",
		})
		if reportErr != nil {
			return reportErr
		}
		if len(report.DealsByStage) != 1 || report.DealsByStage[0].StageID != pipelineSecondID {
			return errors.New("period report exposed a hidden deal stage")
		}
		if report.Overview.LeadCount != 0 {
			return errors.New("period report included leads in hidden stages")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePipeline(ownerID, pipelineFirstID, sales.StageAccessView); err != nil {
		t.Fatalf("pipeline owner bypass: %v", err)
	}
	if err := withWorkspace(salesID, workspaceID, tenancy.PermissionDealsUpdate, "pipeline-transition-allowed", func(workspace *tenancy.WorkspaceTx) error {
		return salesService.RequirePipelineStageTransition(ctx, workspace, workspaceID, pipelineFirstID, pipelineSecondID)
	}); err != nil {
		t.Fatalf("pipeline transition with unrestricted destination: %v", err)
	}

	// Composite foreign keys reject a role from another workspace and the failed
	// replacement rolls back the preceding delete.
	err = replaceLead(leadNewID, []sales.StageRoleAccessInput{{RoleID: otherOwnerRoleID, CanView: true}})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("cross-tenant role replacement error=%v, want validation", err)
	}
	var persistedRules []sales.StageRoleAccessRule
	err = withWorkspace(ownerID, workspaceID, tenancy.PermissionLeadStagesManage, "list-persisted-stage-access", func(workspace *tenancy.WorkspaceTx) error {
		var listErr error
		persistedRules, listErr = salesService.ListLeadStageRoleAccess(ctx, workspace, workspaceID, leadNewID)
		return listErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(persistedRules) != 1 || persistedRules[0].RoleID != salesRoleID || !persistedRules[0].CanLeave {
		t.Fatalf("rules changed after rolled-back replacement: %+v", persistedRules)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO sales.lead_stage_role_access
 (workspace_id,stage_id,role_id,can_view) VALUES($1,$2,$3,true)`,
		workspaceID.PG(), leadNewID.PG(), otherOwnerRoleID.PG()); err == nil {
		t.Fatal("cross-workspace role unexpectedly satisfied composite foreign key")
	}

	otherMetadata := events.Metadata{WorkspaceID: otherWorkspaceID, ActorID: otherOwnerID, RequestID: "other-stage-access"}
	if err := withWorkspace(otherOwnerID, otherWorkspaceID, tenancy.PermissionLeadStagesManage, otherMetadata.RequestID, func(workspace *tenancy.WorkspaceTx) error {
		_, replaceErr := salesService.ReplaceLeadStageRoleAccess(ctx, workspace, otherMetadata, otherLeadID,
			[]sales.StageRoleAccessInput{{RoleID: otherOwnerRoleID}})
		return replaceErr
	}); err != nil {
		t.Fatal(err)
	}
	err = withWorkspace(ownerID, workspaceID, tenancy.PermissionLeadStagesManage, "cross-tenant-stage-list", func(workspace *tenancy.WorkspaceTx) error {
		if _, listErr := salesService.ListLeadStageRoleAccess(ctx, workspace, workspaceID, otherLeadID); !errors.Is(listErr, errx.ErrNotFound) {
			return errors.New("cross-tenant stage was not hidden")
		}
		var visibleCount int
		if queryErr := workspace.Tx.QueryRow(ctx, `SELECT count(*) FROM sales.lead_stage_role_access WHERE workspace_id=$1`,
			otherWorkspaceID.PG()).Scan(&visibleCount); queryErr != nil {
			return queryErr
		}
		if visibleCount != 0 {
			return errors.New("RLS exposed another workspace's stage rules")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := requireLead(salesID, otherLeadID, sales.StageAccessView); !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("cross-tenant access decision error=%v, want forbidden", err)
	}
}

func stageAccessID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
