SET ROLE veltrix_owner;

CREATE SCHEMA projects;

CREATE TABLE projects.projects (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
  description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 20000),
  status text NOT NULL DEFAULT 'planned'
    CHECK (status IN ('planned', 'active', 'on_hold', 'completed', 'archived')),
  visibility text NOT NULL DEFAULT 'workspace'
    CHECK (visibility IN ('workspace', 'restricted')),
  planned_start_date date,
  target_end_date date,
  owner_user_id uuid,
  created_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, owner_user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id),
  FOREIGN KEY (workspace_id, created_by)
    REFERENCES tenancy.memberships(workspace_id, user_id),
  CHECK (planned_start_date IS NULL OR target_end_date IS NULL OR planned_start_date <= target_end_date)
);

CREATE INDEX projects_active_list_idx
  ON projects.projects (workspace_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX projects_owner_status_idx
  ON projects.projects (workspace_id, owner_user_id, status, target_end_date, id)
  WHERE deleted_at IS NULL;

CREATE TABLE projects.project_assignments (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  project_id uuid NOT NULL,
  assignment_kind text NOT NULL CHECK (assignment_kind IN ('responsible', 'watcher')),
  user_id uuid,
  department_id uuid,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, project_id)
    REFERENCES projects.projects(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, department_id)
    REFERENCES tenancy.teams(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, created_by)
    REFERENCES tenancy.memberships(workspace_id, user_id),
  CHECK ((user_id IS NOT NULL)::integer + (department_id IS NOT NULL)::integer = 1)
);

CREATE UNIQUE INDEX project_assignment_user_unique_idx
  ON projects.project_assignments (workspace_id, project_id, assignment_kind, user_id)
  WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX project_assignment_department_unique_idx
  ON projects.project_assignments (workspace_id, project_id, assignment_kind, department_id)
  WHERE department_id IS NOT NULL;
CREATE INDEX project_assignment_user_access_idx
  ON projects.project_assignments (workspace_id, user_id, project_id, assignment_kind)
  WHERE user_id IS NOT NULL;
CREATE INDEX project_assignment_department_access_idx
  ON projects.project_assignments (workspace_id, department_id, project_id, assignment_kind)
  WHERE department_id IS NOT NULL;

ALTER TABLE activities.activities DROP CONSTRAINT activities_related_type_check;
ALTER TABLE activities.activities ADD CONSTRAINT activities_related_type_check
  CHECK (related_type IS NULL OR related_type IN ('contact', 'company', 'deal', 'project'));

ALTER TABLE files.attachments DROP CONSTRAINT attachments_entity_type_check;
ALTER TABLE files.attachments ADD CONSTRAINT attachments_entity_type_check
  CHECK (entity_type IN ('contact', 'company', 'deal', 'activity', 'project', 'import'));

ALTER TABLE projects.projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects.projects FORCE ROW LEVEL SECURITY;
ALTER TABLE projects.project_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects.project_assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_scope ON projects.projects
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY tenant_scope ON projects.project_assignments
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

GRANT USAGE ON SCHEMA projects TO veltrix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON projects.projects, projects.project_assignments TO veltrix_app;

RESET ROLE;
