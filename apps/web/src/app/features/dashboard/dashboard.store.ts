import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Activity, Dashboard } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class DashboardStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly dashboard = signal<Dashboard | null>(null);
  readonly activities = signal<readonly Activity[]>([]);
  readonly loading = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [dashboard, activities] = await Promise.all([
        this.api.dashboard(workspaceId),
        this.api.listActivities(workspaceId),
      ]);
      this.dashboard.set(dashboard);
      this.activities.set(activities.slice(0, 8));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }
}
