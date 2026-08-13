CREATE EXTENSION IF NOT EXISTS pg_trgm;

DO $roles$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'veltrix_owner') THEN
    CREATE ROLE veltrix_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'veltrix_app') THEN
    CREATE ROLE veltrix_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'veltrix_dispatcher') THEN
    CREATE ROLE veltrix_dispatcher LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;

  IF current_setting('bootstrap.app_db_password', true) IS NOT NULL THEN
    EXECUTE format('ALTER ROLE veltrix_app PASSWORD %L', current_setting('bootstrap.app_db_password'));
    EXECUTE format('ALTER ROLE veltrix_dispatcher PASSWORD %L', current_setting('bootstrap.app_db_password'));
  END IF;

  -- veltrix_owner is NOLOGIN and is assumed only by the privileged migrator.
  -- PostgreSQL requires database-level CREATE before that role can create the
  -- application schemas on a freshly created database.
  EXECUTE format(
    'GRANT CONNECT, TEMPORARY, CREATE ON DATABASE %I TO veltrix_owner',
    current_database()
  );
END
$roles$;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;

SET ROLE veltrix_owner;

CREATE SCHEMA IF NOT EXISTS security AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS identity AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS tenancy AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS customers AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS sales AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS activities AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS automation AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS notifications AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS reporting AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS search AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS files AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS integrations AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS audit AUTHORIZATION veltrix_owner;
CREATE SCHEMA IF NOT EXISTS platform AUTHORIZATION veltrix_owner;

CREATE OR REPLACE FUNCTION security.current_workspace_id()
RETURNS uuid
LANGUAGE sql
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $$
  SELECT NULLIF(pg_catalog.current_setting('app.workspace_id', true), '')::uuid
$$;

CREATE OR REPLACE FUNCTION security.current_actor_id()
RETURNS uuid
LANGUAGE sql
STABLE
PARALLEL SAFE
SET search_path = pg_catalog
AS $$
  SELECT NULLIF(pg_catalog.current_setting('app.actor_id', true), '')::uuid
$$;

REVOKE ALL ON FUNCTION security.current_workspace_id() FROM PUBLIC;
REVOKE ALL ON FUNCTION security.current_actor_id() FROM PUBLIC;

CREATE TABLE identity.users (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  email_normalized text NOT NULL UNIQUE,
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 160),
  password_hash text NOT NULL,
  preferred_locale text NOT NULL DEFAULT 'en' CHECK (preferred_locale IN ('en', 'ru')),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  session_version bigint NOT NULL DEFAULT 1 CHECK (session_version > 0),
  failed_login_count integer NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  locked_until timestamptz,
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identity.sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash) = 32),
  session_version bigint NOT NULL,
  user_agent text NOT NULL DEFAULT '' CHECK (char_length(user_agent) <= 512),
  ip_address inet,
  expires_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_active_token_idx ON identity.sessions (token_hash, expires_at)
  WHERE revoked_at IS NULL;
CREATE INDEX sessions_user_active_idx ON identity.sessions (user_id, created_at DESC)
  WHERE revoked_at IS NULL;

CREATE TABLE identity.password_reset_tokens (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identity.mfa_configurations (
  user_id uuid PRIMARY KEY REFERENCES identity.users(id) ON DELETE CASCADE,
  secret_ciphertext bytea NOT NULL,
  secret_nonce bytea NOT NULL,
  key_id text NOT NULL,
  enabled_at timestamptz,
  last_accepted_step bigint,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identity.recovery_codes (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
  code_hash bytea NOT NULL CHECK (octet_length(code_hash) = 32),
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, code_hash)
);

CREATE TABLE tenancy.workspaces (
  id uuid PRIMARY KEY,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
  slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,62}$'),
  default_locale text NOT NULL DEFAULT 'en' CHECK (default_locale IN ('en', 'ru')),
  timezone text NOT NULL DEFAULT 'UTC' CHECK (char_length(timezone) BETWEEN 1 AND 80),
  default_currency char(3) NOT NULL DEFAULT 'USD' CHECK (default_currency ~ '^[A-Z]{3}$'),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenancy.memberships (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('owner', 'admin', 'manager', 'sales', 'viewer')),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  locale_override text CHECK (locale_override IN ('en', 'ru')),
  timezone_override text CHECK (timezone_override IS NULL OR char_length(timezone_override) BETWEEN 1 AND 80),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, user_id)
);

