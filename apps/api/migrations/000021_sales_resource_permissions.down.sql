SET ROLE veltrix_owner;

DROP POLICY IF EXISTS workspace_roles_first_owner_select ON tenancy.workspace_roles;
DROP POLICY IF EXISTS workspace_roles_pending_invitee_select ON tenancy.workspace_roles;
DROP TRIGGER IF EXISTS workspace_sales_resource_permissions ON tenancy.workspaces;
DROP FUNCTION IF EXISTS tenancy.expand_sales_resource_permissions_trigger();
DROP FUNCTION IF EXISTS tenancy.expand_sales_resource_permissions(uuid);

DELETE FROM tenancy.role_permissions
WHERE permission IN (
  'leads.read', 'leads.create', 'leads.update', 'leads.delete',
  'deals.read', 'deals.create', 'deals.update', 'deals.delete',
  'lead_stages.manage', 'deal_stages.manage'
);

RESET ROLE;
