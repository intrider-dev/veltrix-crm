SET ROLE veltrix_owner;

ALTER TABLE sales.pipeline_stages
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0);

ALTER TABLE sales.leads
  ADD COLUMN phone text CHECK (phone IS NULL OR char_length(phone) <= 40),
  ADD COLUMN email_normalized text,
  ADD COLUMN phone_normalized text,
  ADD COLUMN job_title text CHECK (job_title IS NULL OR char_length(job_title) <= 160),
  ADD COLUMN team_id uuid,
  ADD COLUMN deleted_by uuid,
  ADD CONSTRAINT leads_team_fk
    FOREIGN KEY (workspace_id, team_id) REFERENCES tenancy.teams(workspace_id, id),
  ADD CONSTRAINT leads_deleted_by_fk
    FOREIGN KEY (workspace_id, deleted_by) REFERENCES tenancy.memberships(workspace_id, user_id),
  ADD CONSTRAINT leads_converted_contact_fk
    FOREIGN KEY (workspace_id, converted_contact_id) REFERENCES customers.contacts(workspace_id, id),
  ADD CONSTRAINT leads_converted_company_fk
    FOREIGN KEY (workspace_id, converted_company_id) REFERENCES customers.companies(workspace_id, id),
  ADD CONSTRAINT leads_converted_deal_fk
    FOREIGN KEY (workspace_id, converted_deal_id) REFERENCES sales.deals(workspace_id, id);

ALTER TABLE sales.deals
  ADD COLUMN forecast_category text NOT NULL DEFAULT 'pipeline'
    CHECK (forecast_category IN ('pipeline', 'best_case', 'commit', 'closed')),
  ADD COLUMN won_at timestamptz,
  ADD COLUMN lost_at timestamptz,
  ADD COLUMN deleted_by uuid,
  ADD CONSTRAINT deals_deleted_by_fk
    FOREIGN KEY (workspace_id, deleted_by) REFERENCES tenancy.memberships(workspace_id, user_id);

UPDATE sales.deals
SET won_at = CASE WHEN status = 'won' THEN updated_at ELSE NULL END,
    lost_at = CASE WHEN status = 'lost' THEN updated_at ELSE NULL END,
    lost_reason = CASE WHEN status = 'lost' THEN lost_reason ELSE NULL END,
    forecast_category = CASE WHEN status IN ('won', 'lost') THEN 'closed' ELSE forecast_category END;

ALTER TABLE sales.deals
  ADD CONSTRAINT deals_outcome_shape CHECK (
    (status = 'open' AND won_at IS NULL AND lost_at IS NULL AND lost_reason IS NULL)
    OR (status = 'won' AND won_at IS NOT NULL AND lost_at IS NULL AND lost_reason IS NULL)
    OR (status = 'lost' AND won_at IS NULL AND lost_at IS NOT NULL)
  );

ALTER TABLE sales.deal_line_items
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0);

CREATE TABLE sales.deal_participants (
  workspace_id uuid NOT NULL,
  deal_id uuid NOT NULL,
  contact_id uuid NOT NULL,
  role text CHECK (role IS NULL OR char_length(role) <= 120),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, deal_id, contact_id),
  FOREIGN KEY (workspace_id, deal_id) REFERENCES sales.deals(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, contact_id) REFERENCES customers.contacts(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX leads_filter_idx
  ON sales.leads (workspace_id, status, owner_user_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX leads_trash_idx
  ON sales.leads (workspace_id, deleted_at DESC, id DESC)
  WHERE deleted_at IS NOT NULL;
CREATE INDEX leads_name_trgm_idx ON sales.leads USING gin (name gin_trgm_ops);
CREATE INDEX leads_email_trgm_idx ON sales.leads USING gin (email_normalized gin_trgm_ops);

CREATE INDEX deals_filter_idx
  ON sales.deals (workspace_id, status, owner_user_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX deals_trash_idx
  ON sales.deals (workspace_id, deleted_at DESC, id DESC)
  WHERE deleted_at IS NOT NULL;
CREATE INDEX deals_name_trgm_idx ON sales.deals USING gin (name gin_trgm_ops);
CREATE INDEX deal_line_items_order_idx
  ON sales.deal_line_items (workspace_id, deal_id, position, id);
CREATE INDEX deal_participants_contact_idx
  ON sales.deal_participants (workspace_id, contact_id, deal_id);

ALTER TABLE sales.deal_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.deal_participants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON sales.deal_participants
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON sales.deal_participants TO veltrix_app;

RESET ROLE;
