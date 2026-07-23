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

    await expect(store.send(' Hello ', null)).resolves.toBe(true);

    expect(api.sendChatMessage).toHaveBeenCalledWith(
      'workspace-1',
      'conversation-1',
      {
        kind: 'text',
        body: 'Hello',
        replyToMessageId: null,
      },
      expect.any(String),
    );
    expect(store.messages()).toEqual([message]);
  });

  it('keeps a message intact when the entity conversation is not ready', async () => {
    const api = { sendChatMessage: vi.fn() };
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

    await expect(TestBed.inject(ChatDockStore).send('Do not lose me', null)).resolves.toBe(false);
    expect(api.sendChatMessage).not.toHaveBeenCalled();
  });

  it('reuses the message idempotency key after an ambiguous response failure', async () => {
    const api = {
      sendChatMessage: vi
        .fn()
        .mockRejectedValueOnce(new Error('response lost'))
        .mockResolvedValueOnce(message),
      listConversations: vi.fn().mockResolvedValue([]),
      callConfig: vi.fn().mockResolvedValue({ enabled: false }),
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

    await expect(store.send('Hello', null)).resolves.toBe(false);
    await expect(store.send('Hello', null)).resolves.toBe(true);

    const firstKey: unknown = api.sendChatMessage.mock.calls[0]?.[3];
    const secondKey: unknown = api.sendChatMessage.mock.calls[1]?.[3];
    expect(firstKey).toEqual(expect.any(String));
    expect(secondKey).toBe(firstKey);
  });

  it('reuses the call idempotency key after an ambiguous create response', async () => {
    const call = {
      id: 'call-1',
      conversationId: 'conversation-1',
      kind: 'audio' as const,
      state: 'ringing' as const,
      participantState: 'invited' as const,
      createdBy: 'user-1',
      startedAt: null,
      endedAt: null,
      version: 1,
      createdAt: '2026-07-22T00:00:00Z',
    };
    const join = {
      call,
      url: 'wss://calls.example.test',
      token: 'token',
      expiresAt: '2026-07-22T00:05:00Z',
    };
    const api = {
      createCall: vi
        .fn()
        .mockRejectedValueOnce(new Error('response lost'))
        .mockResolvedValueOnce(call),
      joinCall: vi.fn().mockResolvedValue(join),
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
    store.callsEnabled.set(true);

    await expect(store.startCall('audio')).rejects.toThrow('response lost');
    await expect(store.startCall('audio')).resolves.toEqual(join);

    expect(api.createCall.mock.calls[0]?.[3]).toEqual(expect.any(String));
    expect(api.createCall.mock.calls[1]?.[3]).toBe(api.createCall.mock.calls[0]?.[3]);
  });

  it('stops local media before waiting for server-side call cleanup', async () => {
    const call = {
      id: 'call-1',
      conversationId: 'conversation-1',
      kind: 'video' as const,
      state: 'active' as const,
      participantState: 'joined' as const,
      createdBy: 'another-user',
      startedAt: '2026-07-22T00:00:00Z',
      endedAt: null,
      version: 2,
      createdAt: '2026-07-22T00:00:00Z',
    };
    let resolveLeave!: () => void;
    const api = {
      leaveCall: vi.fn().mockReturnValue(new Promise<void>((resolve) => (resolveLeave = resolve))),
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
    store.callSession.activeCall.set(call);

    const leaving = store.leaveActiveCall();
    expect(store.callSession.activeCall()).toBeNull();
    expect(api.leaveCall).toHaveBeenCalledWith('workspace-1', 'call-1');
    resolveLeave();
    await leaving;
  });

  it('removes a media message that resolves after the conversation changed', async () => {
    const file = new File(['voice'], 'voice.webm', { type: 'audio/webm' });
    let resolveMessage!: (value: ChatMessage) => void;
    const response = new Promise<ChatMessage>((resolve) => (resolveMessage = resolve));
    const api = {
      sendChatMessage: vi.fn().mockReturnValue(response),
      deleteProvisionalChatMessage: vi.fn().mockResolvedValue(undefined),
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
    const send = store.send('Voice message', file, 'voice');
    store.activeConversationId.set('conversation-2');
    resolveMessage({ ...message, kind: 'voice' });

    await expect(send).resolves.toBe(false);
    expect(api.deleteProvisionalChatMessage).toHaveBeenCalledWith('workspace-1', 'message-1');
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
    store.activeConversationId.set('conversation-1');

    await store.loadMessages('conversation-1');

    expect(store.messages().map((item) => item.id)).toEqual(['message-1', 'message-2']);
    expect(store.nextCursor()).toBe('older');
    expect(api.listChatAttachments).toHaveBeenCalledWith('workspace-1', 'conversation-1', [
      'message-2',
      'message-1',
    ]);
  });

  it('does not clear unread state when the message page fails to load', async () => {
    const api = {
      listChatMessages: vi.fn().mockRejectedValue(new Error('offline')),
      markConversationRead: vi.fn(),
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

    await TestBed.inject(ChatDockStore).select('conversation-1');

    expect(api.markConversationRead).not.toHaveBeenCalled();
  });

  it('discards an unattached provisional message after an upload failure', async () => {
    const file = new File(['voice'], 'voice.webm', { type: 'audio/webm' });
    const mediaMessage = { ...message, kind: 'voice' as const, body: 'Voice message' };
    const api = {
      sendChatMessage: vi.fn().mockResolvedValue(mediaMessage),
      uploadAttachment: vi.fn().mockRejectedValue(new Error('upload failed')),
      deleteProvisionalChatMessage: vi.fn().mockResolvedValue(undefined),
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

    await expect(store.send('Voice message', file, 'voice')).resolves.toBe(false);

    expect(api.deleteProvisionalChatMessage).toHaveBeenCalledWith('workspace-1', 'message-1');
    expect(store.messages()).toEqual([]);
  });

  it('deduplicates an attachment when SSE refresh wins the upload response race', async () => {
    const file = new File(['voice'], 'voice.webm', { type: 'audio/webm' });
    const uploaded = {
      id: 'attachment-1',
      entityType: 'chat_message' as const,
      entityId: 'message-1',
      displayName: 'voice.webm',
      mediaType: 'audio/webm',
      sizeBytes: 5,
      scanState: 'clean' as const,
      createdAt: '2026-07-22T00:00:01Z',
    };
    let resolveUpload!: (value: typeof uploaded) => void;
    const upload = new Promise<typeof uploaded>((resolve) => (resolveUpload = resolve));
    const api = {
      sendChatMessage: vi.fn().mockResolvedValue({ ...message, kind: 'voice' as const }),
      uploadAttachment: vi.fn().mockReturnValue(upload),
      listConversations: vi.fn().mockResolvedValue([]),
      callConfig: vi.fn().mockResolvedValue({ enabled: false }),
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
    const send = store.send('Voice message', file, 'voice');
    await Promise.resolve();
    store.attachments.set([{ ...uploaded, messageId: 'message-1' }]);
    resolveUpload(uploaded);

    await expect(send).resolves.toBe(true);
    expect(store.attachments().map((item) => item.id)).toEqual(['attachment-1']);
  });
});
