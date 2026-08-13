-- name: GetWorkspaceLocalizationSettings :one
SELECT default_locale, supported_locales, version, updated_at
FROM tenancy.workspaces
WHERE id = sqlc.arg(workspace_id);

-- name: UpdateWorkspaceLocales :one
UPDATE tenancy.workspaces
SET default_locale = sqlc.arg(default_locale),
    supported_locales = sqlc.arg(supported_locales),
    version = version + 1,
    updated_at = now()
WHERE id = sqlc.arg(workspace_id)
  AND version = sqlc.arg(expected_version)
RETURNING default_locale, supported_locales, version, updated_at;

-- name: GetContentResource :one
SELECT workspace_id, namespace, resource_key, source_locale, source_text,
       description, placeholders, version, created_by, updated_by,
       created_at, updated_at
FROM localization.content_resources
WHERE workspace_id = sqlc.arg(workspace_id)
  AND namespace = sqlc.arg(namespace)
  AND resource_key = sqlc.arg(resource_key);

-- name: CreateContentResource :one
INSERT INTO localization.content_resources (
  workspace_id, namespace, resource_key, source_locale, source_text,
  description, placeholders, created_by, updated_by
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(namespace), sqlc.arg(resource_key),
  sqlc.arg(source_locale), sqlc.arg(source_text), sqlc.arg(description),
  sqlc.arg(placeholders), sqlc.arg(actor_id), sqlc.arg(actor_id)
)
ON CONFLICT (workspace_id, namespace, resource_key) DO NOTHING
RETURNING workspace_id, namespace, resource_key, source_locale, source_text,
          description, placeholders, version, created_by, updated_by,
          created_at, updated_at;

-- name: UpdateContentResourceSource :one
UPDATE localization.content_resources
SET source_locale = sqlc.arg(source_locale),
    source_text = sqlc.arg(source_text),
    description = sqlc.arg(description),
    placeholders = sqlc.arg(placeholders),
    updated_by = sqlc.arg(actor_id),
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND namespace = sqlc.arg(namespace)
  AND resource_key = sqlc.arg(resource_key)
RETURNING workspace_id, namespace, resource_key, source_locale, source_text,
          description, placeholders, version, created_by, updated_by,
          created_at, updated_at;

-- name: MarkContentTranslationsDraft :exec
UPDATE localization.content_translations
SET status = 'draft',
    updated_by = sqlc.arg(actor_id),
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND namespace = sqlc.arg(namespace)
  AND resource_key = sqlc.arg(resource_key)
  AND status <> 'draft';

-- name: DeleteContentResources :exec
DELETE FROM localization.content_resources
WHERE workspace_id = sqlc.arg(workspace_id)
  AND namespace = sqlc.arg(namespace)
  AND resource_key = ANY(sqlc.arg(resource_keys)::text[]);

-- name: CreateContentTranslation :one
INSERT INTO localization.content_translations (
  workspace_id, namespace, resource_key, locale, translated_text, status,
  updated_by
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(namespace), sqlc.arg(resource_key),
  sqlc.arg(locale), sqlc.arg(translated_text), sqlc.arg(status),
  sqlc.arg(actor_id)
)
ON CONFLICT (workspace_id, namespace, resource_key, locale) DO NOTHING
RETURNING workspace_id, namespace, resource_key, locale, translated_text,
          status, version, updated_by, created_at, updated_at;

-- name: UpdateContentTranslation :one
UPDATE localization.content_translations
SET translated_text = sqlc.arg(translated_text),
    status = sqlc.arg(status),
    updated_by = sqlc.arg(actor_id),
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND namespace = sqlc.arg(namespace)
  AND resource_key = sqlc.arg(resource_key)
  AND locale = sqlc.arg(locale)
  AND version = sqlc.arg(expected_version)
RETURNING workspace_id, namespace, resource_key, locale, translated_text,
          status, version, updated_by, created_at, updated_at;

-- name: ListContentTranslations :many
SELECT r.namespace, r.resource_key, r.source_locale, r.source_text,
       r.description, r.placeholders, r.version AS resource_version,
       t.locale, t.translated_text, t.status, t.version AS translation_version,
       COALESCE(t.updated_at, r.updated_at) AS updated_at
FROM localization.content_resources r
LEFT JOIN localization.content_translations t
  ON t.workspace_id = r.workspace_id
 AND t.namespace = r.namespace
 AND t.resource_key = r.resource_key
 AND t.locale = sqlc.arg(locale)
