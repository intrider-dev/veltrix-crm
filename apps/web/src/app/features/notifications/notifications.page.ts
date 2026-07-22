import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

import type { NotificationItem } from '../../core/api/api.types';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { NotificationsStore } from './notifications.store';

@Component({
  selector: 'app-notifications-page',
  imports: [ErrorPanelComponent, MatButtonModule, RouterLink],
  providers: [NotificationsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.notifications') }}</h1>
          <p>{{ i18n.t('notifications.subtitle') }}</p>
        </div>
        <button mat-stroked-button type="button" (click)="store.markAllRead()">
          {{ i18n.t('notifications.markAllRead') }}
        </button>
      </header>
      <section class="panel feature-toolbar">
        <button
          mat-button
          type="button"
          [class.active]="!store.unreadOnly()"
          (click)="setUnread(false)"
        >
          {{ i18n.t('notifications.all') }}
        </button>
        <button
          mat-button
          type="button"
          [class.active]="store.unreadOnly()"
          (click)="setUnread(true)"
        >
          {{ i18n.t('notifications.unread') }}
        </button>
        <span class="live-status" role="status">{{ i18n.t('notifications.live') }}</span>
      </section>
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      <section class="panel notification-list" [attr.aria-busy]="store.loading()">
        @if (store.loading() && store.items().length === 0) {
          <div class="list-skeleton">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else if (store.items().length === 0) {
          <div class="empty-state">{{ i18n.t('notifications.empty') }}</div>
        } @else {
          @for (item of store.items(); track item.id) {
            <article [class.unread]="!item.readAt">
              <span class="unread-mark" aria-hidden="true"></span>
              <div>
                <p>{{ message(item) }}</p>
                <time [attr.datetime]="item.createdAt">{{
                  i18n.date(item.createdAt, { dateStyle: 'medium', timeStyle: 'short' })
                }}</time>
              </div>
              <div class="actions">
                @if (entityLink(item); as link) {
                  <a mat-button [routerLink]="link">{{ i18n.t('notifications.open') }}</a>
                }
                @if (!item.readAt) {
                  <button mat-button type="button" (click)="store.markRead(item)">
                    {{ i18n.t('notifications.markRead') }}
                  </button>
                }
              </div>
            </article>
          }
        }
      </section>
      @if (store.nextCursor()) {
        <button
          mat-stroked-button
          type="button"
          [disabled]="store.loading()"
          (click)="store.load(false)"
        >
          {{ i18n.t('notifications.loadMore') }}
        </button>
      }
    </div>
  `,
  styles: `
    .feature-toolbar .active {
      color: var(--brand);
      background: var(--brand-soft);
    }
    .live-status {
      margin-inline-start: auto;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .notification-list article {
      display: grid;
      grid-template-columns: 0.5rem minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.75rem;
      padding: 0.9rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .notification-list article:last-child {
      border: 0;
    }
    .notification-list article.unread {
      background: color-mix(in srgb, var(--brand) 4%, transparent);
    }
    .unread-mark {
      width: 0.45rem;
      height: 0.45rem;
      border-radius: 50%;
      background: transparent;
    }
    .unread .unread-mark {
      background: var(--brand);
    }
    article p {
      margin: 0 0 0.25rem;
    }
    article time {
      color: var(--text-faint);
      font-size: 0.72rem;
    }
    .actions {
      display: flex;
      gap: 0.25rem;
    }
    @media (max-width: 650px) {
      .notification-list article {
        grid-template-columns: 0.5rem 1fr;
      }
      .actions {
        grid-column: 2;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotificationsPage implements OnInit {
  readonly store = inject(NotificationsStore);
  readonly i18n = inject(I18nService);
  ngOnInit(): void {
    void this.store.load();
  }
  setUnread(value: boolean): void {
    this.store.unreadOnly.set(value);
    void this.store.load();
  }
  message(item: NotificationItem): string {
    const params: Record<string, string | number | boolean> = {};
    for (const [key, value] of Object.entries(item.messageParams)) {
      if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
        params[key] = value;
      }
    }
    return this.i18n.t(item.messageKey as AppMessageKey, params);
  }
  entityLink(item: NotificationItem): string | null {
    if (!item.entityId) return null;
    if (item.entityType === 'contact') return `/contacts/${item.entityId}`;
    if (item.entityType === 'company') return `/companies/${item.entityId}`;
    if (item.entityType === 'deal') return `/deals/${item.entityId}`;
    return null;
  }
}
