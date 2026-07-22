import { effect, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { NotificationItem } from '../../core/api/api.types';
import { NotificationRealtimeService } from '../../core/notifications/notification-realtime.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class NotificationsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly realtime = inject(NotificationRealtimeService);

  readonly items = signal<readonly NotificationItem[]>([]);
  readonly unreadOnly = signal(false);
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly error = signal<unknown>(null);

  constructor() {
    effect(() => {
      const sequence = this.realtime.sequence();
      if (sequence > 0) void this.load();
    });
  }

  async load(reset = true): Promise<void> {
    const id = this.workspace.id();
    if (!id || (!reset && !this.nextCursor())) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listNotifications(id, {
        unread: this.unreadOnly(),
        cursor: reset ? undefined : (this.nextCursor() ?? undefined),
        limit: 50,
      });
      this.items.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async markRead(item: NotificationItem): Promise<void> {
    const id = this.workspace.id();
    if (!id || item.readAt) return;
    try {
      const updated = await this.api.markNotificationRead(id, item);
      if (this.unreadOnly())
        this.items.update((items) => items.filter((current) => current.id !== item.id));
      else this.replace(updated);
    } catch (error) {
      this.error.set(error);
    }
  }

  async markAllRead(): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    try {
      await this.api.markAllNotificationsRead(id);
      await this.load();
    } catch (error) {
      this.error.set(error);
    }
  }

  private replace(item: NotificationItem): void {
    this.items.update((items) => items.map((current) => (current.id === item.id ? item : current)));
  }
}