CREATE INDEX memberships_user_idx ON tenancy.memberships (user_id, status, workspace_id);

CREATE TABLE tenancy.teams (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, name)
);

CREATE TABLE tenancy.team_memberships (
  workspace_id uuid NOT NULL,
  team_id uuid NOT NULL,
  membership_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, team_id, membership_id),
  FOREIGN KEY (workspace_id, team_id) REFERENCES tenancy.teams(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, membership_id) REFERENCES tenancy.memberships(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE tenancy.invitations (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  email_normalized text NOT NULL,
  role text NOT NULL CHECK (role IN ('admin', 'manager', 'sales', 'viewer')),
  token_hash bytea NOT NULL CHECK (octet_length(token_hash) = 32),
  invited_by uuid NOT NULL,
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, token_hash),
  FOREIGN KEY (workspace_id, invited_by) REFERENCES tenancy.memberships(workspace_id, id)
);

CREATE TABLE customers.companies (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
  domain text CHECK (domain IS NULL OR char_length(domain) <= 253),
  domain_normalized text,
  industry text CHECK (industry IS NULL OR char_length(industry) <= 120),
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
  owner_user_id uuid,
  team_id uuid,
  address jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(address::text) <= 8192),
  custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(custom_fields::text) <= 65536),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  deleted_at timestamptz,
  deleted_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, owner_user_id) REFERENCES tenancy.memberships(workspace_id, user_id),
  FOREIGN KEY (workspace_id, team_id) REFERENCES tenancy.teams(workspace_id, id)
);

CREATE INDEX companies_active_list_idx ON customers.companies (workspace_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX companies_name_trgm_idx ON customers.companies USING gin (name gin_trgm_ops);
CREATE UNIQUE INDEX companies_active_domain_idx ON customers.companies (workspace_id, domain_normalized)
  WHERE deleted_at IS NULL AND domain_normalized IS NOT NULL;

CREATE TABLE customers.contacts (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  first_name text NOT NULL CHECK (char_length(first_name) BETWEEN 1 AND 120),
  last_name text NOT NULL CHECK (char_length(last_name) BETWEEN 1 AND 120),
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 241),
  email text CHECK (email IS NULL OR char_length(email) <= 254),
  email_normalized text,
  phone text CHECK (phone IS NULL OR char_length(phone) <= 40),
  phone_normalized text,
  job_title text CHECK (job_title IS NULL OR char_length(job_title) <= 160),
  company_id uuid,
  owner_user_id uuid,
  team_id uuid,
  source text CHECK (source IS NULL OR char_length(source) <= 80),
  status text NOT NULL DEFAULT 'active' CHECK (char_length(status) BETWEEN 1 AND 40),
  address jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(address::text) <= 8192),
  custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(custom_fields::text) <= 65536),
  last_contacted_at timestamptz,
  next_activity_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  deleted_at timestamptz,
  deleted_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, company_id) REFERENCES customers.companies(workspace_id, id),
  FOREIGN KEY (workspace_id, owner_user_id) REFERENCES tenancy.memberships(workspace_id, user_id),
  FOREIGN KEY (workspace_id, team_id) REFERENCES tenancy.teams(workspace_id, id)
);

CREATE INDEX contacts_active_list_idx ON customers.contacts (workspace_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX contacts_active_status_idx ON customers.contacts (workspace_id, status, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX contacts_active_owner_idx ON customers.contacts (workspace_id, owner_user_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX contacts_name_trgm_idx ON customers.contacts USING gin (display_name gin_trgm_ops);
CREATE INDEX contacts_email_idx ON customers.contacts (workspace_id, email_normalized)
  WHERE deleted_at IS NULL AND email_normalized IS NOT NULL;
CREATE INDEX contacts_phone_idx ON customers.contacts (workspace_id, phone_normalized)
  WHERE deleted_at IS NULL AND phone_normalized IS NOT NULL;

CREATE TABLE customers.tags (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  color text NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9a-fA-F]{6}$'),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, name)
);

CREATE TABLE customers.contact_tags (
  workspace_id uuid NOT NULL,
  contact_id uuid NOT NULL,
  tag_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, contact_id, tag_id),
  FOREIGN KEY (workspace_id, contact_id) REFERENCES customers.contacts(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, tag_id) REFERENCES customers.tags(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE customers.custom_field_definitions (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company', 'lead', 'deal')),
  field_key text NOT NULL CHECK (field_key ~ '^[a-z][a-z0-9_]{1,62}$'),
  label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 120),
  value_type text NOT NULL CHECK (value_type IN ('text', 'number', 'money', 'date', 'boolean', 'single_select', 'multi_select', 'user_reference')),
  validation jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(validation::text) <= 16384),
  options jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (octet_length(options::text) <= 32768),
  schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, entity_type, field_key)
);

