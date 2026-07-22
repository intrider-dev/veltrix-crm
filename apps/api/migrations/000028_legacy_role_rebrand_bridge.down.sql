-- PostgreSQL roles are cluster-wide and may own objects in other databases.
-- An automatic reverse rename would be unsafe, so rollback is intentionally a
-- no-op. The application schema itself is unchanged by this compatibility step.
SELECT 1;
