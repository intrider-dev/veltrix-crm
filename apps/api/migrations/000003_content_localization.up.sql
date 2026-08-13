SET ROLE veltrix_owner;

CREATE SCHEMA IF NOT EXISTS localization AUTHORIZATION veltrix_owner;

CREATE OR REPLACE FUNCTION localization.valid_locale(value text)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $$
  SELECT value IS NOT NULL
    AND char_length(value) BETWEEN 2 AND 35
    AND value ~ '^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$'
$$;

REVOKE ALL ON FUNCTION localization.valid_locale(text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION localization.valid_locale_list(values_to_check text[], default_value text)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
SET search_path = pg_catalog, localization
AS $$
  SELECT cardinality(values_to_check) BETWEEN 1 AND 20
    AND default_value = ANY(values_to_check)
    AND NOT EXISTS (
      SELECT 1
      FROM unnest(values_to_check) AS value
      WHERE NOT localization.valid_locale(value)
    )
    AND cardinality(values_to_check) = (
      SELECT count(DISTINCT lower(value)) FROM unnest(values_to_check) AS value
    )
$$;

REVOKE ALL ON FUNCTION localization.valid_locale_list(text[], text) FROM PUBLIC;

ALTER TABLE identity.users
  DROP CONSTRAINT users_preferred_locale_check,
  ADD CONSTRAINT users_preferred_locale_check
    CHECK (localization.valid_locale(preferred_locale));

ALTER TABLE tenancy.workspaces
  DROP CONSTRAINT workspaces_default_locale_check,
  ADD COLUMN supported_locales text[] NOT NULL DEFAULT ARRAY['en', 'ru']::text[],
  ADD CONSTRAINT workspaces_default_locale_check
    CHECK (localization.valid_locale_list(supported_locales, default_locale));

ALTER TABLE tenancy.memberships
  DROP CONSTRAINT memberships_locale_override_check,
  ADD CONSTRAINT memberships_locale_override_check
    CHECK (locale_override IS NULL OR localization.valid_locale(locale_override));

CREATE TABLE localization.content_resources (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  namespace text NOT NULL CHECK (
    char_length(namespace) BETWEEN 1 AND 64
    AND namespace ~ '^[a-z][a-z0-9_.-]*$'
  ),
  resource_key text NOT NULL CHECK (
    char_length(resource_key) BETWEEN 1 AND 160
    AND resource_key ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]*$'
  ),
  source_locale text NOT NULL CHECK (localization.valid_locale(source_locale)),
  source_text text NOT NULL CHECK (char_length(source_text) BETWEEN 1 AND 8192),
  description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
  placeholders text[] NOT NULL DEFAULT '{}'::text[] CHECK (cardinality(placeholders) <= 32),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_by uuid NOT NULL REFERENCES identity.users(id),
  updated_by uuid NOT NULL REFERENCES identity.users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, namespace, resource_key)
);

CREATE TABLE localization.content_translations (
  workspace_id uuid NOT NULL,
  namespace text NOT NULL,
  resource_key text NOT NULL,
  locale text NOT NULL CHECK (localization.valid_locale(locale)),
  translated_text text NOT NULL CHECK (char_length(translated_text) BETWEEN 1 AND 8192),
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_by uuid NOT NULL REFERENCES identity.users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, namespace, resource_key, locale),
  FOREIGN KEY (workspace_id, namespace, resource_key)
    REFERENCES localization.content_resources(workspace_id, namespace, resource_key)
    ON DELETE CASCADE
);

CREATE INDEX content_resources_list_idx
  ON localization.content_resources (workspace_id, namespace, resource_key);

CREATE INDEX content_translations_workflow_idx
  ON localization.content_translations
    (workspace_id, locale, status, namespace, resource_key);

ALTER TABLE localization.content_resources ENABLE ROW LEVEL SECURITY;
ALTER TABLE localization.content_resources FORCE ROW LEVEL SECURITY;
ALTER TABLE localization.content_translations ENABLE ROW LEVEL SECURITY;
ALTER TABLE localization.content_translations FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_scope ON localization.content_resources
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

CREATE POLICY tenant_scope ON localization.content_translations
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

RESET ROLE;

GRANT USAGE ON SCHEMA localization TO veltrix_app;
GRANT EXECUTE ON FUNCTION localization.valid_locale(text) TO veltrix_app;
GRANT EXECUTE ON FUNCTION localization.valid_locale_list(text[], text) TO veltrix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON
  localization.content_resources,
  localization.content_translations
TO veltrix_app;
