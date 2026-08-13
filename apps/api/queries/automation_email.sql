-- name: ResolveAutomationEmailRecipient :one
WITH target AS (
  SELECT contact.email AS direct_email, contact.owner_user_id AS user_id
  FROM customers.contacts AS contact
  WHERE sqlc.arg(target_type)::text = 'contact'
    AND contact.workspace_id = sqlc.arg(workspace_id)
    AND contact.id = sqlc.arg(target_id)
    AND contact.deleted_at IS NULL
  UNION ALL
  SELECT NULL::text, company.owner_user_id
  FROM customers.companies AS company
  WHERE sqlc.arg(target_type)::text = 'company'
    AND company.workspace_id = sqlc.arg(workspace_id)
    AND company.id = sqlc.arg(target_id)
    AND company.deleted_at IS NULL
  UNION ALL
  SELECT lead.email, lead.owner_user_id
  FROM sales.leads AS lead
  WHERE sqlc.arg(target_type)::text = 'lead'
    AND lead.workspace_id = sqlc.arg(workspace_id)
    AND lead.id = sqlc.arg(target_id)
    AND lead.deleted_at IS NULL
  UNION ALL
  SELECT contact.email, deal.owner_user_id
  FROM sales.deals AS deal
  LEFT JOIN customers.contacts AS contact
    ON contact.workspace_id = deal.workspace_id
   AND contact.id = deal.contact_id
   AND contact.deleted_at IS NULL
  WHERE sqlc.arg(target_type)::text = 'deal'
    AND deal.workspace_id = sqlc.arg(workspace_id)
    AND deal.id = sqlc.arg(target_id)
    AND deal.deleted_at IS NULL
  UNION ALL
  SELECT NULL::text, activity.assignee_user_id
  FROM activities.activities AS activity
  WHERE sqlc.arg(target_type)::text = 'activity'
    AND activity.workspace_id = sqlc.arg(workspace_id)
    AND activity.id = sqlc.arg(target_id)
    AND activity.deleted_at IS NULL
)
SELECT CASE
         WHEN sqlc.arg(recipient_field)::text = 'email' THEN target.direct_email
         WHEN sqlc.arg(recipient_field)::text = 'owner_email' THEN recipient.email
       END::text AS recipient,
       CASE
         WHEN sqlc.arg(recipient_field)::text = 'owner_email'
           THEN COALESCE(membership.locale_override, recipient.preferred_locale, workspace.default_locale)
         ELSE workspace.default_locale
       END::text AS recipient_locale,
       workspace.default_locale, workspace.name AS workspace_name
FROM target
JOIN tenancy.workspaces AS workspace
  ON workspace.id = sqlc.arg(workspace_id)
LEFT JOIN tenancy.memberships AS membership
  ON membership.workspace_id = workspace.id
 AND membership.user_id = target.user_id
 AND membership.status = 'active'
LEFT JOIN identity.users AS recipient
  ON recipient.id = membership.user_id
 AND recipient.status = 'active'
WHERE (sqlc.arg(recipient_field)::text = 'email' AND target.direct_email IS NOT NULL)
   OR (sqlc.arg(recipient_field)::text = 'owner_email' AND recipient.email IS NOT NULL)
LIMIT 1;
