import { computed, DestroyRef, effect, inject, Injectable, signal, untracked } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  Call,
  CallJoin,
  ChatAttachment,
  ChatConversation,
  ChatMessage,
  ReferenceUser,
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
  private readonly destroyRef = inject(DestroyRef);

  readonly conversations = signal<readonly ChatConversation[]>([]);
  readonly messages = signal<readonly ChatMessage[]>([]);
  readonly attachments = signal<readonly ChatAttachment[]>([]);
  readonly members = signal<readonly ReferenceUser[]>([]);
  readonly activeConversationId = signal<string | null>(null);
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly sending = signal(false);
  readonly error = signal<unknown>(null);
  readonly callsEnabled = signal(false);
  readonly incomingCall = signal<Call | null>(null);
  private callConfigLoaded = false;
  private currentWorkspaceId: string | null = this.workspace.id();
  private conversationRequestSequence = 0;
  private entityOpenSequence = 0;
  private messageRequestSequence = 0;
  private pendingUpload: PendingChatUpload | null = null;
  private pendingSend: PendingChatSend | null = null;
  private pendingCall: PendingCallCreate | null = null;
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
      const workspaceId = this.workspace.id();
      untracked(() => {
        if (workspaceId === this.currentWorkspaceId) return;
        this.cleanupActiveCall(this.currentWorkspaceId);
        this.currentWorkspaceId = workspaceId;
        this.conversationRequestSequence += 1;
        this.entityOpenSequence += 1;
        this.messageRequestSequence += 1;
        this.callConfigLoaded = false;
        this.pendingUpload = null;
        this.pendingSend = null;
        this.conversations.set([]);
        this.messages.set([]);
        this.attachments.set([]);
        this.members.set([]);
        this.activeConversationId.set(null);
        this.nextCursor.set(null);
        this.loading.set(false);
        this.sending.set(false);
        this.error.set(null);
        this.incomingCall.set(null);
        this.callsEnabled.set(false);
        this.callSession.disconnect();
      });
    });
    this.destroyRef.onDestroy(() => this.cleanupActiveCall(this.workspace.id()));
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
        .then((call) => {
          if (this.workspace.id() === workspaceId) this.incomingCall.set(call);
        })
        .catch(() => undefined);
    });
  }

  async loadConversations(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const requestSequence = ++this.conversationRequestSequence;
    this.loading.set(true);
    this.error.set(null);
    try {
      const conversations = await this.api.listConversations(workspaceId);
      if (!this.isCurrentConversationRequest(workspaceId, requestSequence)) return;
      this.conversations.set(conversations);
      if (!this.callConfigLoaded) {
        const config = await this.api.callConfig(workspaceId);
        if (!this.isCurrentConversationRequest(workspaceId, requestSequence)) return;
        this.callsEnabled.set(config.enabled);
        this.callConfigLoaded = true;
      }
    } catch (error) {
      if (this.isCurrentConversationRequest(workspaceId, requestSequence)) this.error.set(error);
    } finally {
      if (this.isCurrentConversationRequest(workspaceId, requestSequence)) this.loading.set(false);
    }
  }

  async startCall(kind: 'audio' | 'video'): Promise<CallJoin> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    if (!workspaceId || !conversationId || !this.callsEnabled()) throw new Error('calls disabled');
    const pending =
      this.pendingCall?.workspaceId === workspaceId &&
      this.pendingCall.conversationId === conversationId &&
      this.pendingCall.kind === kind
        ? this.pendingCall
        : { workspaceId, conversationId, kind, idempotencyKey: crypto.randomUUID() };
    this.pendingCall = pending;
    let call: Call | null = null;
    try {
      call = await this.api.createCall(workspaceId, conversationId, kind, pending.idempotencyKey);
      if (this.pendingCall?.idempotencyKey === pending.idempotencyKey) this.pendingCall = null;
      if (!this.isCurrentContext(workspaceId, conversationId)) throw new Error('workspace changed');
      const join = await this.api.joinCall(workspaceId, call.id);
      if (!this.isCurrentContext(workspaceId, conversationId)) throw new Error('workspace changed');
      return join;
    } catch (error) {
      // A call that was created but never joined must not keep ringing for
      // other participants after a workspace switch or a failed join.
      if (call) await this.api.endCall(workspaceId, call.id).catch(() => undefined);
      throw error;
    }
  }

  async acceptCall(): Promise<CallJoin> {
    const workspaceId = this.workspace.id();
    const call = this.incomingCall();
    if (!workspaceId || !call) throw new Error('no incoming call');
    this.incomingCall.set(null);
    await this.select(call.conversationId);
    if (!this.isCurrentContext(workspaceId, call.conversationId))
      throw new Error('workspace changed');
    const join = await this.api.joinCall(workspaceId, call.id);
    if (!this.isCurrentContext(workspaceId, call.conversationId))
      throw new Error('workspace changed');
    return join;
  }

  async declineCall(): Promise<void> {
    const workspaceId = this.workspace.id();
    const call = this.incomingCall();
    if (!workspaceId || !call) return;
    await this.api.declineCall(workspaceId, call.id);
    if (this.workspace.id() === workspaceId && this.incomingCall()?.id === call.id) {
      this.incomingCall.set(null);
    }
  }

  async leaveActiveCall(): Promise<void> {
    const workspaceId = this.workspace.id();
    const call = this.callSession.activeCall();
    if (!workspaceId || !call) return;
    // Privacy is local-first: stop capture immediately, then let the server
    // cleanup complete independently of network latency.
    this.callSession.disconnect();
    if (call.createdBy === this.auth.user()?.id) await this.api.endCall(workspaceId, call.id);
    else await this.api.leaveCall(workspaceId, call.id);
  }

  async releaseJoinedCall(workspaceId: string | null, grant: CallJoin): Promise<void> {
    if (!workspaceId) return;
    const request =
      grant.call.createdBy === this.auth.user()?.id
        ? this.api.endCall(workspaceId, grant.call.id)
        : this.api.leaveCall(workspaceId, grant.call.id);
    await request.catch(() => undefined);
    if (this.callSession.activeCall()?.id === grant.call.id) this.callSession.disconnect();
  }

  async loadMembers(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.members().length > 0) return;
    try {
      const members = await this.api.listReferenceUsers(workspaceId);
      if (this.workspace.id() === workspaceId) this.members.set(members);
    } catch (error) {
      if (this.workspace.id() === workspaceId) this.error.set(error);
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
      if (this.workspace.id() !== workspaceId) return;
      this.conversations.update((items) => [
        conversation,
        ...items.filter((item) => item.id !== conversation.id),
      ]);
      await this.select(conversation.id);
    } catch (error) {
      if (this.workspace.id() === workspaceId) this.error.set(error);
      throw error;
    } finally {
      if (this.workspace.id() === workspaceId) this.sending.set(false);
    }
  }

  async openEntity(entityType: 'lead' | 'deal', entityId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const openSequence = ++this.entityOpenSequence;
    this.messageRequestSequence += 1;
    this.activeConversationId.set(null);
    this.messages.set([]);
    this.attachments.set([]);
    this.nextCursor.set(null);
    this.loading.set(true);
    this.error.set(null);
    try {
      const conversation = await this.api.resolveEntityConversation(
        workspaceId,
        entityType,
        entityId,
      );
      if (this.workspace.id() !== workspaceId || openSequence !== this.entityOpenSequence) return;
      this.conversations.update((items) => [
        conversation,
        ...items.filter((item) => item.id !== conversation.id),
      ]);
      await this.select(conversation.id);
    } catch (error) {
      if (this.workspace.id() === workspaceId && openSequence === this.entityOpenSequence) {
        this.error.set(error);
      }
    } finally {
      if (this.workspace.id() === workspaceId && openSequence === this.entityOpenSequence) {
        this.loading.set(false);
      }
    }
  }

  async select(conversationId: string): Promise<void> {
    this.messageRequestSequence += 1;
    this.messages.set([]);
    this.attachments.set([]);
    this.nextCursor.set(null);
    this.activeConversationId.set(conversationId);
    const loaded = await this.loadMessages(conversationId);
    const workspaceId = this.workspace.id();
    if (loaded && workspaceId && this.activeConversationId() === conversationId) {
      await this.api.markConversationRead(workspaceId, conversationId).catch(() => undefined);
      this.conversations.update((items) =>
        items.map((item) => (item.id === conversationId ? { ...item, unreadCount: 0 } : item)),
      );
    }
  }

  async loadMessages(conversationId: string, older = false): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    const requestSequence = ++this.messageRequestSequence;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listChatMessages(
        workspaceId,
        conversationId,
        older ? (this.nextCursor() ?? undefined) : undefined,
      );
      if (!this.isCurrentMessageRequest(workspaceId, conversationId, requestSequence)) return false;
      const attachments =
        page.items.length > 0
          ? await this.api.listChatAttachments(
              workspaceId,
              conversationId,
              page.items.map((message) => message.id),
            )
          : [];
      if (!this.isCurrentMessageRequest(workspaceId, conversationId, requestSequence)) return false;
      const chronological = [...page.items].reverse();
      this.messages.update((items) => (older ? [...chronological, ...items] : chronological));
      this.attachments.update((items) => {
        if (!older) return attachments;
        const existing = new Set(items.map((attachment) => attachment.id));
        return [...attachments.filter((attachment) => !existing.has(attachment.id)), ...items];
      });
      this.nextCursor.set(page.nextCursor ?? null);
      return true;
    } catch (error) {
      if (this.isCurrentMessageRequest(workspaceId, conversationId, requestSequence)) {
        this.error.set(error);
      }
      return false;
    } finally {
      if (this.isCurrentMessageRequest(workspaceId, conversationId, requestSequence)) {
        this.loading.set(false);
      }
    }
  }

  async send(
    body: string,
    file: File | null,
    kind: 'text' | 'file' | 'voice' | 'video' = file ? 'file' : 'text',
  ): Promise<boolean> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    const normalized = body.trim() || (file ? file.name : '');
    if (!workspaceId || !conversationId || !normalized || this.sending()) return false;
    this.sending.set(true);
    this.error.set(null);
    try {
      if (this.pendingUpload) {
        const samePending =
          this.pendingUpload.workspaceId === workspaceId &&
          this.pendingUpload.conversationId === conversationId &&
          this.pendingUpload.file === file &&
          this.pendingUpload.body === normalized &&
          this.pendingUpload.kind === kind;
        if (samePending) {
          return await this.uploadPendingAttachment(this.pendingUpload);
        }
        if (!(await this.discardPendingUpload(this.pendingUpload))) return false;
      }
      const pendingSend =
        this.pendingSend?.workspaceId === workspaceId &&
        this.pendingSend.conversationId === conversationId &&
        this.pendingSend.body === normalized &&
        this.pendingSend.kind === kind
          ? this.pendingSend
          : {
              workspaceId,
              conversationId,
              body: normalized,
              kind,
              idempotencyKey: crypto.randomUUID(),
            };
      this.pendingSend = pendingSend;
      const message = await this.api.sendChatMessage(
        workspaceId,
        conversationId,
        { kind, body: normalized, replyToMessageId: null },
        pendingSend.idempotencyKey,
      );
      if (this.pendingSend?.idempotencyKey === pendingSend.idempotencyKey) this.pendingSend = null;
      if (file) {
        this.pendingUpload = {
          workspaceId,
          conversationId,
          messageId: message.id,
          file,
          body: normalized,
          kind,
        };
      }
      if (!this.isCurrentContext(workspaceId, conversationId)) {
        if (this.pendingUpload?.messageId === message.id) {
          await this.discardPendingUpload(this.pendingUpload);
        }
        return false;
      }
      this.messages.update((items) =>
        items.some((item) => item.id === message.id) ? items : [...items, message],
      );
      if (file && this.pendingUpload?.messageId === message.id) {
        if (!(await this.uploadPendingAttachment(this.pendingUpload))) return false;
      }
      await this.loadConversations();
      return true;
    } catch (error) {
      if (this.workspace.id() === workspaceId) this.error.set(error);
      return false;
    } finally {
      if (this.workspace.id() === workspaceId) this.sending.set(false);
    }
  }

  async react(messageId: string, emoji: string): Promise<void> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    if (!workspaceId || !conversationId) return;
    await this.api.addChatReaction(workspaceId, messageId, emoji);
    if (this.isCurrentContext(workspaceId, conversationId)) {
      await this.loadMessages(conversationId);
    }
  }

  async pin(message: ChatMessage): Promise<void> {
    const workspaceId = this.workspace.id();
    const conversationId = this.activeConversationId();
    if (!workspaceId || !conversationId) return;
    await this.api.setChatMessagePinned(workspaceId, message.id, !message.pinned);
    if (this.isCurrentContext(workspaceId, conversationId)) {
      await this.loadMessages(conversationId);
    }
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

  private isCurrentConversationRequest(workspaceId: string, sequence: number): boolean {
    return this.workspace.id() === workspaceId && this.conversationRequestSequence === sequence;
  }

  private cleanupActiveCall(workspaceId: string | null): void {
    const call = this.callSession.activeCall();
    if (!workspaceId || !call) return;
    const request =
      call.createdBy === this.auth.user()?.id
        ? this.api.endCall(workspaceId, call.id)
        : this.api.leaveCall(workspaceId, call.id);
    void request.catch(() => undefined);
  }

  private isCurrentContext(workspaceId: string, conversationId: string): boolean {
    return this.workspace.id() === workspaceId && this.activeConversationId() === conversationId;
  }

  private isCurrentMessageRequest(
    workspaceId: string,
    conversationId: string,
    sequence: number,
  ): boolean {
    return (
      this.workspace.id() === workspaceId &&
      this.activeConversationId() === conversationId &&
      this.messageRequestSequence === sequence
    );
  }

  private async uploadPendingAttachment(pending: PendingChatUpload): Promise<boolean> {
    try {
      const uploaded = await this.api.uploadAttachment(
        pending.workspaceId,
        'chat_message',
        pending.messageId,
        pending.file,
      );
      if (this.pendingUpload?.messageId === pending.messageId) this.pendingUpload = null;
      if (!this.isCurrentContext(pending.workspaceId, pending.conversationId)) return false;
      this.attachments.update((items) => {
        if (items.some((item) => item.id === uploaded.id)) return items;
        return [
          ...items,
          {
            id: uploaded.id,
            messageId: pending.messageId,
            displayName: uploaded.displayName,
            mediaType: uploaded.mediaType,
            sizeBytes: uploaded.sizeBytes,
            scanState: uploaded.scanState,
            createdAt: uploaded.createdAt,
          },
        ];
      });
      this.error.set(null);
      return true;
    } catch (error) {
      const discarded = await this.discardPendingUpload(pending);
      if (this.workspace.id() === pending.workspaceId) {
        this.error.set(error);
        this.toasts.show({ messageKey: 'chat.uploadFailed', messageParams: {} });
      }
      if (discarded) this.pendingUpload = null;
      return false;
    }
  }

  private async discardPendingUpload(pending: PendingChatUpload): Promise<boolean> {
    try {
      await this.api.deleteProvisionalChatMessage(pending.workspaceId, pending.messageId);
      if (this.isCurrentContext(pending.workspaceId, pending.conversationId)) {
        this.messages.update((items) => items.filter((item) => item.id !== pending.messageId));
      }
      if (this.pendingUpload?.messageId === pending.messageId) this.pendingUpload = null;
      return true;
    } catch {
      return false;
    }
  }
}

interface PendingChatUpload {
  readonly workspaceId: string;
  readonly conversationId: string;
  readonly messageId: string;
  readonly file: File;
  readonly body: string;
  readonly kind: 'text' | 'file' | 'voice' | 'video';
}

interface PendingChatSend {
  readonly workspaceId: string;
  readonly conversationId: string;
  readonly body: string;
  readonly kind: 'text' | 'file' | 'voice' | 'video';
  readonly idempotencyKey: string;
}

interface PendingCallCreate {
  readonly workspaceId: string;
  readonly conversationId: string;
  readonly kind: 'audio' | 'video';
  readonly idempotencyKey: string;
}
