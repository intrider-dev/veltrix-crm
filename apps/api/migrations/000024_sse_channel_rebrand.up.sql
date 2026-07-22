-- Existing installations may still have the pre-rebrand trigger function
-- because applied migrations are immutable and 000001 is not replayed.
SET ROLE veltrix_owner;

CREATE OR REPLACE FUNCTION notifications.publish_sse_event()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
  PERFORM pg_notify(
    'veltrix_sse',
    NEW.workspace_id::text || ':' || NEW.id::text || ':' || NEW.event_type
  );
  RETURN NEW;
END
$$;

RESET ROLE;
