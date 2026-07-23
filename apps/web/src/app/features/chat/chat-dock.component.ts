import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import type { CallJoin, ChatConversation, ChatMessage } from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ChatDockStore } from './chat-dock.store';
import { CallSessionService } from './call-session.service';

@Component({
  selector: 'app-chat-dock',
  imports: [ErrorPanelComponent, IconComponent, MatButtonModule],
  providers: [CallSessionService, ChatDockStore],
  template: `
    <button
      class="chat-launcher"
      type="button"
      (click)="toggle()"
      [attr.aria-label]="i18n.t('chat.open')"
    >
      <app-icon name="chat" />
      @if (store.unreadCount() > 0) {
        <span class="unread-badge">{{
          store.unreadCount() > 99 ? '99+' : store.unreadCount()
        }}</span>
      }
    </button>

    @if (open()) {
      <aside
        class="chat-dock"
        role="dialog"
        aria-modal="false"
        [attr.aria-label]="i18n.t('chat.title')"
      >
        <header class="dock-header">
          @if (store.activeConversation(); as conversation) {
            <button class="back-to-list" type="button" (click)="backToList()">
              <app-icon name="back" />
            </button>
            <div>
              <strong>{{ conversationName(conversation) }}</strong
              ><small>{{ memberSummary(conversation) }}</small>
            </div>
            <div class="call-actions">
              <button
                mat-icon-button
                type="button"
                [disabled]="callActionDisabled()"
                [title]="i18n.t(store.callsEnabled() ? 'chat.audioCall' : 'chat.callUnavailable')"
                (click)="startCall('audio')"
              >
                <app-icon name="phone" />
              </button>
              <button
                mat-icon-button
                type="button"
                [disabled]="callActionDisabled()"
                [title]="i18n.t(store.callsEnabled() ? 'chat.videoCall' : 'chat.callUnavailable')"
                (click)="startCall('video')"
              >
                <app-icon name="video" />
              </button>
            </div>
          } @else {
            <div>
              <strong>{{ i18n.t('chat.title') }}</strong
              ><small>{{ i18n.plural('common.resultCount', store.conversations().length) }}</small>
            </div>
            <button
              mat-icon-button
              type="button"
              (click)="openCreate()"
              [attr.aria-label]="i18n.t('chat.addDirect')"
            >
              <app-icon name="add" />
            </button>
          }
          <button
            mat-icon-button
            type="button"
            class="close-chat"
            (click)="closeDock()"
            [attr.aria-label]="i18n.t('chat.close')"
          >
            <app-icon name="close" />
          </button>
        </header>

        @if (store.error()) {
          <app-error-panel [error]="store.error()" (retry)="store.loadConversations()" />
        }

        @if (store.incomingCall(); as call) {
          <section class="incoming-call" aria-live="assertive">
            <app-icon [name]="call.kind === 'video' ? 'video' : 'phone'" />
            <span>
              <strong>{{
                i18n.t(call.kind === 'video' ? 'chat.incomingVideoCall' : 'chat.incomingAudioCall')
              }}</strong>
              <small>{{ i18n.t('chat.incomingCallHint') }}</small>
            </span>
            <button mat-button type="button" (click)="declineCall()">
              {{ i18n.t('chat.decline') }}
            </button>
            <button mat-flat-button type="button" (click)="acceptCall()">
              {{ i18n.t('chat.accept') }}
            </button>
          </section>
        }

        @if (displayedCall(); as activeCall) {
          <section class="call-panel" [attr.aria-label]="i18n.t('chat.activeCall')">
            <header>
              <strong>{{
                i18n.t(activeCall?.kind === 'video' ? 'chat.videoCall' : 'chat.audioCall')
              }}</strong>
              <span>{{
                i18n.t(
                  callSession.status() === 'connecting'
                    ? 'chat.callStatus.connecting'
                    : callSession.status() === 'connected'
                      ? 'chat.callStatus.connected'
                      : callSession.status() === 'failed'
                        ? 'chat.callStatus.failed'
                        : 'chat.callStatus.idle'
                )
              }}</span>
            </header>
            <div #callMedia class="call-media" [class.audio-only]="activeCall?.kind !== 'video'">
              @if (activeCall?.kind !== 'video') {
                <app-icon name="phone" />
              }
            </div>
            <div class="call-controls">
              <button mat-button type="button" (click)="callSession.toggleMicrophone()">
                {{ i18n.t(callSession.microphoneEnabled() ? 'chat.mute' : 'chat.unmute') }}
              </button>
              @if (activeCall?.kind === 'video') {
                <button mat-button type="button" (click)="callSession.toggleCamera()">
                  {{ i18n.t(callSession.cameraEnabled() ? 'chat.cameraOff' : 'chat.cameraOn') }}
                </button>
              }
              <button mat-flat-button type="button" class="hang-up" (click)="leaveCall()">
                {{ i18n.t('chat.hangUp') }}
              </button>
            </div>
          </section>
        }

        @if (createOpen()) {
          <section class="new-conversation">
            <label class="native-field"
              ><span>{{ i18n.t('chat.chooseMember') }}</span
              ><select
                [value]="selectedMemberId()"
                (change)="selectedMemberId.set(selectValue($event))"
              >
                <option value="">{{ i18n.t('chat.chooseMemberPlaceholder') }}</option>
                @for (member of availableMembers(); track member.userId) {
                  <option [value]="member.userId">{{ member.displayName }}</option>
                }
              </select></label
            >
            <div>
              <button mat-button type="button" (click)="closeCreate()">
                {{ i18n.t('common.action.cancel') }}</button
              ><button
                mat-flat-button
                type="button"
                [disabled]="!selectedMemberId() || store.sending()"
                (click)="startDirect(selectedMemberId())"
              >
                {{ i18n.t('common.action.create') }}
              </button>
            </div>
          </section>
        }

        @if (!store.activeConversationId()) {
          <div class="conversation-list" [attr.aria-busy]="store.loading()">
            @for (conversation of store.conversations(); track conversation.id) {
              <button type="button" (click)="selectConversation(conversation.id)">
                <span class="conversation-avatar">{{
                  initials(conversationName(conversation))
                }}</span>
                <span
                  ><strong>{{ conversationName(conversation) }}</strong
                  ><small>{{ memberSummary(conversation) }}</small></span
                >
                @if (conversation.unreadCount > 0) {
                  <span class="conversation-unread">{{ conversation.unreadCount }}</span>
                }
              </button>
            } @empty {
              @if (!store.loading()) {
                <div class="empty-state">{{ i18n.t('chat.empty') }}</div>
              }
            }
          </div>
        } @else {
          <div class="message-pane">
            @if (store.nextCursor()) {
              <button mat-button type="button" class="load-older" (click)="loadOlder()">
                {{ i18n.t('chat.loadOlder') }}
              </button>
            }
            <div class="message-list" [attr.aria-busy]="store.loading()">
              @for (message of store.messages(); track message.id) {
                <article [class.own]="message.senderUserId === auth.user()?.id">
                  <header>
                    <strong>{{ message.senderDisplayName }}</strong
                    ><time [attr.datetime]="message.createdAt">{{
                      i18n.date(message.createdAt, { hour: '2-digit', minute: '2-digit' })
                    }}</time>
                  </header>
                  <p>{{ message.body }}</p>
                  @for (attachment of attachmentsFor(message); track attachment.id) {
                    <button
                      type="button"
                      class="attachment"
                      [disabled]="attachment.scanState === 'rejected'"
                      (click)="store.download(attachment)"
                    >
                      <span>{{ attachment.displayName }}</span
                      ><small>{{ formatBytes(attachment.sizeBytes) }}</small>
                    </button>
                  }
                  <footer>
                    @if (message.pinned) {
                      <span class="pinned"><app-icon name="pin" />{{ i18n.t('chat.pinned') }}</span>
                    }
                    @for (reaction of groupedReactions(message); track reaction.emoji) {
                      <span class="reaction">{{ reaction.emoji }} {{ reaction.count }}</span>
                    }
                    <span class="message-actions">
                      @for (emoji of emojis; track emoji) {
                        <button
                          type="button"
                          (click)="store.react(message.id, emoji)"
                          [attr.aria-label]="i18n.t('chat.reaction', { emoji })"
                        >
                          {{ emoji }}
                        </button>
                      }
                      <button
                        type="button"
                        (click)="store.pin(message)"
                        [attr.aria-label]="i18n.t('chat.pin')"
                      >
                        <app-icon name="pin" /></button
                    ></span>
                  </footer>
                </article>
              } @empty {
                @if (!store.loading()) {
                  <div class="empty-state">{{ i18n.t('chat.emptyMessages') }}</div>
                }
              }
            </div>
            <form class="composer" (submit)="send($event)">
              <label class="attach-action" [title]="i18n.t('chat.attach')"
                ><input
                  type="file"
                  [disabled]="store.loading()"
                  (change)="chooseFile($event)" /><app-icon name="add"
              /></label>
              <textarea
                rows="1"
                [disabled]="store.loading()"
                [value]="draft()"
                (input)="draft.set(messageValue($event))"
                (keydown)="composerKeydown($event)"
                [placeholder]="i18n.t('chat.message')"
              ></textarea>
              <button
                mat-flat-button
                type="submit"
                [disabled]="
                  store.loading() || store.sending() || (!draft().trim() && !selectedFile())
                "
                [attr.aria-label]="i18n.t('chat.send')"
              >
                <app-icon name="send" />
              </button>
              @if (selectedFile(); as file) {
                <span class="selected-file"
                  >{{ file.name }}
                  <button type="button" (click)="selectedFile.set(null)">×</button></span
                >
              }
            </form>
          </div>
        }
      </aside>
    }
  `,
  styleUrl: './chat-dock.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChatDockComponent implements OnInit {
  readonly store = inject(ChatDockStore);
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  private readonly workspace = inject(WorkspaceStore);
  readonly callSession = this.store.callSession;
  readonly callMedia = viewChild<ElementRef<HTMLElement>>('callMedia');
  readonly open = signal(false);
  readonly createOpen = signal(false);
  readonly selectedMemberId = signal('');
  readonly draft = signal('');
  readonly selectedFile = signal<File | null>(null);
  readonly connectingGrant = signal<CallJoin | null>(null);
  readonly displayedCall = computed(
    () => this.callSession.activeCall() ?? this.connectingGrant()?.call ?? null,
  );
  readonly emojis = ['👍', '❤️', '🔥'] as const;
  readonly availableMembers = computed(() =>
    this.store.members().filter((member) => member.userId !== this.auth.user()?.id),
  );
  private workspaceId = this.workspace.id();
  private readonly workspaceReset = effect(() => {
    const workspaceId = this.workspace.id();
    if (workspaceId === this.workspaceId) return;
    this.workspaceId = workspaceId;
    this.open.set(false);
    this.createOpen.set(false);
    this.selectedMemberId.set('');
    this.draft.set('');
    this.selectedFile.set(null);
    this.connectingGrant.set(null);
  });

  ngOnInit(): void {
    void this.i18n.loadNamespaces(['chat']);
  }
  toggle(): void {
    if (this.open()) {
      void this.closeDock();
      return;
    }
    this.open.set(true);
    void this.store.loadConversations();
  }
  async closeDock(): Promise<void> {
    this.open.set(false);
    const grant = this.connectingGrant();
    this.connectingGrant.set(null);
    if (this.callSession.activeCall()) {
      await this.store.leaveActiveCall();
    } else if (grant) {
      await this.store.releaseJoinedCall(this.workspace.id(), grant);
    }
  }
  openCreate(): void {
    this.selectedMemberId.set('');
    this.createOpen.set(true);
    void this.store.loadMembers();
  }
  closeCreate(): void {
    this.selectedMemberId.set('');
    this.createOpen.set(false);
  }
  async startDirect(userId: string): Promise<void> {
    await this.store.startDirect(userId);
    this.clearComposer();
    this.closeCreate();
  }
  async selectConversation(conversationId: string): Promise<void> {
    this.clearComposer();
    await this.store.select(conversationId);
  }
  backToList(): void {
    this.clearComposer();
    this.store.activeConversationId.set(null);
  }
  callActionDisabled(): boolean {
    return (
      !this.store.callsEnabled() ||
      this.callSession.activeCall() !== null ||
      this.connectingGrant() !== null
    );
  }
  async startCall(kind: 'audio' | 'video'): Promise<void> {
    await this.connectCall(await this.store.startCall(kind));
  }
  async acceptCall(): Promise<void> {
    await this.connectCall(await this.store.acceptCall());
  }
  async declineCall(): Promise<void> {
    await this.store.declineCall();
  }
  async leaveCall(): Promise<void> {
    await this.store.leaveActiveCall();
    this.connectingGrant.set(null);
  }
  async loadOlder(): Promise<void> {
    const id = this.store.activeConversationId();
    if (id) await this.store.loadMessages(id, true);
  }
  async send(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!(await this.store.send(this.draft(), this.selectedFile()))) return;
    this.draft.set('');
    this.selectedFile.set(null);
  }
  composerKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      (event.currentTarget as HTMLTextAreaElement).form?.requestSubmit();
    }
  }
  chooseFile(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.selectedFile.set(input.files?.item(0) ?? null);
    input.value = '';
  }
  messageValue(event: Event): string {
    return (event.target as HTMLTextAreaElement).value;
  }
  selectValue(event: Event): string {
    return (event.target as HTMLSelectElement).value;
  }
  private clearComposer(): void {
    this.draft.set('');
    this.selectedFile.set(null);
  }
  conversationName(conversation: ChatConversation): string {
    if (conversation.title) return conversation.title;
    const others = conversation.members.filter((member) => member.userId !== this.auth.user()?.id);
    return others.map((member) => member.displayName).join(', ') || this.i18n.t('chat.group');
  }
  memberSummary(conversation: ChatConversation): string {
    return conversation.members.map((member) => member.displayName).join(', ');
  }
  initials(name: string): string {
    return name
      .split(/\s+/u)
      .slice(0, 2)
      .map((part) => part.slice(0, 1).toUpperCase())
      .join('');
  }
  attachmentsFor(message: ChatMessage) {
    return this.store.attachmentsByMessage().get(message.id) ?? [];
  }
  groupedReactions(message: ChatMessage): ReadonlyArray<{ emoji: string; count: number }> {
    const counts = new Map<string, number>();
    for (const reaction of message.reactions)
      counts.set(reaction.emoji, (counts.get(reaction.emoji) ?? 0) + 1);
    return [...counts].map(([emoji, count]) => ({ emoji, count }));
  }
  formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
  }

  private async connectCall(grant: CallJoin): Promise<void> {
    const workspaceId = this.workspace.id();
    this.connectingGrant.set(grant);
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    if (this.workspace.id() !== workspaceId || !this.open()) {
      this.connectingGrant.set(null);
      await this.store.releaseJoinedCall(workspaceId, grant);
      return;
    }
    const host = this.callMedia()?.nativeElement;
    if (!host) {
      this.connectingGrant.set(null);
      await this.store.releaseJoinedCall(workspaceId, grant);
      return;
    }
    try {
      await this.callSession.connect(grant, host);
      if (
        this.workspace.id() !== workspaceId ||
        !this.open() ||
        !host.isConnected ||
        this.callSession.status() !== 'connected'
      ) {
        this.callSession.disconnect();
        await this.store.releaseJoinedCall(workspaceId, grant);
      }
    } catch (error) {
      await this.store.releaseJoinedCall(workspaceId, grant);
      throw error;
    } finally {
      if (this.connectingGrant()?.call.id === grant.call.id) this.connectingGrant.set(null);
    }
  }
}