CREATE TABLE customers.custom_field_values (
  workspace_id uuid NOT NULL,
  definition_id uuid NOT NULL,
  entity_type text NOT NULL,
  entity_id uuid NOT NULL,
  value jsonb NOT NULL CHECK (octet_length(value::text) <= 65536),
  schema_version integer NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, definition_id, entity_id),
  FOREIGN KEY (workspace_id, definition_id) REFERENCES customers.custom_field_definitions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE customers.saved_views (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  owner_user_id uuid NOT NULL,
  entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company', 'lead', 'deal', 'activity')),
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  definition jsonb NOT NULL CHECK (octet_length(definition::text) <= 65536),
  is_shared boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, owner_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE customers.import_sessions (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  actor_user_id uuid NOT NULL,
  entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company')),
  status text NOT NULL CHECK (status IN ('preview', 'queued', 'running', 'completed', 'failed')),
  mapping jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(mapping::text) <= 65536),
  total_rows integer NOT NULL DEFAULT 0,
  processed_rows integer NOT NULL DEFAULT 0,
  error_rows integer NOT NULL DEFAULT 0,
  error_report_attachment_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, actor_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE sales.pipelines (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  is_default boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, name)
);

CREATE UNIQUE INDEX pipelines_one_default_idx ON sales.pipelines (workspace_id) WHERE is_default;

