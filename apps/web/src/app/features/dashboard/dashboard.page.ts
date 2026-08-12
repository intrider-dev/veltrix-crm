import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

import type { Activity, Dashboard } from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { IconComponent, type IconName } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { DashboardStore } from './dashboard.store';

interface ActivityGroup {
  readonly type: Activity['type'];
  readonly count: number;
  readonly width: number;
  readonly icon: IconName;
}

interface ChartPoint {
  readonly x: number;
  readonly y: number;
  readonly stageName: string;
  readonly amountMinor: number;
}

@Component({
  selector: 'app-dashboard-page',
  imports: [ErrorPanelComponent, IconComponent, MatButtonModule, RouterLink],
  providers: [DashboardStore],
  templateUrl: './dashboard.page.html',
  styleUrl: './dashboard.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardPage implements OnInit {
  readonly store = inject(DashboardStore);
  readonly i18n = inject(I18nService);
  private readonly auth = inject(AuthStore);

  readonly firstName = computed(() => {
    const displayName = this.auth.user()?.displayName.trim() ?? '';
    return displayName.split(/\s+/)[0] || this.i18n.t('dashboard.dashboard.userFallback');
  });

  readonly openTasks = computed(() =>
    this.store
      .activities()
      .filter((activity) => activity.type === 'task' && activity.status === 'open')
      .sort((left, right) => {
        if (!left.dueAt) return 1;
        if (!right.dueAt) return -1;
        return Date.parse(left.dueAt) - Date.parse(right.dueAt);
      }),
  );

  readonly activityGroups = computed<readonly ActivityGroup[]>(() => {
    const activities = this.store.activities();
    if (activities.length === 0) return [];
    const types: readonly Activity['type'][] = ['task', 'meeting', 'call', 'note'];
    const counts = types.map((type) => ({
      type,
      count: activities.filter((activity) => activity.type === type).length,
    }));
    const maximum = Math.max(...counts.map((group) => group.count), 1);
    return counts
      .filter((group) => group.count > 0)
      .map((group) => ({
        ...group,
        width: Math.max(8, (group.count / maximum) * 100),
        icon: this.activityIcon(group.type),
      }));
  });

  private readonly stageColors = ['#4f78ff', '#a855f7', '#ff9d3d', '#6558d9', '#3dd39f', '#ec5d8f'];

  ngOnInit(): void {
    void this.store.load();
  }

  totalDeals(dashboard: Dashboard): number {
    return dashboard.dealsByStage?.reduce((total, stage) => total + stage.count, 0) ?? 0;
  }

  stagePercent(count: number, dashboard: Dashboard): number {
    const total = this.totalDeals(dashboard);
    return total === 0 ? 0 : Math.round((count / total) * 100);
  }

  stageColor(index: number): string {
    return this.stageColors[index % this.stageColors.length] ?? this.stageColors[0];
  }

  donutGradient(dashboard: Dashboard): string {
    const stages = dashboard.dealsByStage ?? [];
    const total = Math.max(this.totalDeals(dashboard), 1);
    let cursor = 0;
    const segments = stages.map((stage, index) => {
      const start = cursor;
      cursor += (stage.count / total) * 100;
      return `${this.stageColor(index)} ${start}% ${cursor}%`;
    });
    return segments.length > 0 ? `conic-gradient(${segments.join(', ')})` : '#1b2634';
  }

  chartPoints(dashboard: Dashboard): readonly ChartPoint[] {
    const stages = dashboard.dealsByStage ?? [];
    const maximum = Math.max(...stages.map((stage) => stage.amountMinor), 1);
    const usableWidth = 690;
    return stages.map((stage, index) => ({
      x: stages.length === 1 ? 386 : 42 + (index / (stages.length - 1)) * usableWidth,
      y: 225 - (stage.amountMinor / maximum) * 175,
      stageName: stage.stageName,
      amountMinor: stage.amountMinor,
    }));
  }

  chartLine(dashboard: Dashboard): string {
    return this.chartPoints(dashboard)
      .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`)
      .join(' ');
  }

  chartArea(dashboard: Dashboard): string {
    const points = this.chartPoints(dashboard);
    if (points.length === 0) return '';
    const line = points.map((point) => `L ${point.x} ${point.y}`).join(' ');
    const first = points[0];
    const last = points.at(-1) ?? first;
    return `M ${first.x} 235 ${line} L ${last.x} 235 Z`;
  }

  isOverdue(activity: Activity): boolean {
    return Boolean(activity.dueAt && Date.parse(activity.dueAt) < Date.now());
  }

  activityIcon(type: Activity['type']): IconName {
    if (type === 'task') return 'check';
    if (type === 'meeting') return 'calendar';
    if (type === 'call') return 'phone';
    return 'file';
  }

  activityLabel(type: Activity['type']): string {
    return this.i18n.t(`activities.activity.${type}`);
  }

  priorityLabel(priority: Activity['priority']): string {
    return this.i18n.t(`activities.priority.${priority ?? 'normal'}`);
  }
}
