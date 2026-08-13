SET ROLE veltrix_owner;

DROP TABLE IF EXISTS localization.content_translations;
DROP TABLE IF EXISTS localization.content_resources;

ALTER TABLE tenancy.memberships
  DROP CONSTRAINT IF EXISTS memberships_locale_override_check,
  ADD CONSTRAINT memberships_locale_override_check
    CHECK (locale_override IN ('en', 'ru'));

ALTER TABLE tenancy.workspaces
  DROP CONSTRAINT IF EXISTS workspaces_default_locale_check,
  DROP COLUMN IF EXISTS supported_locales,
  ADD CONSTRAINT workspaces_default_locale_check
    CHECK (default_locale IN ('en', 'ru'));

ALTER TABLE identity.users
  DROP CONSTRAINT IF EXISTS users_preferred_locale_check,
  ADD CONSTRAINT users_preferred_locale_check
    CHECK (preferred_locale IN ('en', 'ru'));

DROP FUNCTION IF EXISTS localization.valid_locale_list(text[], text);
DROP FUNCTION IF EXISTS localization.valid_locale(text);
DROP SCHEMA IF EXISTS localization;

RESET ROLE;