WHERE r.workspace_id = sqlc.arg(workspace_id)
  AND (sqlc.arg(namespace_filter)::text = '' OR r.namespace = sqlc.arg(namespace_filter))
  AND (
    sqlc.arg(search_query)::text = ''
    OR r.resource_key ILIKE '%' || sqlc.arg(search_query) || '%'
    OR r.source_text ILIKE '%' || sqlc.arg(search_query) || '%'
    OR COALESCE(t.translated_text, '') ILIKE '%' || sqlc.arg(search_query) || '%'
  )
  AND (
    sqlc.arg(status_filter)::text = ''
    OR (sqlc.arg(status_filter) = 'missing' AND t.locale IS NULL)
    OR (sqlc.arg(status_filter) IN ('draft', 'published') AND t.status = sqlc.arg(status_filter))
  )
  AND (r.namespace, r.resource_key) >
      (sqlc.arg(cursor_namespace)::text, sqlc.arg(cursor_resource_key)::text)
ORDER BY r.namespace, r.resource_key
LIMIT sqlc.arg(page_limit);

-- name: ContentTranslationCoverage :many
SELECT r.namespace,
       count(*)::bigint AS total,
       count(t.resource_key) FILTER (WHERE t.status = 'published')::bigint AS published,
       count(t.resource_key) FILTER (WHERE t.status = 'draft')::bigint AS draft,
       (count(*) - count(t.resource_key))::bigint AS missing
FROM localization.content_resources r
LEFT JOIN localization.content_translations t
  ON t.workspace_id = r.workspace_id
 AND t.namespace = r.namespace
 AND t.resource_key = r.resource_key
 AND t.locale = sqlc.arg(locale)
WHERE r.workspace_id = sqlc.arg(workspace_id)
GROUP BY r.namespace
ORDER BY r.namespace;

-- name: ResolvePublishedContent :one
SELECT r.source_text,
       COALESCE(requested.translated_text, fallback.translated_text, r.source_text) AS resolved_text,
       (CASE
         WHEN requested.translated_text IS NOT NULL THEN sqlc.arg(requested_locale)::text
         WHEN fallback.translated_text IS NOT NULL THEN sqlc.arg(fallback_locale)::text
         ELSE r.source_locale
       END)::text AS resolved_locale
FROM localization.content_resources r
LEFT JOIN localization.content_translations requested
  ON requested.workspace_id = r.workspace_id
 AND requested.namespace = r.namespace
 AND requested.resource_key = r.resource_key
 AND requested.locale = sqlc.arg(requested_locale)
 AND requested.status = 'published'
LEFT JOIN localization.content_translations fallback
  ON fallback.workspace_id = r.workspace_id
 AND fallback.namespace = r.namespace
 AND fallback.resource_key = r.resource_key
 AND fallback.locale = sqlc.arg(fallback_locale)
 AND fallback.status = 'published'
WHERE r.workspace_id = sqlc.arg(workspace_id)
  AND r.namespace = sqlc.arg(namespace)
  AND r.resource_key = sqlc.arg(resource_key);

-- name: ResolvePublishedContents :many
SELECT r.resource_key,
       COALESCE(requested.translated_text, fallback.translated_text, r.source_text) AS resolved_text,
       (CASE
         WHEN requested.translated_text IS NOT NULL THEN sqlc.arg(requested_locale)::text
         WHEN fallback.translated_text IS NOT NULL THEN sqlc.arg(fallback_locale)::text
         ELSE r.source_locale
       END)::text AS resolved_locale
FROM localization.content_resources r
LEFT JOIN localization.content_translations requested
  ON requested.workspace_id = r.workspace_id
 AND requested.namespace = r.namespace
 AND requested.resource_key = r.resource_key
 AND requested.locale = sqlc.arg(requested_locale)
 AND requested.status = 'published'
LEFT JOIN localization.content_translations fallback
  ON fallback.workspace_id = r.workspace_id
 AND fallback.namespace = r.namespace
 AND fallback.resource_key = r.resource_key
 AND fallback.locale = sqlc.arg(fallback_locale)
 AND fallback.status = 'published'
WHERE r.workspace_id = sqlc.arg(workspace_id)
  AND r.namespace = sqlc.arg(namespace)
  AND r.resource_key = ANY(sqlc.arg(resource_keys)::text[])
ORDER BY r.resource_key;