CREATE TABLE sales.pipeline_stages (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  pipeline_id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  probability smallint NOT NULL CHECK (probability BETWEEN 0 AND 100),
  forecast_category text NOT NULL DEFAULT 'pipeline' CHECK (forecast_category IN ('pipeline', 'best_case', 'commit', 'closed')),
  position integer NOT NULL CHECK (position >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, pipeline_id, position),
  FOREIGN KEY (workspace_id, pipeline_id) REFERENCES sales.pipelines(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE sales.leads (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
  email text,
  company_name text,
  source text,
  status text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'qualified', 'converted', 'disqualified')),
  owner_user_id uuid,
  converted_contact_id uuid,
  converted_company_id uuid,
  converted_deal_id uuid,
  custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(custom_fields::text) <= 65536),
  version bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, owner_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX leads_active_list_idx ON sales.leads (workspace_id, updated_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE TABLE sales.deals (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  pipeline_id uuid NOT NULL,
  stage_id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
  contact_id uuid,
  company_id uuid,
  owner_user_id uuid,
  amount_minor bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
  currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  expected_close_date date,
  position integer NOT NULL DEFAULT 0 CHECK (position >= 0),
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'won', 'lost')),
  lost_reason text CHECK (lost_reason IS NULL OR char_length(lost_reason) <= 500),
  custom_fields jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(custom_fields::text) <= 65536),
  version bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, pipeline_id) REFERENCES sales.pipelines(workspace_id, id),
  FOREIGN KEY (workspace_id, stage_id) REFERENCES sales.pipeline_stages(workspace_id, id),
  FOREIGN KEY (workspace_id, contact_id) REFERENCES customers.contacts(workspace_id, id),
  FOREIGN KEY (workspace_id, company_id) REFERENCES customers.companies(workspace_id, id),
  FOREIGN KEY (workspace_id, owner_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX deals_stage_board_idx ON sales.deals (workspace_id, pipeline_id, stage_id, position, id)
  WHERE deleted_at IS NULL AND status = 'open';
CREATE INDEX deals_owner_close_idx ON sales.deals (workspace_id, owner_user_id, expected_close_date, id)
  WHERE deleted_at IS NULL;

CREATE TABLE sales.deal_stage_history (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  deal_id uuid NOT NULL,
  from_stage_id uuid,
  to_stage_id uuid NOT NULL,
  changed_by uuid NOT NULL,
  changed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, deal_id) REFERENCES sales.deals(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, from_stage_id) REFERENCES sales.pipeline_stages(workspace_id, id),
  FOREIGN KEY (workspace_id, to_stage_id) REFERENCES sales.pipeline_stages(workspace_id, id),
  FOREIGN KEY (workspace_id, changed_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX deal_history_timeline_idx ON sales.deal_stage_history (workspace_id, deal_id, changed_at DESC, id DESC);

CREATE TABLE sales.deal_line_items (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  deal_id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
  quantity numeric(14,4) NOT NULL DEFAULT 1 CHECK (quantity > 0),
  unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
  currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  position integer NOT NULL DEFAULT 0,
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, deal_id) REFERENCES sales.deals(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE activities.activities (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  activity_type text NOT NULL CHECK (activity_type IN ('task', 'call', 'meeting', 'note')),
  title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 240),
  body text CHECK (body IS NULL OR char_length(body) <= 20000),
  related_type text CHECK (related_type IS NULL OR related_type IN ('contact', 'company', 'deal')),
  related_id uuid,
  assignee_user_id uuid,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'completed', 'cancelled')),
  priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high')),
  due_at timestamptz,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  recurrence_rule text CHECK (recurrence_rule IS NULL OR char_length(recurrence_rule) <= 500),
  completed_at timestamptz,
  created_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, assignee_user_id) REFERENCES tenancy.memberships(workspace_id, user_id),
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX activities_timeline_idx ON activities.activities (workspace_id, related_type, related_id, occurred_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX activities_due_idx ON activities.activities (workspace_id, assignee_user_id, due_at, id)
  WHERE deleted_at IS NULL AND status = 'open' AND activity_type = 'task';

CREATE TABLE activities.comments (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  activity_id uuid NOT NULL,
  author_user_id uuid NOT NULL,
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 10000),
  mentioned_user_ids uuid[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, activity_id) REFERENCES activities.activities(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, author_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE activities.reminders (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  activity_id uuid NOT NULL,
  recipient_user_id uuid NOT NULL,
  remind_at timestamptz NOT NULL,
  delivered_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, activity_id) REFERENCES activities.activities(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, recipient_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX reminders_pending_idx ON activities.reminders (workspace_id, remind_at, id) WHERE delivered_at IS NULL;

CREATE TABLE audit.events (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  actor_user_id uuid,
  action text NOT NULL CHECK (char_length(action) BETWEEN 1 AND 120),
  entity_type text NOT NULL CHECK (char_length(entity_type) BETWEEN 1 AND 80),
  entity_id uuid NOT NULL,
  request_id text NOT NULL CHECK (char_length(request_id) BETWEEN 1 AND 80),
  summary jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(summary::text) <= 32768),
  ip_address inet,
  user_agent text NOT NULL DEFAULT '' CHECK (char_length(user_agent) <= 512),
  occurred_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id)
);

CREATE INDEX audit_timeline_idx ON audit.events (workspace_id, occurred_at DESC, id DESC);
CREATE INDEX audit_entity_idx ON audit.events (workspace_id, entity_type, entity_id, occurred_at DESC);

CREATE TABLE platform.idempotency_keys (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  key text NOT NULL CHECK (char_length(key) BETWEEN 16 AND 128),
  actor_id uuid NOT NULL,
  operation text NOT NULL CHECK (char_length(operation) BETWEEN 1 AND 120),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  response_status integer,
  response_body jsonb CHECK (response_body IS NULL OR octet_length(response_body::text) <= 262144),
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, key)
);

CREATE TABLE platform.outbox_events (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  event_type text NOT NULL CHECK (char_length(event_type) BETWEEN 1 AND 120),
  schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
  aggregate_type text NOT NULL CHECK (char_length(aggregate_type) BETWEEN 1 AND 80),
  aggregate_id uuid NOT NULL,
  causation_id uuid,
  correlation_id uuid NOT NULL,
  payload jsonb NOT NULL CHECK (octet_length(payload::text) <= 262144),
  available_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  last_error_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id)
);

CREATE INDEX outbox_claim_idx ON platform.outbox_events (available_at, created_at, id)
  WHERE published_at IS NULL;

