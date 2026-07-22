import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';

import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { DashboardStore } from './dashboard.store';

@Component({
  selector: 'app-dashboard-page',
  imports: [ErrorPanelComponent, MatButtonModule, MatCardModule],
  providers: [DashboardStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('dashboard.dashboard.title') }}</h1>
        </div>
        <div class="period" role="group" [attr.aria-label]="i18n.t('dashboard.dashboard.title')">
          <button mat-button type="button" class="active">
            {{ i18n.t('dashboard.dashboard.period.30d') }}
          </button>
        </div>
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      @if (store.loading() && !store.dashboard()) {
        <div class="kpi-grid" [attr.aria-label]="i18n.t('common.app.loading')">
          <div class="skeleton card"></div>
          <div class="skeleton card"></div>
          <div class="skeleton card"></div>
          <div class="skeleton card"></div>
        </div>
      } @else if (store.dashboard(); as dashboard) {
        <section class="kpi-grid">
          <mat-card appearance="outlined"
            ><span>{{ i18n.t('dashboard.dashboard.openPipeline') }}</span
            ><strong>{{
              i18n.money(dashboard.openPipelineMinor, dashboard.currency)
            }}</strong></mat-card
          >
          <mat-card appearance="outlined"
            ><span>{{ i18n.t('dashboard.dashboard.weighted') }}</span
            ><strong>{{
              i18n.money(dashboard.weightedForecastMinor, dashboard.currency)
            }}</strong></mat-card
          >
          <mat-card appearance="outlined"
            ><span>{{ i18n.t('dashboard.dashboard.won') }}</span
            ><strong>{{ dashboard.wonCount }}</strong></mat-card
          >
          <mat-card appearance="outlined"
            ><span>{{ i18n.t('dashboard.dashboard.overdue') }}</span
            ><strong [class.danger]="dashboard.overdueTasks > 0">{{
              dashboard.overdueTasks
            }}</strong></mat-card
          >
        </section>

        <section class="dashboard-grid">
          <article class="panel chart-panel">
            <header>
              <h2>{{ i18n.t('dashboard.dashboard.dealsByStage') }}</h2>
            </header>
            @defer (on viewport) {
              @if ((dashboard.dealsByStage?.length ?? 0) > 0) {
                <div class="bars">
                  @for (stage of dashboard.dealsByStage; track stage.stageId) {
                    <div class="bar-row">
                      <div>
                        <span>{{ stage.stageName }}</span
                        ><strong>{{ i18n.money(stage.amountMinor, dashboard.currency) }}</strong>
                      </div>
                      <div class="track">
                        <span
                          [style.width.%]="
                            stageWidth(stage.amountMinor, dashboard.openPipelineMinor)
                          "
                        ></span>
                      </div>
                    </div>
                  }
                </div>
              } @else {
                <div class="empty-state">{{ i18n.t('sales.pipeline.empty') }}</div>
              }
            } @placeholder {
              <div class="skeleton chart-placeholder"></div>
            }
          </article>

          <article class="panel activity-panel">
            <header>
              <h2>{{ i18n.t('dashboard.dashboard.activity') }}</h2>
            </header>
            @if (store.activities().length === 0) {
              <div class="empty-state">{{ i18n.t('dashboard.dashboard.emptyActivity') }}</div>
            } @else {
              <ol>
                @for (activity of store.activities(); track activity.id) {
                  <li>
                    <span class="activity-dot" aria-hidden="true"></span>
                    <div>
                      <strong>{{ activity.title }}</strong
                      ><small>{{
                        i18n.date(activity.occurredAt, { dateStyle: 'medium', timeStyle: 'short' })
                      }}</small>
                    </div>
                  </li>
                }
              </ol>
            }
          </article>
        </section>
      }
    </div>
  `,
  styles: `
    .period {
      padding: 0.2rem;
      border-radius: 0.55rem;
      background: var(--surface-subtle);
    }
    .period .active {
      color: var(--brand);
      background: var(--surface-raised);
    }
    .kpi-grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 1rem;
    }
    .kpi-grid mat-card {
      min-height: 7.5rem;
      padding: 1.1rem;
      border-color: var(--border);
      background: var(--surface-raised);
    }
    .kpi-grid span {
      color: var(--text-muted);
      font-size: 0.82rem;
    }
    .kpi-grid strong {
      display: block;
      margin-top: 0.75rem;
      font-size: clamp(1.4rem, 3vw, 2rem);
      letter-spacing: -0.04em;
    }
    .kpi-grid strong.danger {
      color: var(--danger);
    }
    .card {
      min-height: 7.5rem;
    }
    .dashboard-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.55fr) minmax(18rem, 0.75fr);
      gap: 1rem;
    }
    article > header {
      padding: 1rem 1.1rem;
      border-bottom: 1px solid var(--border);
    }
    h2 {
      margin: 0;
      font-size: 1rem;
    }
    .bars {
      display: grid;
      gap: 1.1rem;
      padding: 1.25rem;
    }
    .bar-row > div:first-child {
      display: flex;
      justify-content: space-between;
      gap: 1rem;
      margin-bottom: 0.45rem;
      font-size: 0.82rem;
    }
    .track {
      height: 0.55rem;
      overflow: hidden;
      border-radius: 2rem;
      background: var(--surface-subtle);
    }
    .track span {
      display: block;
      min-width: 0.25rem;
      height: 100%;
      border-radius: inherit;
      background: var(--brand);
    }
    .chart-placeholder {
      min-height: 16rem;
      margin: 1rem;
    }
    ol {
      display: grid;
      gap: 0;
      margin: 0;
      padding: 0.4rem 1rem;
      list-style: none;
    }
    li {
      position: relative;
      display: grid;
      grid-template-columns: 1rem 1fr;
      gap: 0.6rem;
      padding: 0.75rem 0;
      border-bottom: 1px solid var(--border);
    }
    li:last-child {
      border: 0;
    }
    .activity-dot {
      width: 0.55rem;
      height: 0.55rem;
      margin-top: 0.25rem;
      border-radius: 50%;
      background: var(--brand);
      box-shadow: 0 0 0 0.25rem var(--brand-soft);
    }
    li strong,
    li small {
      display: block;
    }
    li strong {
      font-size: 0.85rem;
    }
    li small {
      margin-top: 0.25rem;
      color: var(--text-muted);
      font-size: 0.72rem;
    }
    @media (max-width: 980px) {
      .kpi-grid {
        grid-template-columns: repeat(2, 1fr);
      }
      .dashboard-grid {
        grid-template-columns: 1fr;
      }
    }
    @media (max-width: 520px) {
      .kpi-grid {
        grid-template-columns: 1fr;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardPage implements OnInit {
  readonly store = inject(DashboardStore);
  readonly i18n = inject(I18nService);

  ngOnInit(): void {
    void this.store.load();
  }

  stageWidth(amount: number, total: number): number {
    return total <= 0 ? 0 : Math.max(2, Math.min(100, (amount / total) * 100));
  }
}
