import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Project, ProjectInput } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class ProjectsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly items = signal<readonly Project[]>([]);
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(reset = true): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.loading()) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listProjects(workspaceId, {
        cursor: reset ? undefined : (this.nextCursor() ?? undefined),
        limit: 25,
      });
      this.items.update((items) => (reset ? page.items : [...items, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async create(input: ProjectInput): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const project = await this.api.createProject(workspaceId, input);
      this.items.update((items) => [project, ...items]);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }
}
