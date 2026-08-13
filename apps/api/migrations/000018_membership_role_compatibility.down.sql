SET ROLE veltrix_owner;
DROP TRIGGER IF EXISTS memberships_resolve_legacy_role ON tenancy.memberships;
DROP FUNCTION IF EXISTS tenancy.resolve_membership_role_id();
RESET ROLE;
