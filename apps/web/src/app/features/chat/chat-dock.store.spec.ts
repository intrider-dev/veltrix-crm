import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { ChatMessage } from '../../core/api/api.types';
import { NotificationRealtimeService } from '../../core/notifications/notification-realtime.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';
import { ChatDockStore } from './chat-dock.store';

const message: ChatMessage = {
  id: 'message-1',
  conversationId: 'conversation-1',
  senderUserId: 'user-1',
  senderDisplayName: 'Alice',
  kind: 'text',
  body: 'Hello',
  pinned: false,
  reactions: [],
  version: 1,
  createdAt: '2026-07-22T00:00:00Z',
};

describe('ChatDockStore', () => {
  it('sends through the active membership-scoped conversation and appends once', async () => {
    const api = {
      sendChatMessage: vi.fn().mockResolvedValue(message),
      listConversations: vi.fn().mockResolvedValue([]),
    };
    TestBed.configureTestingModule({
      providers: [
        ChatDockStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        {
          provide: NotificationRealtimeService,
          useValue: { chatSequence: signal(0), chatConversationId: signal<string | null>(null) },
        },
        { provide: ToastService, useValue: { show: vi.fn() } },
      ],
    });
    const store = TestBed.inject(ChatDockStore);
    store.activeConversationId.set('conversation-1');

    await store.send(' Hello ', null);

    expect(api.sendChatMessage).toHaveBeenCalledWith('workspace-1', 'conversation-1', {
      kind: 'text',
      body: 'Hello',
      replyToMessageId: null,
    });
    expect(store.messages()).toEqual([message]);
  });

  it('orders each server page chronologically while keeping pagination bounded', async () => {
    const later = { ...message, id: 'message-2', createdAt: '2026-07-22T00:01:00Z' };
    const api = {
      listChatMessages: vi.fn().mockResolvedValue({ items: [later, message], nextCursor: 'older' }),
      listChatAttachments: vi.fn().mockResolvedValue([]),
    };
    TestBed.configureTestingModule({
      providers: [
        ChatDockStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        {
          provide: NotificationRealtimeService,
          useValue: { chatSequence: signal(0), chatConversationId: signal<string | null>(null) },
        },
        { provide: ToastService, useValue: { show: vi.fn() } },
      ],
    });
    const store = TestBed.inject(ChatDockStore);

    await store.loadMessages('conversation-1');

    expect(store.messages().map((item) => item.id)).toEqual(['message-1', 'message-2']);
    expect(store.nextCursor()).toBe('older');
  });
});
