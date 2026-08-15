import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ReportsStore } from './reports.store';

@Component({
  selector: 'app-reports-page',
  imports: [ErrorPanelComponent, MatButtonModule],
  providers: [ReportsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.reports') }}</h1>
          <p>{{ i18n.t('reports.subtitle') }}</p>
        </div>
        @if (permissions.allows('reports.read')) {
          <div
            class="segmented segmented-control"
            role="group"
            [attr.aria-label]="i18n.t('reports.period')"
          >
            @for (days of periods; track days) {
              <button
                mat-button
                type="button"
                [class.active]="store.periodDays() === days"
                (click)="store.setPeriod(days)"
              >
                {{ i18n.t(periodKey(days)) }}
              </button>
            }
          </div>
        }
      </header>
      @if (!permissions.allows('reports.read')) {
        <div class="error-panel" role="alert">{{ i18n.t('reports.permission') }}</div>
      } @else {
        @if (store.error()) {
          <app-error-panel [error]="store.error()" (retry)="store.load()" />
        }
        @if (store.loading() && !store.report()) {
          <section class="metric-grid">
            @for (item of skeletons; track item) {
              <div class="panel metric-card skeleton"></div>
            }
          </section>
        } @else if (store.report(); as report) {
          <section class="metric-grid" [attr.aria-label]="i18n.t('reports.overview')">
            <article class="panel metric-card metric-card--accent metric-card--brand">
              <span>{{ i18n.t('reports.wonValue') }}</span
              ><strong>{{ i18n.money(report.overview.wonValueMinor, store.currency()) }}</strong>
            </article>
            <article class="panel metric-card metric-card--accent metric-card--signal">
              <span>{{ i18n.t('reports.wonLost') }}</span
              ><strong>{{ report.overview.wonCount }} / {{ report.overview.lostCount }}</strong>
            </article>
            <article class="panel metric-card">
              <span>{{ i18n.t('reports.conversion') }}</span
              ><strong>{{ percent(report.overview.conversionRate) }}</strong>
            </article>
            <article class="panel metric-card">
              <span>{{ i18n.t('reports.activities') }}</span
              ><strong>{{ report.overview.activityCount }}</strong>
            </article>
          </section>
          <section class="report-grid">
            <article class="panel report-card">
              <header>
                <h2>{{ i18n.t('reports.dealsByStage') }}</h2>
              </header>
              @for (stage of report.dealsByStage; track stage.stageId) {
                <div class="bar-row">
                  <div>
                    <strong>{{ stage.stageName }}</strong
                    ><span>{{ stage.dealCount }}</span>
                  </div>
                  <div class="bar-track">
                    <span
                      [style.width.%]="(stage.amountMinor / store.maxStageValue()) * 100"
                    ></span>
                  </div>
                  <small>{{ i18n.money(stage.amountMinor, store.currency()) }}</small>
                </div>
              } @empty {
                <div class="empty-state compact">{{ i18n.t('reports.empty') }}</div>
              }
            </article>
            <article class="panel report-card">
              <header>
                <h2>{{ i18n.t('reports.dealsByOwner') }}</h2>
              </header>
              <div class="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>{{ i18n.t('common.field.owner') }}</th>
                      <th>{{ i18n.t('reports.deals') }}</th>
                      <th>{{ i18n.t('reports.won') }}</th>
                      <th>{{ i18n.t('reports.value') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    @for (owner of report.dealsByOwner; track owner.ownerId || owner.ownerName) {
                      <tr>
                        <td>{{ owner.ownerName }}</td>
                        <td>{{ owner.dealCount }}</td>
                        <td>{{ owner.wonCount }}</td>
                        <td>{{ i18n.money(owner.amountMinor, store.currency()) }}</td>
                      </tr>
                    }
                  </tbody>
                </table>
              </div>
            </article>
            <article class="panel report-card full">
              <header>
                <h2>{{ i18n.t('reports.activityTrend') }}</h2>
              </header>
              <div
                class="activity-bars"
                role="img"
                [attr.aria-label]="i18n.t('reports.activityChart')"
              >
                @for (day of report.activities; track day.date) {
                  <div>
                    <span
                      [style.height.%]="activityHeight(day.count, report.overview.activityCount)"
                    ></span
                    ><time [attr.datetime]="day.date">{{
                      i18n.date(day.date, { month: 'short', day: 'numeric' })
                    }}</time>
                  </div>
                }
              </div>
            </article>
          </section>
        }
      }
    </div>
  `,
  styles: `
    .segmented {
      align-self: center;
    }
    .segmented .active {
      color: var(--text);
      background: var(--surface-raised);
    }
    .metric-grid {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 0.875rem;
    }
    .metric-card {
      display: grid;
      min-height: 7.5rem;
      align-content: center;
      gap: 0.65rem;
      padding: 1.25rem;
      border: 0;
      box-shadow: var(--shadow-sm);
    }
    .metric-card--brand {
      color: white;
      background: var(--brand);
    }
    .metric-card--signal {
      color: var(--on-signal);
      background: var(--signal);
    }
    .metric-card--accent {
      position: relative;
      isolation: isolate;
      overflow: hidden;
    }
    .metric-card--accent > * {
      position: relative;
      z-index: 1;
    }
    .metric-card--accent::before,
    .metric-card--accent::after {
      position: absolute;
      z-index: 0;
      content: '';
      pointer-events: none;
    }
    .metric-card--accent::before {
      right: -3rem;
      bottom: -5rem;
      width: 8rem;
      height: 8rem;
      border: 1px solid color-mix(in srgb, currentcolor 27%, transparent);
      border-radius: 44% 56% 52% 48%;
      box-shadow:
        0 0 0 1rem color-mix(in srgb, currentcolor 7%, transparent),
        0 0 0 2rem color-mix(in srgb, currentcolor 4%, transparent);
      opacity: 0.8;
      transform: rotate(-24deg);
    }
    .metric-card--accent::after {
      right: 0.5rem;
      bottom: -2.65rem;
      width: 4.5rem;
      height: 4.5rem;
      border-radius: 50%;
      background: radial-gradient(
        circle at 35% 30%,
        color-mix(in srgb, currentcolor 30%, transparent),
        color-mix(in srgb, currentcolor 8%, transparent) 45%,
        transparent 68%
      );
      opacity: 0.85;
    }
    .metric-card--accent span {
      color: inherit;
      opacity: 0.78;
    }
    .metric-card span {
      color: var(--text-muted);
      font-size: 0.76rem;
      font-weight: 620;
    }
    .metric-card strong {
      font-size: 1.75rem;
      letter-spacing: -0.045em;
    }
    .report-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 0.875rem;
    }
    .report-card > header {
      padding: 0.85rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .report-card h2 {
      margin: 0;
      font-size: 0.95rem;
    }
    .report-card.full {
      grid-column: 1 / -1;
    }
    .bar-row {
      display: grid;
      grid-template-columns: minmax(8rem, 1fr) 2fr auto;
      align-items: center;
      gap: 0.75rem;
      padding: 0.75rem 1rem;
    }
    .bar-row > div:first-child {
      display: flex;
      justify-content: space-between;
      gap: 0.5rem;
      font-size: 0.78rem;
    }
    .bar-row small {
      color: var(--text-muted);
    }
    .bar-track {
      height: 0.5rem;
      overflow: hidden;
      border-radius: var(--radius-pill);
      background: var(--surface-subtle);
    }
    .bar-track span {
      display: block;
      height: 100%;
      border-radius: inherit;
      background: linear-gradient(90deg, var(--brand), var(--signal));
    }
    .table-scroll {
      overflow-x: auto;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.78rem;
    }
    th,
    td {
      padding: 0.7rem 1rem;
      border-bottom: 1px solid var(--border);
      text-align: left;
      white-space: nowrap;
    }
    th {
      color: var(--text-muted);
      font-weight: 500;
    }
    .activity-bars {
      display: flex;
      align-items: end;
      gap: 0.3rem;
      height: 12rem;
      padding: 1rem;
      overflow-x: auto;
    }
    .activity-bars > div {
      display: grid;
      min-width: 1.5rem;
      height: 100%;
      grid-template-rows: 1fr auto;
      align-items: end;
      gap: 0.35rem;
    }
    .activity-bars span {
      display: block;
      min-height: 0.15rem;
      border-radius: 0.25rem 0.25rem 0 0;
      background: var(--brand);
    }
    .activity-bars time {
      color: var(--text-faint);
      font-size: 0.6rem;
      writing-mode: vertical-rl;
    }
    .empty-state.compact {
      min-height: 8rem;
    }
    @media (max-width: 800px) {
      .metric-grid {
        grid-template-columns: 1fr 1fr;
      }
      .report-grid {
        grid-template-columns: 1fr;
      }
      .report-card.full {
        grid-column: auto;
      }
    }

    :host {
      --workspace-surface: var(--color-surface, var(--surface-raised));
      --workspace-subtle: var(--color-surface-subtle, var(--surface-subtle));
      --workspace-hover: var(--color-surface-hover, var(--surface-selected));
      --workspace-border: var(--color-border, var(--border));
      --workspace-anchor: var(--color-anchor, var(--brand));
    }
    .page-header {
      align-items: flex-end;
      margin-bottom: 0.25rem;
    }
    .page-header p {
      max-width: 48rem;
    }
    .segmented {
      flex-wrap: wrap;
      padding: 0.2rem;
      border-radius: var(--radius-control, 0.625rem);
      background: var(--workspace-subtle);
    }
    .segmented .active {
      color: var(--workspace-anchor);
      background: var(--workspace-surface);
      box-shadow: 0 1px 2px rgb(18 36 29 / 10%);
    }
    .metric-grid,
    .report-grid {
      gap: 0.75rem;
    }
    .metric-card {
      min-width: 0;
      min-height: 8rem;
      padding: 1.25rem;
      border-radius: var(--radius-panel, 0.875rem);
      box-shadow: none;
    }
    .metric-card:not(.metric-card--accent) {
      border-color: var(--workspace-border);
      background: var(--workspace-surface);
    }
    .metric-card strong {
      overflow-wrap: anywhere;
      font-size: clamp(1.5rem, 3vw, 2rem);
      line-height: 1.15;
    }
    .report-card {
      overflow: hidden;
      padding: 0;
      border-radius: var(--radius-panel, 0.875rem);
      background: var(--workspace-surface);
    }
    .report-card > header {
      min-height: 3.5rem;
      display: flex;
      align-items: center;
      padding: 0.875rem 1rem;
      border-color: var(--workspace-border);
      background: var(--workspace-subtle);
    }
    .bar-row {
      min-height: 4rem;
      padding: 0.75rem 1rem;
      border-bottom: 1px solid color-mix(in srgb, var(--workspace-border) 72%, transparent);
    }
    .bar-row:last-child {
      border-bottom: 0;
    }
    .bar-track {
      background: var(--workspace-subtle);
    }
    .bar-track span {
      background: linear-gradient(90deg, var(--workspace-anchor), var(--signal));
    }
    th,
    td {
      min-height: 2.75rem;
      border-color: color-mix(in srgb, var(--workspace-border) 72%, transparent);
    }
    th {
      background: var(--workspace-subtle);
      font-weight: 600;
    }
    tbody tr:hover {
      background: var(--workspace-hover);
    }
    .activity-bars {
      gap: 0.375rem;
      min-height: 14rem;
      padding: 1.25rem;
    }
    .activity-bars > div {
      min-width: 1.75rem;
    }
    .activity-bars span {
      background: var(--workspace-anchor);
    }
    @media (max-width: 800px) {
      .page-header {
        align-items: stretch;
      }
      .page-header .segmented {
        align-self: stretch;
      }
      .page-header .segmented button {
        flex: 1;
      }
      .bar-row {
        grid-template-columns: minmax(8rem, 1fr) minmax(8rem, 2fr) auto;
      }
    }
    @media (max-width: 560px) {
      .metric-grid {
        grid-template-columns: 1fr;
      }
      .bar-row {
        grid-template-columns: 1fr auto;
      }
      .bar-track {
        grid-column: 1 / -1;
        grid-row: 2;
      }
      .activity-bars {
        padding-inline: 0.75rem;
      }
    }
    @media (forced-colors: active) {
      .segmented .active,
      .metric-card--accent {
        border: 1px solid CanvasText;
      }
      .metric-card--accent::before,
      .metric-card--accent::after {
        display: none;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReportsPage implements OnInit {
  readonly store = inject(ReportsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly periods = [7, 30, 90] as const;
  readonly skeletons = [1, 2, 3, 4] as const;
  ngOnInit(): void {
    if (this.permissions.allows('reports.read')) void this.store.load(true);
  }
  periodKey(days: number): 'reports.period.7' | 'reports.period.30' | 'reports.period.90' {
    return `reports.period.${days}` as
      'reports.period.7' | 'reports.period.30' | 'reports.period.90';
  }
  percent(value: number): string {
    return new Intl.NumberFormat(this.i18n.locale(), {
      style: 'percent',
      maximumFractionDigits: 1,
    }).format(value);
  }
  activityHeight(count: number, total: number): number {
    return Math.max(2, (count / Math.max(1, total)) * 100);
  }
}
