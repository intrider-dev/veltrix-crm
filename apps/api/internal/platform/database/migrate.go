package database

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/migrations"
)

func Migrate(ctx context.Context, adminURL, appPassword string) error {
	pool, err := Open(ctx, adminURL, 1, "veltrix-crm-migrator")
	if err != nil {
		return err
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	// Multiple application instances and parallel integration packages may
	// start against the same empty database. A session-level lock serializes
	// both ledger creation and the check/apply/record sequence.
	const migrationLockID int64 = 861714290347
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID) }()

	if _, err := conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS public.schema_migrations (
  version bigint PRIMARY KEY,
  name text NOT NULL,
  checksum text,
  applied_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE public.schema_migrations ADD COLUMN IF NOT EXISTS checksum text
`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return fmt.Errorf("migration %q has no numeric prefix", entry.Name())
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return fmt.Errorf("migration %q: %w", entry.Name(), err)
		}
		contents, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		checksum := migrationChecksum(contents)
		var recordedName string
		var recordedChecksum *string
		err = conn.QueryRow(ctx, `
SELECT name, checksum FROM public.schema_migrations WHERE version=$1
`, version).Scan(&recordedName, &recordedChecksum)
		if err == nil {
			if recordedName != entry.Name() {
				return fmt.Errorf("migration %d name changed: recorded %q, embedded %q", version, recordedName, entry.Name())
			}
			if recordedChecksum == nil {
				if _, err := conn.Exec(ctx, `
UPDATE public.schema_migrations SET checksum=$2 WHERE version=$1 AND checksum IS NULL
`, version, checksum); err != nil {
					return fmt.Errorf("backfill migration %d checksum: %w", version, err)
				}
				continue
			}
			if *recordedChecksum != checksum {
				return fmt.Errorf("migration %d checksum changed: recorded %s, embedded %s", version, *recordedChecksum, checksum)
			}
			continue
		}
		if err != pgx.ErrNoRows {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if err := applyMigration(ctx, conn, version, entry.Name(), string(contents), checksum, appPassword); err != nil {
			return err
		}
	}
	return nil
}

func migrationChecksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return fmt.Sprintf("sha256:%x", sum)
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, version int64, name, sql, checksum, appPassword string) error {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('bootstrap.app_db_password', $1, true)", appPassword); err != nil {
		return fmt.Errorf("configure migration %d: %w", version, err)
	}
	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", version, name, err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO public.schema_migrations(version, name, checksum) VALUES($1, $2, $3)
`, version, name, checksum); err != nil {
		return fmt.Errorf("record migration %d: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %d: %w", version, err)
	}
	return nil
}
