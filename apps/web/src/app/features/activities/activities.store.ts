import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Activity, CreateActivity } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class ActivitiesStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly activities = signal<readonly Activity[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.activities.set(await this.api.listActivities(id));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }
  async create(body: CreateActivity): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    try {
      const activity = await this.api.createActivity(id, body);
      this.activities.update((items) => [activity, ...items]);
    } finally {
      this.saving.set(false);
    }
  }

  async complete(activity: Activity): Promise<void> {
    const id = this.workspace.id();
    if (!id || activity.status !== 'open' || activity.type !== 'task') return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const completed = await this.api.completeActivity(id, activity);
      this.activities.update((items) =>
        items.map((item) => (item.id === completed.id ? completed : item)),
      );
    } catch (error) {
      this.error.set(error);
    } finally {
      this.saving.set(false);
    }
  }

  setVersion(activityId: string, version: number): void {
    this.activities.update((items) =>
      items.map((item) => (item.id === activityId ? { ...item, version } : item)),
    );
  }
}
