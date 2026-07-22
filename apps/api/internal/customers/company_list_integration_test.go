//go:build integration

package customers_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestCompanyListFiltersAndCursorOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "company-list-test-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 1, "company-list-test-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceID := mustCompanyListID(t)
	userID := mustCompanyListID(t)
	membershipID := mustCompanyListID(t)
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	_, err = admin.Exec(ctx, `
		INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash)
		VALUES ($1,$2,$2,'Company List User','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')
	`, userID.PG(), "company-list-"+suffix+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, userID.PG())
	}()
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES ($1,'Company List Workspace',$2)`,
		workspaceID.PG(), "company-list-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES ($1,$2,$3,'owner')`,
		workspaceID.PG(), membershipID.PG(), userID.PG()); err != nil {
		t.Fatal(err)
	}
	firstID := mustCompanyListID(t)
	secondID := mustCompanyListID(t)
	inactiveID := mustCompanyListID(t)
	_, err = admin.Exec(ctx, `
		INSERT INTO customers.companies(workspace_id,id,name,domain,domain_normalized,status,updated_at)
		VALUES
		  ($1,$2,'Alpha New','new.example','new.example','active','2026-01-03T00:00:00Z'),
		  ($1,$3,'Alpha Earlier','earlier.example','earlier.example','active','2026-01-02T00:00:00Z'),
		  ($1,$4,'Alpha Inactive','inactive.example','inactive.example','inactive','2026-01-01T00:00:00Z')
	`, workspaceID.PG(), firstID.PG(), secondID.PG(), inactiveID.PG())
	if err != nil {
		t.Fatal(err)
	}

	principal := identity.Principal{UserID: userID, Email: "company-list-" + suffix + "@example.invalid"}
	tenantService := tenancy.NewService(appPool)
	customerService := customers.NewService()
	var firstPage, secondPage customers.CompanyPage
	err = tenantService.WithWorkspace(ctx, principal, workspaceID, "company-list-test", tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var listErr error
			firstPage, listErr = customerService.ListCompaniesPage(ctx, workspace, workspaceID, "alpha", "active", "", 1)
			if listErr != nil {
				return listErr
			}
			secondPage, listErr = customerService.ListCompaniesPage(ctx, workspace, workspaceID, "alpha", "active", firstPage.NextCursor, 1)
			return listErr
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].Status != "active" || firstPage.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", firstPage)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].Status != "active" || secondPage.NextCursor != "" {
		t.Fatalf("unexpected second page: %#v", secondPage)
	}
	if firstPage.Items[0].ID == secondPage.Items[0].ID {
		t.Fatal("cursor repeated the first company")
	}
}

func mustCompanyListID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