CREATE TABLE platform.jobs (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  kind text NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 120),
  schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
  idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
  payload jsonb NOT NULL CHECK (octet_length(payload::text) <= 262144),
  state text NOT NULL DEFAULT 'ready' CHECK (state IN ('ready', 'running', 'completed', 'dead')),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts integer NOT NULL DEFAULT 8 CHECK (max_attempts BETWEEN 1 AND 100),
  available_at timestamptz NOT NULL DEFAULT now(),
  locked_at timestamptz,
  locked_until timestamptz,
  worker_id text,
  fencing_token bigint NOT NULL DEFAULT 0,
  last_error_code text,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, kind, idempotency_key)
);

CREATE INDEX jobs_claim_idx ON platform.jobs (available_at, created_at, id)
  WHERE state = 'ready';
CREATE INDEX jobs_lease_idx ON platform.jobs (locked_until, id)
  WHERE state = 'running';

-- Administrative seed ledger. It deliberately has no runtime grants or RLS:
-- only the migrator/owner path may create deterministic demo and benchmark data.
CREATE TABLE platform.seed_runs (
  profile text NOT NULL CHECK (profile IN ('demo', 'small', 'benchmark')),
  seed_version integer NOT NULL CHECK (seed_version > 0),
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  dataset_hash text NOT NULL CHECK (char_length(dataset_hash) = 64),
  counts jsonb NOT NULL CHECK (octet_length(counts::text) <= 4096),
  completed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile, seed_version),
  UNIQUE (workspace_id)
);

CREATE TABLE search.documents (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company', 'lead', 'deal', 'note')),
  entity_id uuid NOT NULL,
  title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 500),
  subtitle text CHECK (subtitle IS NULL OR char_length(subtitle) <= 500),
  searchable_text text NOT NULL CHECK (char_length(searchable_text) <= 20000),
  search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', searchable_text)) STORED,
  rank_boost real NOT NULL DEFAULT 1 CHECK (rank_boost > 0),
  version bigint NOT NULL DEFAULT 1,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, entity_type, entity_id)
);

CREATE INDEX search_vector_idx ON search.documents USING gin (search_vector);
CREATE INDEX search_text_trgm_idx ON search.documents USING gin (searchable_text gin_trgm_ops);

CREATE TABLE notifications.notifications (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  recipient_user_id uuid NOT NULL,
  message_key text NOT NULL CHECK (char_length(message_key) BETWEEN 1 AND 160),
  message_params jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(message_params::text) <= 32768),
  template_version integer NOT NULL DEFAULT 1,
  entity_type text,
  entity_id uuid,
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, recipient_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX notifications_user_unread_idx ON notifications.notifications (workspace_id, recipient_user_id, created_at DESC, id DESC)
  WHERE read_at IS NULL;

CREATE TABLE notifications.sse_events (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  event_type text NOT NULL CHECK (char_length(event_type) BETWEEN 1 AND 120),
  data jsonb NOT NULL CHECK (octet_length(data::text) <= 65536),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (workspace_id, id)
);

CREATE INDEX sse_replay_idx ON notifications.sse_events (workspace_id, created_at, id);

CREATE FUNCTION notifications.publish_sse_event()
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

CREATE TRIGGER publish_sse_event_after_commit
AFTER INSERT ON notifications.sse_events
FOR EACH ROW EXECUTE FUNCTION notifications.publish_sse_event();

