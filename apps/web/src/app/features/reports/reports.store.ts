import { computed, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { DashboardPreferences, PeriodReport } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class ReportsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly report = signal<PeriodReport | null>(null);
  readonly preferences = signal<DashboardPreferences | null>(null);
  readonly currency = signal('USD');
  readonly periodDays = signal(30);
  readonly loading = signal(false);
  readonly error = signal<unknown>(null);
  readonly maxStageValue = computed(() =>
    Math.max(1, ...(this.report()?.dealsByStage.map((stage) => stage.amountMinor) ?? [1])),
  );

  async load(initial = false): Promise<void> {
    const id = this.workspace.id();
    const active = this.workspace.active();
    if (!id || !active) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      if (initial) {
        const [preferences, dashboard] = await Promise.all([
          this.api.dashboardPreferences(id),
          this.api.dashboard(id),
        ]);
        this.preferences.set(preferences);
        this.periodDays.set(preferences.periodDays);
        this.currency.set(dashboard.currency);
      }
      const end = new Date();
      const start = new Date(end.getTime() - this.periodDays() * 86_400_000);
      this.report.set(await this.api.periodReport(id, start, end, active.timezone));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async setPeriod(days: number): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.periodDays.set(days);
    const current = this.preferences();
    try {
      this.preferences.set(
        await this.api.saveDashboardPreferences(
          id,
          {
            layout: current?.layout ?? { order: ['overview', 'stages', 'owners', 'activities'] },
            periodDays: days,
            timezone: current?.timezone,
          },
          current?.version,
        ),
      );
      await this.load();
    } catch (error) {
      this.error.set(error);
    }
  }
}
