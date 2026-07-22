//go:build integration

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type fixture struct {
	admin      *pgxpool.Pool
	app        *pgxpool.Pool
	workspaceA ids.UUID
	workspaceB ids.UUID
	userA      ids.UUID
	userB      ids.UUID
	contactA   ids.UUID
	contactB   ids.UUID
	projectA   ids.UUID
	projectB   ids.UUID
}

func TestRuntimeRoleAndRLSIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	item := newFixture(t, ctx)
	defer item.cleanup(t)

	t.Run("runtime role cannot bypass RLS", func(t *testing.T) {
		var bypass, superuser bool
		if err := item.app.QueryRow(ctx, `
SELECT rolbypassrls, rolsuper FROM pg_roles WHERE rolname = current_user
`).Scan(&bypass, &superuser); err != nil {
			t.Fatal(err)
		}
		if bypass || superuser {
			t.Fatalf("runtime role is privileged: bypass=%v superuser=%v", bypass, superuser)
		}
	})

	t.Run("empty context is fail closed", func(t *testing.T) {
		var count int
		if err := item.app.QueryRow(ctx, `SELECT count(*) FROM customers.contacts`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("expected zero rows without context, got %d", count)
		}
		if _, err := item.app.Exec(ctx, `
INSERT INTO customers.contacts(workspace_id,id,first_name,last_name,display_name)
VALUES($1,$2,'Blocked','Insert','Blocked Insert')
`, item.workspaceA.PG(), mustID(t).PG()); err == nil {
			t.Fatal("insert without tenant context unexpectedly succeeded")
		}
	})

	t.Run("tenant A cannot observe or mutate tenant B", func(t *testing.T) {
		tx := beginTenant(t, ctx, item.app, item.userA, item.workspaceA)
		defer func() { _ = tx.Rollback(ctx) }()
		var visible []string
		rows, err := tx.Query(ctx, `SELECT display_name FROM customers.contacts ORDER BY display_name`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatal(err)
			}
			visible = append(visible, name)
		}
		rows.Close()
		if len(visible) != 1 || visible[0] != "Tenant A Contact" {
			t.Fatalf("unexpected visible rows: %v", visible)
		}

		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM customers.contacts WHERE id=$1`, item.contactB.PG()).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("foreign contact was readable by exact id")
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM search.documents WHERE searchable_text ILIKE '%secret omega%'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("foreign search document was visible")
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM projects.projects WHERE id=$1`, item.projectB.PG()).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("foreign project was visible")
		}
		command, err := tx.Exec(ctx, `UPDATE customers.contacts SET status='inactive' WHERE id=$1`, item.contactB.PG())
		if err != nil {
			t.Fatal(err)
		}
		if command.RowsAffected() != 0 {
			t.Fatal("foreign contact was updated")
		}
		command, err = tx.Exec(ctx, `DELETE FROM customers.contacts WHERE id=$1`, item.contactB.PG())
		if err != nil {
			t.Fatal(err)
		}
		if command.RowsAffected() != 0 {
			t.Fatal("foreign contact was deleted")
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO customers.contacts(workspace_id,id,first_name,last_name,display_name)
VALUES($1,$2,'Cross','Tenant','Cross Tenant')
`, item.workspaceB.PG(), mustID(t).PG()); err == nil {
			t.Fatal("tenant A inserted a tenant B row")
		}
	})

	t.Run("transaction local context does not leak through pool", func(t *testing.T) {
		tx := beginTenant(t, ctx, item.app, item.userA, item.workspaceA)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var workspaceSetting *string
		if err := item.app.QueryRow(ctx, `SELECT NULLIF(current_setting('app.workspace_id', true), '')`).Scan(&workspaceSetting); err != nil {
			t.Fatal(err)
		}
		if workspaceSetting != nil {
			t.Fatalf("workspace context leaked after commit: %q", *workspaceSetting)
		}
		var count int
		if err := item.app.QueryRow(ctx, `SELECT count(*) FROM customers.contacts`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("pool reuse leaked %d rows", count)
		}
	})
}

func TestTenantTableCatalogInvariants(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	item := newFixture(t, ctx)
	defer item.cleanup(t)

	rows, err := item.admin.Query(ctx, `
SELECT n.nspname, c.relname, c.relrowsecurity, c.relforcerowsecurity,
       EXISTS (SELECT 1 FROM pg_policy p WHERE p.polrelid = c.oid) AS has_policy
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'workspace_id' AND NOT a.attisdropped
WHERE c.relkind = 'r'
  AND n.nspname IN ('tenancy','customers','sales','activities','projects','collaboration','automation','notifications','reporting','search','files','integrations','audit','platform')
  AND NOT (n.nspname = 'platform' AND c.relname = 'seed_runs')
ORDER BY n.nspname, c.relname
`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		var schema, table string
		var enabled, forced, policy bool
		if err := rows.Scan(&schema, &table, &enabled, &forced, &policy); err != nil {
			t.Fatal(err)
		}
		if !enabled || !forced || !policy {
			t.Errorf("%s.%s: enabled=%v forced=%v policy=%v", schema, table, enabled, forced, policy)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 20 {
		t.Fatalf("catalog test covered only %d tenant tables", count)
	}
}

func newFixture(t *testing.T, ctx context.Context) fixture {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_ADMIN_URL")
	appURL := os.Getenv("TEST_DATABASE_URL")
	password := os.Getenv("TEST_APP_DB_PASSWORD")
	if adminURL == "" || appURL == "" || password == "" {
		t.Skip("set TEST_DATABASE_ADMIN_URL, TEST_DATABASE_URL and TEST_APP_DB_PASSWORD")
	}
	if err := database.Migrate(ctx, adminURL, password); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	admin, err := database.Open(ctx, adminURL, 2, "crm-integration-admin")
	if err != nil {
		t.Fatal(err)
	}
	appPool, err := database.Open(ctx, appURL, 1, "crm-integration-app")
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	item := fixture{
		admin: admin, app: appPool,
		workspaceA: mustID(t), workspaceB: mustID(t), userA: mustID(t), userB: mustID(t),
		contactA: mustID(t), contactB: mustID(t), projectA: mustID(t), projectB: mustID(t),
	}
	suffix := strings.ReplaceAll(item.workspaceA.String(), "-", "")[:12]
	setup, err := admin.Begin(ctx)
	if err != nil {
		appPool.Close()
		admin.Close()
		t.Fatal(err)
	}
	defer func() { _ = setup.Rollback(ctx) }()
	_, err = setup.Exec(ctx, `
INSERT INTO identity.users(id,email,email_normalized,display_name,password_hash)
VALUES ($1,$2,$2,'Tenant A User','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g'),
	       ($3,$4,$4,'Tenant B User','$argon2id$v=19$m=32768,t=2,p=1$ZmFrZXNhbHQ$ZmFrZWhhc2g')
`, item.userA.PG(), "a-"+suffix+"@example.invalid", item.userB.PG(), "b-"+suffix+"@example.invalid")
	if err == nil {
		_, err = setup.Exec(ctx, `INSERT INTO tenancy.workspaces(id,name,slug) VALUES ($1,'Tenant A',$2), ($3,'Tenant B',$4)`,
			item.workspaceA.PG(), "tenant-a-"+suffix, item.workspaceB.PG(), "tenant-b-"+suffix)
	}
	if err == nil {
		_, err = setup.Exec(ctx, `INSERT INTO tenancy.memberships(workspace_id,id,user_id,role) VALUES ($1,$2,$3,'owner'), ($4,$5,$6,'owner')`,
			item.workspaceA.PG(), mustID(t).PG(), item.userA.PG(), item.workspaceB.PG(), mustID(t).PG(), item.userB.PG())
	}
	if err == nil {
		_, err = setup.Exec(ctx, `INSERT INTO customers.contacts(workspace_id,id,first_name,last_name,display_name) VALUES
  ($1,$2,'Tenant A','Contact','Tenant A Contact'), ($3,$4,'Tenant B','Contact','Tenant B Contact')`,
			item.workspaceA.PG(), item.contactA.PG(), item.workspaceB.PG(), item.contactB.PG())
	}
	if err == nil {
		_, err = setup.Exec(ctx, `INSERT INTO search.documents(workspace_id,entity_type,entity_id,title,searchable_text) VALUES
  ($1,'contact',$2,'Tenant A Contact','tenant a unique alpha'), ($3,'contact',$4,'Tenant B Contact','tenant b secret omega')`,
			item.workspaceA.PG(), item.contactA.PG(), item.workspaceB.PG(), item.contactB.PG())
	}
	if err == nil {
		_, err = setup.Exec(ctx, `INSERT INTO projects.projects(workspace_id,id,name,created_by) VALUES
  ($1,$2,'Tenant A Project',$3), ($4,$5,'Tenant B Project',$6)`,
			item.workspaceA.PG(), item.projectA.PG(), item.userA.PG(), item.workspaceB.PG(), item.projectB.PG(), item.userB.PG())
	}
	if err != nil {
		appPool.Close()
		admin.Close()
		t.Fatalf("create test fixture: %v", err)
	}
	if err := setup.Commit(ctx); err != nil {
		appPool.Close()
		admin.Close()
		t.Fatalf("commit test fixture: %v", err)
	}
	return item
}

func beginTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, actor, workspace ids.UUID) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.actor_id',$1,true), set_config('app.workspace_id',$2,true)`, actor.String(), workspace.String()); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	return tx
}

func (item fixture) cleanup(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := item.admin.Exec(ctx, `DELETE FROM tenancy.workspaces WHERE id IN ($1,$2)`, item.workspaceA.PG(), item.workspaceB.PG()); err != nil {
		t.Logf("fixture cleanup failed: %v", err)
	}
	if _, err := item.admin.Exec(ctx, `DELETE FROM identity.users WHERE id IN ($1,$2)`, item.userA.PG(), item.userB.PG()); err != nil {
		t.Logf("user cleanup failed: %v", err)
	}
	item.app.Close()
	item.admin.Close()
}

func mustID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