CREATE TABLE automation.rules (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
  trigger_type text NOT NULL CHECK (trigger_type IN ('record_created', 'record_updated', 'deal_stage_changed', 'deal_won', 'deal_lost', 'task_overdue', 'scheduled')),
  conditions jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(conditions::text) <= 65536),
  actions jsonb NOT NULL CHECK (octet_length(actions::text) <= 65536),
  enabled boolean NOT NULL DEFAULT false,
  rate_limit_per_hour integer NOT NULL DEFAULT 1000 CHECK (rate_limit_per_hour BETWEEN 1 AND 100000),
  version bigint NOT NULL DEFAULT 1,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE automation.executions (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  rule_id uuid NOT NULL,
  event_id uuid NOT NULL,
  correlation_id uuid NOT NULL,
  depth integer NOT NULL DEFAULT 0 CHECK (depth BETWEEN 0 AND 16),
  state text NOT NULL CHECK (state IN ('preview', 'queued', 'running', 'completed', 'failed', 'dead')),
  attempts integer NOT NULL DEFAULT 0,
  result jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(result::text) <= 65536),
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, rule_id, event_id),
  FOREIGN KEY (workspace_id, rule_id) REFERENCES automation.rules(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE integrations.api_keys (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  key_prefix text NOT NULL CHECK (char_length(key_prefix) BETWEEN 8 AND 32),
  secret_hash bytea NOT NULL CHECK (octet_length(secret_hash) = 32),
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  scopes text[] NOT NULL DEFAULT '{}',
  created_by uuid NOT NULL,
  last_used_at timestamptz,
  expires_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, key_prefix),
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE integrations.webhook_subscriptions (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
  event_types text[] NOT NULL,
  secret_ciphertext bytea NOT NULL,
  secret_nonce bytea NOT NULL,
  key_id text NOT NULL,
  previous_secret_ciphertext bytea,
  previous_secret_expires_at timestamptz,
  enabled boolean NOT NULL DEFAULT true,
  version bigint NOT NULL DEFAULT 1,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE integrations.webhook_deliveries (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  subscription_id uuid NOT NULL,
  event_id uuid NOT NULL,
  status text NOT NULL CHECK (status IN ('queued', 'delivering', 'succeeded', 'failed', 'dead')),
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  response_status integer,
  response_summary text CHECK (response_summary IS NULL OR char_length(response_summary) <= 2048),
  delivered_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, subscription_id, event_id),
  FOREIGN KEY (workspace_id, subscription_id) REFERENCES integrations.webhook_subscriptions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE files.attachments (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company', 'deal', 'activity', 'import')),
  entity_id uuid NOT NULL,
  storage_backend text NOT NULL DEFAULT 'local' CHECK (storage_backend IN ('local', 's3')),
  storage_key text NOT NULL CHECK (storage_key ~ '^[a-zA-Z0-9/_-]{1,500}$'),
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 255),
  media_type text NOT NULL CHECK (char_length(media_type) BETWEEN 1 AND 120),
  size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 0 AND 104857600),
  checksum_sha256 bytea NOT NULL CHECK (octet_length(checksum_sha256) = 32),
  scan_state text NOT NULL DEFAULT 'pending' CHECK (scan_state IN ('pending', 'clean', 'rejected', 'unavailable')),
  uploaded_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, storage_key),
  FOREIGN KEY (workspace_id, uploaded_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE reporting.dashboard_preferences (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  user_id uuid NOT NULL,
  preferences jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(preferences::text) <= 32768),
  version bigint NOT NULL DEFAULT 1,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, user_id),
  FOREIGN KEY (workspace_id, user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE TABLE reporting.dashboard_summaries (
  workspace_id uuid PRIMARY KEY REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  open_pipeline_minor bigint NOT NULL DEFAULT 0,
  weighted_forecast_minor bigint NOT NULL DEFAULT 0,
  won_count bigint NOT NULL DEFAULT 0,
  lost_count bigint NOT NULL DEFAULT 0,
  overdue_tasks bigint NOT NULL DEFAULT 0,
  deals_by_stage jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (octet_length(deals_by_stage::text) <= 65536),
  computed_at timestamptz NOT NULL DEFAULT now(),
  source_version bigint NOT NULL DEFAULT 1
);

ALTER TABLE tenancy.workspaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.workspaces FORCE ROW LEVEL SECURITY;
ALTER TABLE tenancy.memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.memberships FORCE ROW LEVEL SECURITY;

CREATE POLICY workspace_read ON tenancy.workspaces
  FOR SELECT TO veltrix_app
  USING (
    id = security.current_workspace_id()
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships m
      WHERE m.workspace_id = tenancy.workspaces.id
        AND m.user_id = security.current_actor_id()
        AND m.status = 'active'
    )
  );
CREATE POLICY workspace_insert ON tenancy.workspaces
  FOR INSERT TO veltrix_app WITH CHECK (true);
CREATE POLICY workspace_modify ON tenancy.workspaces
  FOR UPDATE TO veltrix_app
  USING (id = security.current_workspace_id())
  WITH CHECK (id = security.current_workspace_id());
CREATE POLICY workspace_delete ON tenancy.workspaces
  FOR DELETE TO veltrix_app
  USING (id = security.current_workspace_id());

CREATE POLICY membership_read ON tenancy.memberships
  FOR SELECT TO veltrix_app
  USING (user_id = security.current_actor_id() OR workspace_id = security.current_workspace_id());
