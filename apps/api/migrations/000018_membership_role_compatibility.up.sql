SET ROLE veltrix_owner;

CREATE OR REPLACE FUNCTION tenancy.resolve_membership_role_id()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
  IF NEW.role_id IS NULL OR (
    TG_OP = 'UPDATE'
    AND NEW.role IS DISTINCT FROM OLD.role
    AND NEW.role_id IS NOT DISTINCT FROM OLD.role_id
  ) THEN
    SELECT role.id INTO NEW.role_id
    FROM tenancy.workspace_roles role
    WHERE role.workspace_id = NEW.workspace_id
      AND role.role_key = NEW.role
      AND role.is_system;
  END IF;
  RETURN NEW;
END
$function$;
REVOKE ALL ON FUNCTION tenancy.resolve_membership_role_id() FROM PUBLIC;

CREATE TRIGGER memberships_resolve_legacy_role
  BEFORE INSERT OR UPDATE OF role ON tenancy.memberships
  FOR EACH ROW EXECUTE FUNCTION tenancy.resolve_membership_role_id();

RESET ROLE;
