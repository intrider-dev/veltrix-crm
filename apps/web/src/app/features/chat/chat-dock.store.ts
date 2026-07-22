import { computed, effect, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  Call,
  CallJoin,
  ChatAttachment,
  ChatConversation,
  ChatMessage,
  WorkspaceMember,
} from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { NotificationRealtimeService } from '../../core/notifications/notification-realtime.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';
import { CallSessionService } from './call-session.service';

@Injectable()
export class ChatDockStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly realtime = inject(NotificationRealtimeService);
  private readonly auth = inject(AuthStore);
  readonly callSession = inject(CallSessionService);
  private readonly toasts = inject(ToastService);

  readonly conversations = signal<readonly ChatConversation[]>([]);
  readonly messages = signal<readonly ChatMessage[]>([]);
  readonly attachments = signal<readonly ChatAttachment[]>([]);
  readonly members = signal<readonly WorkspaceMember[]>([]);
  readonly activeConversationId = signal<string | null>(null);
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly sending = signal(false);
  readonly error = signal<unknown>(null);
  readonly callsEnabled = signal(false);
  readonly incomingCall = signal<Call | null>(null);
  private callConfigLoaded = false;
  readonly unreadCount = computed(() =>
    this.conversations().reduce((total, conversation) => total + conversation.unreadCount, 0),
  );
  readonly activeConversation = computed(
    () =>
      this.conversations().find(
        (conversation) => conversation.id === this.activeConversationId(),
      ) ?? null,
  );
  readonly attachmentsByMessage = computed(() => {
    const result = new Map<string, ChatAttachment[]>();
    for (const attachment of this.attachments()) {
      const items = result.get(attachment.messageId) ?? [];
      items.push(attachment);
      result.set(attachment.messageId, items);
    }
    return result;
  });

  constructor() {
    effect(() => {
      const sequence = this.realtime.chatSequence();
      const changedConversation = this.realtime.chatConversationId();
      if (sequence === 0) return;
      void this.loadConversations();
      if (changedConversation && changedConversation === this.activeConversationId()) {
        void this.loadMessages(changedConversation);
      }
    });
    effect(() => {
      const sequence = this.realtime.callSequence();
      const event = this.realtime.callEvent();
      if (sequence === 0 || !event) return;
      if (event.type === 'call.ended') {
        if (this.incomingCall()?.id === event.callId) this.incomingCall.set(null);
        if (this.callSession.activeCall()?.id === event.callId) this.callSession.disconnect();
        return;
      }
      if (event.createdBy === this.auth.user()?.id) return;
      const workspaceId = this.workspace.id();
      if (!workspaceId) return;
      void this.api
        .getCall(workspaceId, event.callId)
        .then((call) => this.incomingCall.set(call))
        .catch(() => undefined);
    });
  }

  async loadConversations(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const conversations = await this.api.listConversations(workspaceId);
      this.conversations.set(conversations);
      if (!this.callConfigLoaded) {
        this.callConfigLoaded = true;
        const config = await this.api.callConfig(workspaceId);
        this.callsEnabled.set(config.enabled);
      }
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async startCall(kind: 'audio' | 'video'): Promise<CallJoin> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    if (!workspaceId || !conversationId || !this.callsEnabled()) throw new Error('calls disabled');
    const call = await this.api.createCall(workspaceId, conversationId, kind);
    return this.api.joinCall(workspaceId, call.id);
  }

  async acceptCall(): Promise<CallJoin> {
    const workspaceId = this.workspace.id();
    const call = this.incomingCall();
    if (!workspaceId || !call) throw new Error('no incoming call');
    this.incomingCall.set(null);
    await this.select(call.conversationId);
    return this.api.joinCall(workspaceId, call.id);
  }

  async declineCall(): Promise<void> {
    const workspaceId = this.workspace.id();
    const call = this.incomingCall();
    if (!workspaceId || !call) return;
    await this.api.declineCall(workspaceId, call.id);
    this.incomingCall.set(null);
  }

  async leaveActiveCall(): Promise<void> {
    const workspaceId = this.workspace.id();
    const call = this.callSession.activeCall();
    if (!workspaceId || !call) return;
    try {
      if (call.createdBy === this.auth.user()?.id) await this.api.endCall(workspaceId, call.id);
      else await this.api.leaveCall(workspaceId, call.id);
    } finally {
      this.callSession.disconnect();
    }
  }

  async loadMembers(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.members().length > 0) return;
    try {
      this.members.set(
        (await this.api.listMembers(workspaceId)).filter((item) => item.status === 'active'),
      );
    } catch (error) {
      this.error.set(error);
    }
  }

  async startDirect(userId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.sending.set(true);
    this.error.set(null);
    try {
      const conversation = await this.api.createConversation(workspaceId, {
        title: '',
        memberUserIds: [userId],
      });
      this.conversations.update((items) => [
        conversation,
        ...items.filter((item) => item.id !== conversation.id),
      ]);
      await this.select(conversation.id);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.sending.set(false);
    }
  }

  async select(conversationId: string): Promise<void> {
    this.activeConversationId.set(conversationId);
    await this.loadMessages(conversationId);
    const workspaceId = this.workspace.id();
    if (workspaceId) {
      await this.api.markConversationRead(workspaceId, conversationId).catch(() => undefined);
      this.conversations.update((items) =>
        items.map((item) => (item.id === conversationId ? { ...item, unreadCount: 0 } : item)),
      );
    }
  }

  async loadMessages(conversationId: string, older = false): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [page, attachments] = await Promise.all([
        this.api.listChatMessages(
          workspaceId,
          conversationId,
          older ? (this.nextCursor() ?? undefined) : undefined,
        ),
        older
          ? Promise.resolve(this.attachments())
          : this.api.listChatAttachments(workspaceId, conversationId),
      ]);
      const chronological = [...page.items].reverse();
      this.messages.update((items) => (older ? [...chronological, ...items] : chronological));
      if (!older) this.attachments.set(attachments);
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async send(body: string, file: File | null): Promise<void> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    const normalized = body.trim() || (file ? file.name : '');
    if (!workspaceId || !conversationId || !normalized || this.sending()) return;
    this.sending.set(true);
    this.error.set(null);
    try {
      const message = await this.api.sendChatMessage(workspaceId, conversationId, {
        kind: 'text',
        body: normalized,
        replyToMessageId: null,
      });
      this.messages.update((items) =>
        items.some((item) => item.id === message.id) ? items : [...items, message],
      );
      if (file) {
        try {
          const uploaded = await this.api.uploadAttachment(
            workspaceId,
            'chat_message',
            message.id,
            file,
          );
          this.attachments.update((items) => [
            ...items,
            {
              id: uploaded.id,
              messageId: message.id,
              displayName: uploaded.displayName,
              mediaType: uploaded.mediaType,
              sizeBytes: uploaded.sizeBytes,
              scanState: uploaded.scanState,
              createdAt: uploaded.createdAt,
            },
          ]);
        } catch {
          this.toasts.show({ messageKey: 'chat.uploadFailed', messageParams: {} });
        }
      }
      await this.loadConversations();
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.sending.set(false);
    }
  }

  async react(messageId: string, emoji: string): Promise<void> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    if (!workspaceId || !conversationId) return;
    await this.api.addChatReaction(workspaceId, messageId, emoji);
    await this.loadMessages(conversationId);
  }

  async pin(message: ChatMessage): Promise<void> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    if (!workspaceId || !conversationId) return;
    await this.api.setChatMessagePinned(workspaceId, message.id, !message.pinned);
    await this.loadMessages(conversationId);
  }

  async download(attachment: ChatAttachment): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const blob = await this.api.downloadAttachment(workspaceId, attachment.id);
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = attachment.displayName;
    link.rel = 'noopener';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
}
