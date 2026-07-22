SET ROLE veltrix_owner;

CREATE OR REPLACE FUNCTION tenancy.expand_sales_resource_permissions(target_workspace_id uuid)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  INSERT INTO tenancy.role_permissions (workspace_id, role_id, permission)
  SELECT existing.workspace_id, existing.role_id, mapped.permission
  FROM tenancy.role_permissions existing
  CROSS JOIN LATERAL (
    VALUES
      (CASE existing.permission WHEN 'records.read' THEN 'leads.read' END),
      (CASE existing.permission WHEN 'records.create' THEN 'leads.create' END),
      (CASE existing.permission WHEN 'records.update' THEN 'leads.update' END),
      (CASE existing.permission WHEN 'records.delete' THEN 'leads.delete' END),
      (CASE existing.permission WHEN 'records.read' THEN 'deals.read' END),
      (CASE existing.permission WHEN 'records.create' THEN 'deals.create' END),
      (CASE existing.permission WHEN 'records.update' THEN 'deals.update' END),
      (CASE existing.permission WHEN 'records.delete' THEN 'deals.delete' END),
      (CASE existing.permission WHEN 'settings.write' THEN 'lead_stages.manage' END),
      (CASE existing.permission WHEN 'settings.write' THEN 'deal_stages.manage' END)
  ) AS mapped(permission)
  WHERE existing.workspace_id = target_workspace_id
    AND mapped.permission IS NOT NULL
  ON CONFLICT DO NOTHING
$function$;
REVOKE ALL ON FUNCTION tenancy.expand_sales_resource_permissions(uuid) FROM PUBLIC;

SELECT tenancy.expand_sales_resource_permissions(workspace.id)
FROM tenancy.workspaces workspace;

CREATE OR REPLACE FUNCTION tenancy.expand_sales_resource_permissions_trigger()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
  PERFORM tenancy.expand_sales_resource_permissions(NEW.id);
  RETURN NEW;
END
$function$;
REVOKE ALL ON FUNCTION tenancy.expand_sales_resource_permissions_trigger() FROM PUBLIC;

CREATE TRIGGER workspace_sales_resource_permissions
  AFTER INSERT ON tenancy.workspaces
  FOR EACH ROW EXECUTE FUNCTION tenancy.expand_sales_resource_permissions_trigger();

-- Break the secure workspace bootstrap cycle: the first membership needs the
-- system owner role, while the normal role policy requires a membership.
CREATE POLICY workspace_roles_first_owner_select ON tenancy.workspace_roles
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND role_key = 'owner'
    AND is_system
    AND security.current_actor_id() IS NOT NULL
    AND NOT EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = workspace_roles.workspace_id
    )
  );

CREATE POLICY workspace_roles_pending_invitee_select ON tenancy.workspace_roles
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND is_system
    AND EXISTS (
      SELECT 1
      FROM tenancy.invitations invitation
      JOIN identity.users invited_user
        ON invited_user.email_normalized = invitation.email_normalized
      WHERE invitation.workspace_id = workspace_roles.workspace_id
        AND invitation.role = workspace_roles.role_key
        AND invitation.accepted_at IS NULL
        AND invitation.expires_at > statement_timestamp()
        AND invited_user.id = security.current_actor_id()
    )
  );

RESET ROLE;
