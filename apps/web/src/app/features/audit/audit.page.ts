import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { ApiClient } from '../../core/api/api-client.service';
import type { AuditEvent } from '../../core/api/api.types';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';

@Component({
  selector: 'app-audit-page',
  imports: [ErrorPanelComponent, MatButtonModule],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('web.audit.title') }}</h1>
          <p>{{ i18n.plural('common.resultCount', events().length) }}</p>
        </div>
      </header>
      @if (error()) {
        <app-error-panel [error]="error()" (retry)="load()" />
      }
      <section class="panel audit-table" [attr.aria-label]="i18n.t('web.audit.title')" tabindex="0">
        @if (loading()) {
          <div class="loading">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else if (events().length === 0) {
          <div class="empty-state">{{ i18n.t('web.audit.empty') }}</div>
        } @else {
          <div class="table-header" aria-hidden="true">
            <span>{{ i18n.t('web.audit.date') }}</span
            ><span>{{ i18n.t('web.audit.action') }}</span
            ><span>{{ i18n.t('web.audit.entity') }}</span
            ><span>{{ i18n.t('web.audit.id') }}</span>
          </div>
          @for (event of events(); track event.id) {
            <article>
              <time [attr.datetime]="event.occurredAt">{{
                i18n.date(event.occurredAt, { dateStyle: 'medium', timeStyle: 'medium' })
              }}</time
              ><strong>{{ auditAction(event.action) }}</strong
              ><span>{{ entityType(event.entityType) }}</span
              ><code>{{ event.entityId }}</code>
            </article>
          }
        }
      </section>
    </div>
  `,
  styles: `
    .audit-table {
      overflow: auto;
    }
    .audit-table:focus-visible {
      outline: 3px solid var(--brand);
      outline-offset: 2px;
    }
    .table-header,
    article {
      display: grid;
      grid-template-columns: minmax(11rem, 0.8fr) minmax(10rem, 1fr) minmax(8rem, 0.7fr) minmax(
          14rem,
          1fr
        );
      gap: 1rem;
      align-items: center;
      min-width: 46rem;
      padding: 0.75rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .table-header {
      color: var(--text-muted);
      background: var(--surface-subtle);
      font-size: 0.72rem;
      font-weight: 650;
      text-transform: uppercase;
    }
    .audit-table article:last-child {
      border: 0;
    }
    article time,
    article span {
      color: var(--text-muted);
      font-size: 0.8rem;
    }
    article strong {
      font-size: 0.82rem;
    }
    article code {
      overflow: hidden;
      color: var(--text-faint);
      font-size: 0.72rem;
      text-overflow: ellipsis;
    }
    .loading {
      display: grid;
      gap: 0.75rem;
      padding: 1rem;
    }
    .loading > div {
      min-height: 2.5rem;
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuditPage implements OnInit {
  readonly i18n = inject(I18nService);
  readonly events = signal<readonly AuditEvent[]>([]);
  readonly loading = signal(false);
  readonly error = signal<unknown>(null);
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  ngOnInit(): void {
    void this.load();
  }
  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.events.set(await this.api.listAudit(workspaceId));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }
  auditAction(action: string): string {
    const key = `web.audit.action.${action}` as AppMessageKey;
    const value = this.i18n.t(key);
    return value === key ? action : value;
  }
  entityType(entity: string): string {
    const key = `web.entity.${entity}` as AppMessageKey;
    const value = this.i18n.t(key);
    return value === key ? entity : value;
  }
}
