import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { WorkspaceRoleDefinition, WorkspaceRoleInput } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class RolesStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly roles = signal<readonly WorkspaceRoleDefinition[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.roles.set(await this.api.listWorkspaceRoles(workspaceId));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async create(input: WorkspaceRoleInput): Promise<WorkspaceRoleDefinition> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace.required');
    this.saving.set(true);
    this.error.set(null);
    try {
      const created = await this.api.createWorkspaceRole(workspaceId, input);
      this.roles.update((roles) => [...roles, created]);
      return created;
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async update(
    role: WorkspaceRoleDefinition,
    input: WorkspaceRoleInput,
  ): Promise<WorkspaceRoleDefinition> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace.required');
    this.saving.set(true);
    this.error.set(null);
    try {
      const updated = await this.api.updateWorkspaceRole(workspaceId, role, input);
      this.roles.update((roles) => roles.map((item) => (item.id === updated.id ? updated : item)));
      return updated;
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async remove(role: WorkspaceRoleDefinition): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.error.set(null);
    try {
      await this.api.deleteWorkspaceRole(workspaceId, role);
      this.roles.update((roles) => roles.filter((item) => item.id !== role.id));
    } catch (error) {
      this.error.set(error);
    }
  }
}
