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
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/sales"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestLeadStagesLifecycleAndIsolationOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "lead-stages-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "lead-stages-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	ownerID, workspaceID, otherWorkspaceID := testID(t), testID(t), testID(t)
	membershipID := testID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	_, err = admin.Exec(ctx, `INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash)
VALUES ($1,$2,$2,'Lead Stage Owner','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		ownerID.PG(), "lead-stages-"+suffix+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES
 ($1,'Lead Stages',$2), ($3,'Other Lead Stages',$4)`,
		workspaceID.PG(), "lead-stages-"+suffix,
		otherWorkspaceID.PG(), "other-lead-stages-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role)
VALUES ($1,$2,$3,'owner')`, workspaceID.PG(), membershipID.PG(), ownerID.PG()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id = ANY($1::uuid[])`,
			[]string{workspaceID.String(), otherWorkspaceID.String()})
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, ownerID.PG())
	}()

	tenantService := tenancy.NewService(appPool)
	salesService := sales.NewService()
	principal := identity.Principal{UserID: ownerID}
	metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "lead-stages-test"}
	var stages []sales.LeadStageRecord
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var listErr error
			stages, listErr = salesService.ListLeadStages(ctx, workspace, workspaceID)
			return listErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 4 {
		t.Fatalf("bootstrap lead stages=%d, want 4", len(stages))
	}

	var custom sales.LeadStageRecord
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			custom, createErr = salesService.CreateLeadStage(ctx, workspace, metadata, sales.LeadStageInput{
				Name: "Discovery", Category: "qualified", Color: "#7c3aed",
			})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}
	var stagesWithCustom []sales.LeadStageRecord
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var listErr error
			stagesWithCustom, listErr = salesService.ListLeadStages(ctx, workspace, workspaceID)
			return listErr
		})
	if err != nil {
		t.Fatal(err)
	}
	order := make([]sales.StageOrderItem, 0, len(stagesWithCustom))
	for index := len(stagesWithCustom) - 1; index >= 0; index-- {
		stage := stagesWithCustom[index]
		order = append(order, sales.StageOrderItem{ID: ids.MustParse(stage.ID), Version: stage.Version})
	}
	var reordered []sales.LeadStageRecord
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionLeadStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			var reorderErr error
			reordered, reorderErr = salesService.ReorderLeadStages(ctx, workspace, metadata, order)
			return reorderErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(reordered) != len(stagesWithCustom) || reordered[0].ID != custom.ID {
		t.Fatalf("reordered first stage=%v, want custom %s", reordered, custom.ID)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionLeadStagesManage,
		func(workspace *tenancy.WorkspaceTx) error {
			_, reorderErr := salesService.ReorderLeadStages(ctx, workspace, metadata, order)
			return reorderErr
		})
	if !errors.Is(err, errx.ErrVersionConflict) {
		t.Fatalf("stale reorder error=%v, want version conflict", err)
	}
	customID := ids.MustParse(custom.ID)
	var lead sales.LeadRecord
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsCreate,
		func(workspace *tenancy.WorkspaceTx) error {
			var createErr error
			lead, createErr = salesService.CreateLead(ctx, workspace, metadata, sales.LeadInput{
				Name: "Synthetic Prospect", Status: "qualified", StageID: &customID, CustomFields: map[string]any{},
			})
			return createErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if lead.StageID != custom.ID || lead.Status != "qualified" {
		t.Fatalf("created lead stage=%s status=%s", lead.StageID, lead.Status)
	}

	var newStage, convertedStage sales.LeadStageRecord
	for _, stage := range stages {
		if stage.SystemKey != nil && *stage.SystemKey == "new" {
			newStage = stage
		}
		if stage.SystemKey != nil && *stage.SystemKey == "converted" {
			convertedStage = stage
		}
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var moveErr error
			lead, moveErr = salesService.MoveLeadStage(ctx, workspace, metadata, ids.MustParse(lead.ID), ids.MustParse(newStage.ID), lead.Version)
			return moveErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if lead.StageID != newStage.ID || lead.Status != "new" {
		t.Fatalf("moved lead stage=%s status=%s", lead.StageID, lead.Status)
	}

	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, moveErr := salesService.MoveLeadStage(ctx, workspace, metadata, ids.MustParse(lead.ID), customID, lead.Version-1)
			return moveErr
		})
	if !errors.Is(err, errx.ErrVersionConflict) {
		t.Fatalf("stale move error=%v, want version conflict", err)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, moveErr := salesService.MoveLeadStage(ctx, workspace, metadata, ids.MustParse(lead.ID), ids.MustParse(convertedStage.ID), lead.Version)
			return moveErr
		})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("direct converted move error=%v, want validation error", err)
	}

	var otherStageID string
	if err := admin.QueryRow(ctx, `SELECT id::text FROM sales.lead_stages WHERE workspace_id=$1 ORDER BY position LIMIT 1`, otherWorkspaceID.PG()).Scan(&otherStageID); err != nil {
		t.Fatal(err)
	}
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			_, moveErr := salesService.MoveLeadStage(ctx, workspace, metadata, ids.MustParse(lead.ID), ids.MustParse(otherStageID), lead.Version)
			return moveErr
		})
	if !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("cross-workspace stage move error=%v, want forbidden", err)
	}

	var historyCount int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales.lead_stage_history WHERE workspace_id=$1 AND lead_id=$2`,
		workspaceID.PG(), ids.MustParse(lead.ID).PG()).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if historyCount != 2 {
		t.Fatalf("stage history rows=%d, want create+move", historyCount)
	}

	legacyLeadID := testID(t)
	if _, err := admin.Exec(ctx, `INSERT INTO sales.leads(workspace_id,id,name,status) VALUES($1,$2,'Legacy insert','new')`,
		workspaceID.PG(), legacyLeadID.PG()); err != nil {
		t.Fatalf("legacy insert without stage_id: %v", err)
	}
	var legacyCategory string
	if err := admin.QueryRow(ctx, `SELECT stage.category FROM sales.leads lead JOIN sales.lead_stages stage
	  ON stage.workspace_id=lead.workspace_id AND stage.id=lead.stage_id WHERE lead.workspace_id=$1 AND lead.id=$2`,
		workspaceID.PG(), legacyLeadID.PG()).Scan(&legacyCategory); err != nil {
		t.Fatal(err)
	}
	if legacyCategory != "new" {
		t.Fatalf("legacy lead category=%s, want new", legacyCategory)
	}
}

func testID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
