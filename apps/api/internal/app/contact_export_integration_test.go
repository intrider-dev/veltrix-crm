//go:build integration

package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestContactExportEnforcesTenantIsolationForSessionsAndAPIKeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adminURL, appURL := isolatedContactExportDatabase(t, ctx)
	admin, err := database.Open(ctx, adminURL, 2, "contact-export-integration-admin")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	appPool, err := database.Open(ctx, appURL, 4, "contact-export-integration-app")
	if err != nil {
		t.Fatal(err)
	}
	defer appPool.Close()

	workspaceA, workspaceB := mustContactExportID(t), mustContactExportID(t)
	userA, userB := mustContactExportID(t), mustContactExportID(t)
	membershipA, membershipB := mustContactExportID(t), mustContactExportID(t)
	contactA, contactB := mustContactExportID(t), mustContactExportID(t)
	suffix := compactContactExportID(workspaceA)[:18]
	emailA := "export-a-" + suffix + "@example.invalid"
	emailB := "export-b-" + suffix + "@example.invalid"
	password := "Integration-export-123!"
	passwordHash, err := identity.NewPasswordHasher(1).Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash) VALUES
  ($1,$2,$2,'Export user A',$3),
  ($4,$5,$5,'Export user B',$3)`,
		userA.PG(), emailA, passwordHash, userB.PG(), emailB); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.workspaces(id,name,slug) VALUES
  ($1,'Export tenant A',$2),
  ($3,'Export tenant B',$4)`,
		workspaceA.PG(), "export-a-"+suffix, workspaceB.PG(), "export-b-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES
  ($1,$2,$3,'owner'),
  ($4,$5,$6,'owner')`,
		workspaceA.PG(), membershipA.PG(), userA.PG(), workspaceB.PG(), membershipB.PG(), userB.PG()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
INSERT INTO customers.contacts(workspace_id,id,first_name,last_name,display_name,email,email_normalized) VALUES
  ($1,$2,'Alpha','Sentinel','Alpha export sentinel',$3,$3),
  ($4,$5,'Omega','Secret','Omega foreign sentinel',$6,$6)`,
		workspaceA.PG(), contactA.PG(), "alpha-"+suffix+"@example.invalid",
		workspaceB.PG(), contactB.PG(), "omega-"+suffix+"@example.invalid"); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(config.Config{
		Environment:      "development",
		PublicURL:        "http://example.test",
		DefaultLocale:    "en",
		SupportedLocales: []string{"en", "ru"},
		SessionTTL:       time.Hour,
		PasswordResetTTL: time.Hour,
		MFAChallengeTTL:  5 * time.Minute,
		MFASetupTTL:      10 * time.Minute,
		UploadDir:        t.TempDir(),
		MaxUploadBytes:   1 << 20,
		StorageBackend:   "local",
		AIProvider:       "disabled",
		CallsProvider:    "disabled",
	}, logger, appPool, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := application.Close(); err != nil {
			t.Errorf("close application: %v", err)
		}
	}()

	session, err := application.identity.Login(ctx, emailA, password, "contact-export-integration", nil)
	if err != nil {
		t.Fatalf("login fixture user: %v", err)
	}
	ownSessionExport := serveContactExport(application, workspaceA, session.Token, "")
	assertContactExportOnlyContainsTenantA(t, ownSessionExport, suffix)

	foreignSessionExport := serveContactExport(application, workspaceB, session.Token, "")
	if foreignSessionExport.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant session export status=%d body=%s, want 404",
			foreignSessionExport.Code, foreignSessionExport.Body.String())
	}

	generated, err := integrations.NewAPIKeyService(integrations.NewPostgresRepository(appPool)).Create(
		ctx, integrations.APIKeyCreate{
			WorkspaceID: workspaceA, CreatedBy: userA, Name: "contact export integration",
			Scopes: []integrations.Scope{integrations.ScopeContactsRead}, Now: time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatalf("create API key: %v", err)
	}
	ownKeyExport := serveContactExport(application, workspaceA, "", generated.Token)
	assertContactExportOnlyContainsTenantA(t, ownKeyExport, suffix)

	foreignKeyExport := serveContactExport(application, workspaceB, "", generated.Token)
	if foreignKeyExport.Code != http.StatusForbidden {
		t.Fatalf("cross-tenant API key export status=%d body=%s, want 403",
			foreignKeyExport.Code, foreignKeyExport.Body.String())
	}

	if _, err := admin.Exec(ctx, `
UPDATE tenancy.memberships SET status='disabled',updated_at=now()
WHERE workspace_id=$1 AND user_id=$2`, workspaceA.PG(), userA.PG()); err != nil {
		t.Fatal(err)
	}
	disabledMemberExport := serveContactExport(application, workspaceA, "", generated.Token)
	if disabledMemberExport.Code != http.StatusNotFound {
		t.Fatalf("disabled API-key owner export status=%d body=%s, want 404",
			disabledMemberExport.Code, disabledMemberExport.Body.String())
	}
}

func serveContactExport(application *Application, workspaceID ids.UUID, sessionToken, apiKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/workspaces/"+workspaceID.String()+"/contacts/export", nil)
	request.RemoteAddr = "192.0.2.10:42000"
	if sessionToken != "" {
		request.AddCookie(&http.Cookie{Name: application.sessionCookieName(), Value: sessionToken})
	}
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response := httptest.NewRecorder()
	application.Handler().ServeHTTP(response, request)
	return response
}

func assertContactExportOnlyContainsTenantA(t *testing.T, response *httptest.ResponseRecorder, suffix string) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("own-tenant export status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "Alpha export sentinel") || !strings.Contains(body, "alpha-"+suffix+"@example.invalid") {
		t.Fatalf("own-tenant CSV is missing sentinel: %s", body)
	}
	if strings.Contains(body, "Omega foreign sentinel") || strings.Contains(body, "omega-"+suffix+"@example.invalid") {
		t.Fatalf("own-tenant CSV leaked foreign sentinel: %s", body)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("export content type=%q, want text/csv", contentType)
	}
}

func isolatedContactExportDatabase(t *testing.T, ctx context.Context) (string, string) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	appPassword := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || appPassword == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	coordinator, err := database.Open(ctx, adminURL, 1, "contact-export-test-coordinator")
	if err != nil {
		t.Fatal(err)
	}
	databaseID := mustContactExportID(t)
	databaseName := "veltrix_export_it_" + compactContactExportID(databaseID)[:20]
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := coordinator.Exec(ctx, "CREATE DATABASE "+quotedName); err != nil {
		coordinator.Close()
		t.Fatalf("create isolated database: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := coordinator.Exec(cleanupCtx, "DROP DATABASE "+quotedName+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated database: %v", err)
		}
		coordinator.Close()
	})

	testAdminURL := contactExportDatabaseURL(t, adminURL, databaseName)
	if err := database.Migrate(ctx, testAdminURL, appPassword); err != nil {
		t.Fatalf("migrate isolated database: %v", err)
	}
	return testAdminURL, contactExportDatabaseURL(t, appURL, databaseName)
}

func contactExportDatabaseURL(t *testing.T, rawURL, databaseName string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	parsed.Path = "/" + databaseName
	return parsed.String()
}

func mustContactExportID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func compactContactExportID(id ids.UUID) string {
	return strings.ReplaceAll(id.String(), "-", "")
}