CREATE POLICY membership_write ON tenancy.memberships
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

DO $rls$
DECLARE
  target regclass;
BEGIN
  FOREACH target IN ARRAY ARRAY[
    'tenancy.teams'::regclass,
    'tenancy.team_memberships'::regclass,
    'tenancy.invitations'::regclass,
    'customers.companies'::regclass,
    'customers.contacts'::regclass,
    'customers.tags'::regclass,
    'customers.contact_tags'::regclass,
    'customers.custom_field_definitions'::regclass,
    'customers.custom_field_values'::regclass,
    'customers.saved_views'::regclass,
    'customers.import_sessions'::regclass,
    'sales.pipelines'::regclass,
    'sales.pipeline_stages'::regclass,
    'sales.leads'::regclass,
    'sales.deals'::regclass,
    'sales.deal_stage_history'::regclass,
    'sales.deal_line_items'::regclass,
    'activities.activities'::regclass,
    'activities.comments'::regclass,
    'activities.reminders'::regclass,
    'audit.events'::regclass,
    'platform.idempotency_keys'::regclass,
    'platform.outbox_events'::regclass,
    'platform.jobs'::regclass,
    'search.documents'::regclass,
    'notifications.notifications'::regclass,
    'notifications.sse_events'::regclass,
    'automation.rules'::regclass,
    'automation.executions'::regclass,
    'integrations.api_keys'::regclass,
    'integrations.webhook_subscriptions'::regclass,
    'integrations.webhook_deliveries'::regclass,
    'files.attachments'::regclass,
    'reporting.dashboard_preferences'::regclass,
    'reporting.dashboard_summaries'::regclass
  ]
  LOOP
    EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', target);
    EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', target);
    EXECUTE format(
      'CREATE POLICY tenant_scope ON %s FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id()) WITH CHECK (workspace_id = security.current_workspace_id())',
      target
    );
  END LOOP;
END
$rls$;

CREATE POLICY dispatcher_outbox ON platform.outbox_events
  FOR ALL TO veltrix_dispatcher USING (true) WITH CHECK (true);
CREATE POLICY dispatcher_jobs ON platform.jobs
  FOR ALL TO veltrix_dispatcher USING (true) WITH CHECK (true);
CREATE POLICY dispatcher_sse ON notifications.sse_events
  FOR SELECT TO veltrix_dispatcher USING (true);

RESET ROLE;

GRANT USAGE ON SCHEMA security TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.current_workspace_id() TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.current_actor_id() TO veltrix_app;

GRANT USAGE ON SCHEMA identity, tenancy, customers, sales, activities, automation,
  notifications, reporting, search, files, integrations, audit, platform TO veltrix_app;
GRANT SELECT, INSERT, UPDATE ON identity.users, identity.sessions,
  identity.password_reset_tokens, identity.mfa_configurations, identity.recovery_codes TO veltrix_app;
GRANT DELETE ON identity.sessions, identity.password_reset_tokens, identity.recovery_codes TO veltrix_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON tenancy.workspaces, tenancy.memberships,
  tenancy.teams, tenancy.team_memberships, tenancy.invitations,
  customers.companies, customers.contacts, customers.tags, customers.contact_tags,
  customers.custom_field_definitions, customers.custom_field_values, customers.saved_views,
  customers.import_sessions, sales.pipelines, sales.pipeline_stages, sales.leads, sales.deals,
  sales.deal_stage_history, sales.deal_line_items, activities.activities, activities.comments,
  activities.reminders, platform.idempotency_keys, platform.outbox_events, platform.jobs,
  search.documents, notifications.notifications, notifications.sse_events, automation.rules,
  automation.executions, integrations.api_keys, integrations.webhook_subscriptions,
  integrations.webhook_deliveries, files.attachments, reporting.dashboard_preferences,
  reporting.dashboard_summaries TO veltrix_app;
GRANT SELECT, INSERT ON audit.events TO veltrix_app;

GRANT USAGE ON SCHEMA platform TO veltrix_dispatcher;
GRANT SELECT, INSERT, UPDATE ON platform.outbox_events, platform.jobs TO veltrix_dispatcher;
GRANT USAGE ON SCHEMA notifications TO veltrix_dispatcher;
GRANT SELECT ON notifications.sse_events TO veltrix_dispatcher;
