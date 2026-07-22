//go:build integration

package localization_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestDomainContentTranslationLifecycleOnPostgreSQL(t *testing.T) {
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
	admin, err := database.Open(ctx, adminURL, 1, "localization-content-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 2, "localization-content-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	ownerID := localizationTestID(t)
	workspaceID := localizationTestID(t)
	membershipID := localizationTestID(t)
	resourceID := localizationTestID(t).String()
	suffix := strings.ReplaceAll(workspaceID.String(), "-", "")[:12]
	if _, err = admin.Exec(ctx, `INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash)
VALUES ($1,$2,$2,'Translation Owner','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')`,
		ownerID.PG(), "translation-"+suffix+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug,default_locale,supported_locales)
VALUES ($1,'Translations',$2,'en',ARRAY['en','ru'])`, workspaceID.PG(), "translations-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role)
VALUES ($1,$2,$3,'owner')`, workspaceID.PG(), membershipID.PG(), ownerID.PG()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenancy.workspaces WHERE id=$1`, workspaceID.PG())
		_, _ = admin.Exec(context.Background(), `DELETE FROM identity.users WHERE id=$1`, ownerID.PG())
	}()

	service := localization.NewContentService([]string{"en", "ru"})
	tenants := tenancy.NewService(appPool)
	principal := identity.Principal{UserID: ownerID, PreferredLocale: "ru"}
	metadata := events.Metadata{WorkspaceID: workspaceID, ActorID: ownerID, RequestID: "content-lifecycle"}
	err = tenants.WithWorkspace(ctx, principal, workspaceID, metadata.RequestID, tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			if _, registerErr := service.RegisterResource(ctx, workspace, metadata,
				"sales.lead_stage.name", resourceID,
				localization.ContentResourceInput{SourceLocale: "en", SourceText: "Discovery"}); registerErr != nil {
				return registerErr
			}
			if _, putErr := service.Put(ctx, workspace, metadata, "ru", "sales.lead_stage.name", resourceID,
				localization.ContentTranslationInput{
					SourceLocale: "en", SourceText: "Discovery", TranslatedText: "Выявление потребности",
					Status: "published", Version: 0,
				}); putErr != nil {
				return putErr
			}
			resolved, resolveErr := service.ResolveBatch(ctx, workspace, workspaceID, "ru",
				"sales.lead_stage.name", []string{resourceID})
			if resolveErr != nil {
				return resolveErr
			}
			if got := resolved[resourceID]; got.Text != "Выявление потребности" || got.Locale != "ru" {
				t.Fatalf("published resolution=%+v", got)
			}
			if _, registerErr := service.RegisterResource(ctx, workspace, metadata,
				"sales.lead_stage.name", resourceID,
				localization.ContentResourceInput{SourceLocale: "en", SourceText: "Needs analysis"}); registerErr != nil {
				return registerErr
			}
			resolved, resolveErr = service.ResolveBatch(ctx, workspace, workspaceID, "ru",
				"sales.lead_stage.name", []string{resourceID})
			if resolveErr != nil {
				return resolveErr
			}
			if got := resolved[resourceID]; got.Text != "Needs analysis" || got.Locale != "en" {
				t.Fatalf("renamed source resolution=%+v", got)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := admin.QueryRow(ctx, `SELECT status FROM localization.content_translations
WHERE workspace_id=$1 AND namespace='sales.lead_stage.name' AND resource_key=$2 AND locale='ru'`,
		workspaceID.PG(), resourceID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("translation status=%q, want draft after source rename", status)
	}
}

func localizationTestID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
