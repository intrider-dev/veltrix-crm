import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type {
  Activity,
  CreateActivity,
  Department,
  Project,
  ProjectAssignment,
  ProjectAssignmentInput,
  ProjectInput,
  WorkspaceMember,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class ProjectDetailsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly project = signal<Project | null>(null);
  readonly assignments = signal<readonly ProjectAssignment[]>([]);
  readonly assignmentVersion = signal(0);
  readonly activities = signal<readonly Activity[]>([]);
  readonly members = signal<readonly WorkspaceMember[]>([]);
  readonly departments = signal<readonly Department[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  readonly conflict = signal(false);

  async load(projectId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [project, assignments, activities, members, departments] = await Promise.all([
        this.api.getProject(workspaceId, projectId),
        this.api.listProjectAssignments(workspaceId, projectId),
        this.api.listActivities(workspaceId, 'project', projectId),
        this.api.listMembers(workspaceId),
        this.api.listDepartments(workspaceId),
      ]);
      this.project.set(project.body);
      this.assignments.set(assignments.items);
      this.assignmentVersion.set(assignments.version);
      this.activities.set(activities);
      this.members.set(members.filter((member) => member.status === 'active'));
      this.departments.set(departments);
      this.conflict.set(false);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async save(input: ProjectInput): Promise<void> {
    const workspaceId = this.workspace.id();
    const project = this.project();
    if (!workspaceId || !project) return;
    await this.mutate(async () => {
      const updated = await this.api.updateProject(workspaceId, project.id, project.version, input);
      this.project.set(updated.body);
    });
  }

  async remove(): Promise<void> {
    const workspaceId = this.workspace.id();
    const project = this.project();
    if (!workspaceId || !project) return;
    await this.mutate(() => this.api.deleteProject(workspaceId, project.id, project.version));
  }

  async addAssignment(input: ProjectAssignmentInput): Promise<void> {
    const existing = this.assignments().map((assignment) => ({
      kind: assignment.kind,
      subjectType: assignment.subjectType,
      subjectId: assignment.subjectId,
    }));
    if (
      existing.some(
        (assignment) =>
          assignment.kind === input.kind &&
          assignment.subjectType === input.subjectType &&
          assignment.subjectId === input.subjectId,
      )
    ) {
      return;
    }
    await this.replaceAssignments([...existing, input]);
  }

  async removeAssignment(assignmentId: string): Promise<void> {
    await this.replaceAssignments(
      this.assignments()
        .filter((assignment) => assignment.id !== assignmentId)
        .map((assignment) => ({
          kind: assignment.kind,
          subjectType: assignment.subjectType,
          subjectId: assignment.subjectId,
        })),
    );
  }

  async addTask(body: CreateActivity): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    await this.mutate(async () => {
      const activity = await this.api.createActivity(workspaceId, body);
      this.activities.update((items) => [activity, ...items]);
    });
  }

  async completeTask(activity: Activity): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    await this.mutate(async () => {
      const completed = await this.api.completeActivity(workspaceId, activity);
      this.activities.update((items) =>
        items.map((item) => (item.id === completed.id ? completed : item)),
      );
    });
  }

  private async replaceAssignments(assignments: readonly ProjectAssignmentInput[]): Promise<void> {
    const workspaceId = this.workspace.id();
    const project = this.project();
    if (!workspaceId || !project) return;
    await this.mutate(async () => {
      const updated = await this.api.replaceProjectAssignments(
        workspaceId,
        project.id,
        this.assignmentVersion(),
        assignments,
      );
      this.assignments.set(updated.items);
      this.assignmentVersion.set(updated.version);
      this.project.update((current) =>
        current ? { ...current, version: updated.version } : current,
      );
    });
  }

  private async mutate(operation: () => Promise<void>): Promise<void> {
    this.saving.set(true);
    this.error.set(null);
    this.conflict.set(false);
    try {
      await operation();
    } catch (error) {
      this.error.set(error);
      if (error instanceof ApiError && error.status === 412) this.conflict.set(true);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }
}
