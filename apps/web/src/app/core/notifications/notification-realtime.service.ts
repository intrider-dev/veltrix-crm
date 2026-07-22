import { effect, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../api/api-client.service';
import { AuthStore } from '../auth/auth.store';
import { I18nService } from '../i18n/i18n.service';
import { WorkspaceStore } from '../workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';

interface NotificationEventPayload {
  readonly recipientUserId?: string;
  readonly messageKey?: string;
  readonly messageParams?: Readonly<Record<string, unknown>>;
}

interface ChatEventPayload {
  readonly conversationId?: string;
  readonly messageId?: string;
}

export interface CallRealtimeEvent {
  readonly type: 'call.invited' | 'call.ended';
  readonly callId: string;
  readonly conversationId: string;
  readonly kind?: 'audio' | 'video';
  readonly createdBy?: string;
}

@Injectable({ providedIn: 'root' })
export class NotificationRealtimeService {
  private readonly api = inject(ApiClient);
  private readonly auth = inject(AuthStore);
  private readonly workspace = inject(WorkspaceStore);
  private readonly i18n = inject(I18nService);
  private readonly toasts = inject(ToastService);
  readonly sequence = signal(0);
  readonly chatSequence = signal(0);
  readonly chatConversationId = signal<string | null>(null);
  readonly callSequence = signal(0);
  readonly callEvent = signal<CallRealtimeEvent | null>(null);

  constructor() {
    effect((onCleanup) => {
      const workspaceId = this.workspace.id();
      const userId = this.auth.user()?.id;
      if (!workspaceId || !userId || typeof EventSource === 'undefined') return;

      const stream = new EventSource(this.api.eventsUrl(workspaceId), { withCredentials: true });
      const listener = (event: Event) => {
        try {
          const payload = JSON.parse(
            (event as MessageEvent<string>).data,
          ) as NotificationEventPayload;
          if (payload.recipientUserId !== userId || !payload.messageKey) return;
          this.sequence.update((value) => value + 1);
          void this.showToast(payload);
        } catch {
          // A malformed event is ignored; reconnect and the notification inbox remain authoritative.
        }
      };
      stream.addEventListener('notification.created', listener);
      stream.addEventListener('authorization.changed', () => {
        // Permission decisions remain server-authoritative. Refresh the session
        // immediately so navigation and controls converge after a role change.
        void this.auth.refreshSession();
      });
      stream.addEventListener('chat.message.created', (event) => {
        try {
          const payload = JSON.parse((event as MessageEvent<string>).data) as ChatEventPayload;
          if (!payload.conversationId || !payload.messageId) return;
          this.chatConversationId.set(payload.conversationId);
          this.chatSequence.update((value) => value + 1);
        } catch {
          // The authorized REST API remains the source of truth after reconnect.
        }
      });
      for (const eventType of ['call.invited', 'call.ended'] as const) {
        stream.addEventListener(eventType, (event) => {
          try {
            const payload = JSON.parse(
              (event as MessageEvent<string>).data,
            ) as Partial<CallRealtimeEvent>;
            if (!payload.callId || !payload.conversationId) return;
            this.callEvent.set({
              type: eventType,
              callId: payload.callId,
              conversationId: payload.conversationId,
              kind: payload.kind,
              createdBy: payload.createdBy,
            });
            this.callSequence.update((value) => value + 1);
          } catch {
            // The participant-scoped REST resource remains authoritative.
          }
        });
      }
      onCleanup(() => stream.close());
    });
  }

  private async showToast(payload: NotificationEventPayload): Promise<void> {
    await this.i18n.loadNamespaces(['notifications']);
    this.toasts.show({
      messageKey: payload.messageKey ?? 'notifications.empty',
      messageParams: payload.messageParams ?? {},
      href: '/notifications',
    });
  }
}
