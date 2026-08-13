-- Forward-only compatibility bridge for databases initialized before the
-- repository/product role rename. This must remain a new migration: changing
-- an already-applied historical migration would leave upgraded databases in a
-- different state from clean installations.
RESET ROLE;

DO $legacy_roles$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'velocity_owner') THEN
    IF EXISTS (
      SELECT 1 FROM pg_roles
      WHERE rolname IN ('veltrix_owner', 'veltrix_app', 'veltrix_dispatcher')
    ) THEN
      RAISE EXCEPTION 'legacy velocity roles and veltrix roles both exist; resolve the cluster-wide role collision before migration 28';
    END IF;
    ALTER ROLE velocity_owner RENAME TO veltrix_owner;
    ALTER ROLE velocity_app RENAME TO veltrix_app;
    ALTER ROLE velocity_dispatcher RENAME TO veltrix_dispatcher;
  END IF;

  IF current_setting('bootstrap.app_db_password', true) IS NOT NULL THEN
    EXECUTE format('ALTER ROLE veltrix_app PASSWORD %L', current_setting('bootstrap.app_db_password'));
    EXECUTE format('ALTER ROLE veltrix_dispatcher PASSWORD %L', current_setting('bootstrap.app_db_password'));
  END IF;
END
$legacy_roles$;
