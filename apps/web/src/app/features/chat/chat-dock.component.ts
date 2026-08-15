import { CdkTrapFocus } from '@angular/cdk/a11y';
import type { ElementRef, OnDestroy, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  Injector,
  computed,
  effect,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import type { MatSelect } from '@angular/material/select';
import { MatSelectModule } from '@angular/material/select';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  CallJoin,
  ChatAttachment,
  ChatConversation,
  ChatMessage,
} from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { CallSessionService } from './call-session.service';
import { ChatDockStore } from './chat-dock.store';

@Component({
  selector: 'app-chat-dock',
  imports: [
    CdkTrapFocus,
    ErrorPanelComponent,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatSelectModule,
  ],
  providers: [CallSessionService, ChatDockStore],
  template: `
    @if (!open()) {
      <button
        #chatLauncher
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
    }

    @if (open()) {
      <aside
        class="chat-dock"
        role="dialog"
        [attr.aria-modal]="mobileViewport()"
        [cdkTrapFocus]="mobileViewport()"
        [cdkTrapFocusAutoCapture]="mobileViewport()"
        [attr.aria-label]="i18n.t('chat.title')"
      >
        <header class="dock-header">
          @if (store.activeConversation(); as conversation) {
            <button
              class="icon-button back-to-list"
              type="button"
              (click)="backToList()"
              [attr.aria-label]="i18n.t('chat.title')"
            >
              <app-icon name="back" />
            </button>
            <span class="conversation-avatar compact">{{
              initials(conversationName(conversation))
            }}</span>
            <div class="conversation-heading">
              <strong>{{ conversationName(conversation) }}</strong>
              <small>{{ memberSummary(conversation) }}</small>
            </div>
            <div class="call-actions">
              <button
                class="icon-button"
                type="button"
                [disabled]="callActionDisabled()"
                [title]="i18n.t(store.callsEnabled() ? 'chat.audioCall' : 'chat.callUnavailable')"
                (click)="startCall('audio')"
              >
                <app-icon name="phone" />
              </button>
              <button
                class="icon-button"
                type="button"
                [disabled]="callActionDisabled()"
                [title]="i18n.t(store.callsEnabled() ? 'chat.videoCall' : 'chat.callUnavailable')"
                (click)="startCall('video')"
              >
                <app-icon name="video" />
              </button>
            </div>
          } @else {
            <div class="conversation-heading root-heading">
              <strong>{{ i18n.t('chat.title') }}</strong>
              <small>{{ i18n.plural('common.resultCount', store.conversations().length) }}</small>
            </div>
            <button
              #newChatButton
              class="icon-button new-chat"
              type="button"
              (click)="openCreate()"
              [attr.aria-label]="i18n.t('chat.addDirect')"
            >
              <app-icon name="add" />
            </button>
          }
          <button
            class="icon-button close-chat"
            type="button"
            (click)="closeDock()"
            [attr.aria-label]="i18n.t('chat.close')"
          >
            <app-icon name="close" />
          </button>
        </header>

        @if (store.error()) {
          <app-error-panel [error]="store.error()" (retry)="retryActiveState()" />
        }

        @if (store.incomingCall(); as call) {
          <section class="incoming-call" aria-live="assertive">
            <span class="incoming-call-icon"
              ><app-icon [name]="call.kind === 'video' ? 'video' : 'phone'"
            /></span>
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
                i18n.t(activeCall.kind === 'video' ? 'chat.videoCall' : 'chat.audioCall')
              }}</strong>
              <span>{{ callStatusLabel() }}</span>
            </header>
            <div #callMedia class="call-media" [class.audio-only]="activeCall.kind !== 'video'">
              @if (activeCall.kind !== 'video') {
                <span class="call-avatar"><app-icon name="phone" /></span>
              }
            </div>
            <div class="call-controls">
              <button mat-button type="button" (click)="callSession.toggleMicrophone()">
                {{ i18n.t(callSession.microphoneEnabled() ? 'chat.mute' : 'chat.unmute') }}
              </button>
              @if (activeCall.kind === 'video') {
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
          <section
            class="new-conversation"
            role="dialog"
            aria-modal="true"
            cdkTrapFocus
            [cdkTrapFocusAutoCapture]="true"
            [attr.aria-label]="i18n.t('chat.addDirect')"
          >
            <header>
              <div>
                <strong>{{ i18n.t('chat.addDirect') }}</strong>
                <small>{{ i18n.t('chat.chooseMember') }}</small>
              </div>
              <button
                class="icon-button"
                type="button"
                (click)="closeCreate()"
                [attr.aria-label]="i18n.t('common.action.cancel')"
              >
                <app-icon name="close" />
              </button>
            </header>
            <mat-form-field appearance="outline" subscriptSizing="dynamic">
              <mat-label>{{ i18n.t('chat.chooseMember') }}</mat-label>
              <mat-select
                panelClass="crm-select-panel"
                #memberSelect
                [value]="selectedMemberId()"
                (selectionChange)="selectedMemberId.set($event.value)"
              >
                @for (member of availableMembers(); track member.userId) {
                  <mat-option [value]="member.userId">{{ member.displayName }}</mat-option>
                }
              </mat-select>
            </mat-form-field>
            @if (store.loading()) {
              <div class="loading-state" aria-hidden="true"><app-icon name="clock" /></div>
            }
            <footer>
              <button mat-button type="button" (click)="closeCreate()">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button
                mat-flat-button
                type="button"
                [disabled]="!selectedMemberId() || store.sending()"
                (click)="startDirect(selectedMemberId())"
              >
                {{ i18n.t('common.action.create') }}
              </button>
            </footer>
          </section>
        }

        @if (!store.activeConversationId()) {
          <div class="conversation-list" [attr.aria-busy]="store.loading()">
            @if (store.loading() && store.conversations().length === 0) {
              <div class="loading-state" aria-hidden="true"><app-icon name="clock" /></div>
            } @else {
              @for (conversation of store.conversations(); track conversation.id) {
                <button type="button" (click)="selectConversation(conversation.id)">
                  <span class="conversation-avatar">{{
                    initials(conversationName(conversation))
                  }}</span>
                  <span class="conversation-copy">
                    <strong>{{ conversationName(conversation) }}</strong>
                    <small>{{ memberSummary(conversation) }}</small>
                  </span>
                  @if (conversation.unreadCount > 0) {
                    <span class="conversation-unread">{{ conversation.unreadCount }}</span>
                  } @else if (conversation.lastMessageAt) {
                    <time [attr.datetime]="conversation.lastMessageAt">{{
                      i18n.date(conversation.lastMessageAt, { hour: '2-digit', minute: '2-digit' })
                    }}</time>
                  }
                </button>
              } @empty {
                <div class="empty-state">
                  <span class="empty-state-icon"><app-icon name="chat" /></span>
                  <strong>{{ i18n.t('chat.empty') }}</strong>
                  <button mat-flat-button type="button" (click)="openCreate()">
                    <app-icon name="add" />{{ i18n.t('chat.addDirect') }}
                  </button>
                </div>
              }
            }
          </div>
        } @else {
          <div class="message-pane">
            @if (pinnedMessages()[0]; as pinned) {
              <button class="pinned-strip" type="button" (click)="scrollToMessage(pinned.id)">
                <span><app-icon name="pin" /></span>
                <span
                  ><strong>{{ i18n.t('chat.pinned') }}</strong
                  ><small>{{ pinned.body }}</small></span
                >
                @if (pinnedMessages().length > 1) {
                  <b>{{ pinnedMessages().length }}</b>
                }
              </button>
            }
            @if (store.nextCursor()) {
              <button mat-button type="button" class="load-older" (click)="loadOlder()">
                {{ i18n.t('chat.loadOlder') }}
              </button>
            }
            <div
              class="message-list"
              role="log"
              aria-live="polite"
              aria-relevant="additions"
              [attr.aria-busy]="store.loading()"
            >
              @if (store.loading() && store.messages().length === 0) {
                <div class="loading-state" aria-hidden="true"><app-icon name="clock" /></div>
              } @else {
                @for (message of store.messages(); track message.id; let index = $index) {
                  <article
                    class="message-row"
                    [id]="'chat-message-' + message.id"
                    [class.own]="isOwn(message)"
                    [class.grouped]="isGrouped(message, index)"
                  >
                    @if (!isOwn(message) && !isGrouped(message, index)) {
                      <span class="message-avatar">{{ initials(message.senderDisplayName) }}</span>
                    }
                    <div class="message-content">
                      @if (!isGrouped(message, index)) {
                        <header>
                          @if (!isOwn(message)) {
                            <strong>{{ message.senderDisplayName }}</strong>
                          }
                          <time [attr.datetime]="message.createdAt">{{
                            i18n.date(message.createdAt, { hour: '2-digit', minute: '2-digit' })
                          }}</time>
                        </header>
                      }
                      <div class="message-bubble">
                        @if (message.body) {
                          <p>{{ message.body }}</p>
                        }
                        @for (attachment of attachmentsFor(message); track attachment.id) {
                          @if (isAudio(attachment) && mediaUrl(attachment.id)) {
                            <div class="media-card voice-card">
                              <span class="media-kind"><app-icon name="microphone" /></span>
                              <audio
                                controls
                                preload="metadata"
                                [src]="mediaUrl(attachment.id)"
                              ></audio>
                              <button
                                class="media-download"
                                type="button"
                                (click)="store.download(attachment)"
                                [attr.aria-label]="i18n.t('chat.attach')"
                              >
                                <app-icon name="download" />
                              </button>
                            </div>
                          } @else if (isVideo(attachment) && mediaUrl(attachment.id)) {
                            <div class="media-card video-card">
                              <video
                                controls
                                preload="metadata"
                                playsinline
                                [src]="mediaUrl(attachment.id)"
                              ></video>
                              <button
                                class="media-download overlay"
                                type="button"
                                (click)="store.download(attachment)"
                                [attr.aria-label]="i18n.t('chat.attach')"
                              >
                                <app-icon name="download" />
                              </button>
                            </div>
                          } @else if (isAudio(attachment) || isVideo(attachment)) {
                            <button
                              type="button"
                              class="media-placeholder"
                              [disabled]="mediaLoading().has(attachment.id)"
                              (click)="loadMedia(attachment)"
                            >
                              <span
                                ><app-icon [name]="isVideo(attachment) ? 'video' : 'play'"
                              /></span>
                              <span
                                ><strong>{{ attachment.displayName }}</strong
                                ><small>{{ i18n.t('chat.loadMedia') }}</small></span
                              >
                              <small>{{ formatBytes(attachment.sizeBytes) }}</small>
                            </button>
                          } @else {
                            <button
                              type="button"
                              class="file-attachment"
                              [disabled]="attachment.scanState === 'rejected'"
                              (click)="store.download(attachment)"
                            >
                              <span><app-icon name="file" /></span>
                              <span
                                ><strong>{{ attachment.displayName }}</strong
                                ><small>{{ formatBytes(attachment.sizeBytes) }}</small></span
                              >
                              <app-icon name="download" />
                            </button>
                          }
                        }
                      </div>
                      <footer>
                        <span class="message-meta">
                          @if (message.pinned) {
                            <app-icon name="pin" />
                          }
                          @if (isOwn(message)) {
                            <span [title]="i18n.t('chat.accepted')">
                              <app-icon name="check" />
                              <span class="visually-hidden">{{ i18n.t('chat.accepted') }}</span>
                            </span>
                          }
                        </span>
                        @for (reaction of groupedReactions(message); track reaction.emoji) {
                          <button
                            class="reaction-chip"
                            type="button"
                            (click)="store.react(message.id, reaction.emoji)"
                            [attr.aria-label]="i18n.t('chat.reaction', { emoji: reaction.emoji })"
                          >
                            {{ reaction.emoji }} <span>{{ reaction.count }}</span>
                          </button>
                        }
                        <span class="message-actions">
                          <button
                            type="button"
                            (click)="store.react(message.id, likeEmoji)"
                            [attr.aria-label]="i18n.t('chat.reaction', { emoji: likeEmoji })"
                          >
                            <app-icon name="like" />
                          </button>
                          <button
                            type="button"
                            (click)="store.react(message.id, heartEmoji)"
                            [attr.aria-label]="i18n.t('chat.reaction', { emoji: heartEmoji })"
                          >
                            <app-icon name="reaction" />
                          </button>
                          <button
                            type="button"
                            (click)="store.pin(message)"
                            [attr.aria-label]="i18n.t('chat.pin')"
                          >
                            <app-icon name="pin" />
                          </button>
                        </span>
                      </footer>
                    </div>
                  </article>
                } @empty {
                  <div class="empty-state messages-empty">
                    <span class="empty-state-icon"><app-icon name="chat" /></span>
                    <strong>{{ i18n.t('chat.emptyMessages') }}</strong>
                  </div>
                }
              }
            </div>
            <form class="composer" (submit)="send($event)">
              @if (selectedFile(); as file) {
                <div class="selected-file">
                  <span><app-icon name="file" /></span>
                  <strong>{{ file.name }}</strong>
                  <small>{{ formatBytes(file.size) }}</small>
                  <button
                    type="button"
                    (click)="selectedFile.set(null)"
                    [attr.aria-label]="i18n.t('common.action.cancel')"
                  >
                    <app-icon name="close" />
                  </button>
                </div>
              }
              <div class="composer-box">
                <label class="composer-action" [title]="i18n.t('chat.attach')">
                  <input
                    type="file"
                    [disabled]="store.loading()"
                    [attr.aria-label]="i18n.t('chat.attach')"
                    (change)="chooseFile($event)"
                  />
                  <app-icon name="attachment" />
                </label>
                <textarea
                  #messageComposer
                  rows="1"
                  [disabled]="store.loading()"
                  [value]="draft()"
                  (input)="draft.set(messageValue($event))"
                  (keydown)="composerKeydown($event)"
                  [placeholder]="i18n.t('chat.message')"
                ></textarea>
                <button
                  class="send-action"
                  type="submit"
                  [disabled]="
                    store.loading() || store.sending() || (!draft().trim() && !selectedFile())
                  "
                  [attr.aria-label]="i18n.t('chat.send')"
                >
                  <app-icon [name]="store.sending() ? 'clock' : 'send'" />
                </button>
              </div>
            </form>
          </div>
        }
      </aside>
    }
  `,
  styleUrl: './chat-dock.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChatDockComponent implements OnInit, OnDestroy {
  readonly store = inject(ChatDockStore);
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  private readonly workspace = inject(WorkspaceStore);
  private readonly api = inject(ApiClient);
  private readonly injector = inject(Injector);
  readonly callSession = this.store.callSession;
  readonly callMedia = viewChild<ElementRef<HTMLElement>>('callMedia');
  readonly chatLauncher = viewChild<ElementRef<HTMLButtonElement>>('chatLauncher');
  readonly newChatButton = viewChild<ElementRef<HTMLButtonElement>>('newChatButton');
  readonly memberSelect = viewChild<MatSelect>('memberSelect');
  readonly messageComposer = viewChild<ElementRef<HTMLTextAreaElement>>('messageComposer');
  readonly open = signal(false);
  readonly createOpen = signal(false);
  readonly mobileViewport = signal(false);
  readonly selectedMemberId = signal('');
  readonly draft = signal('');
  readonly selectedFile = signal<File | null>(null);
  readonly connectingGrant = signal<CallJoin | null>(null);
  readonly mediaUrls = signal<ReadonlyMap<string, string>>(new Map());
  readonly mediaLoading = signal<ReadonlySet<string>>(new Set());
  readonly displayedCall = computed(
    () => this.callSession.activeCall() ?? this.connectingGrant()?.call ?? null,
  );
  readonly pinnedMessages = computed(() =>
    this.store.messages().filter((message) => message.pinned),
  );
  readonly availableMembers = computed(() =>
    this.store.members().filter((member) => member.userId !== this.auth.user()?.id),
  );
  readonly likeEmoji = '\u{1F44D}';
  readonly heartEmoji = '\u2764\uFE0F';
  private workspaceId = this.workspace.id();
  private mobileQuery: MediaQueryList | null = null;
  private readonly mobileQueryListener = (event: MediaQueryListEvent): void => {
    this.mobileViewport.set(event.matches);
  };
  private readonly workspaceReset = effect(() => {
    const workspaceId = this.workspace.id();
    if (workspaceId === this.workspaceId) return;
    this.workspaceId = workspaceId;
    this.open.set(false);
    this.createOpen.set(false);
    this.selectedMemberId.set('');
    this.clearComposer();
    this.connectingGrant.set(null);
    this.clearMediaUrls();
  });

  ngOnInit(): void {
    void this.i18n.loadNamespaces(['chat']);
    if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      this.mobileQuery = window.matchMedia('(max-width: 560px)');
      this.mobileViewport.set(this.mobileQuery.matches);
      this.mobileQuery.addEventListener('change', this.mobileQueryListener);
    }
  }
  ngOnDestroy(): void {
    this.mobileQuery?.removeEventListener('change', this.mobileQueryListener);
    this.clearMediaUrls();
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
    this.clearMediaUrls();
    focusAfterNextRender(this.injector, () => this.chatLauncher()?.nativeElement);
    const grant = this.connectingGrant();
    this.connectingGrant.set(null);
    if (this.callSession.activeCall()) await this.store.leaveActiveCall();
    else if (grant) await this.store.releaseJoinedCall(this.workspace.id(), grant);
  }
  openCreate(): void {
    this.selectedMemberId.set('');
    this.createOpen.set(true);
    void this.store.loadMembers();
    focusAfterNextRender(this.injector, () => this.memberSelect());
  }
  closeCreate(restoreFocus = true): void {
    this.selectedMemberId.set('');
    this.createOpen.set(false);
    if (restoreFocus)
      focusAfterNextRender(this.injector, () => this.newChatButton()?.nativeElement);
  }
  async startDirect(userId: string): Promise<void> {
    await this.store.startDirect(userId);
    this.clearComposer();
    this.closeCreate(false);
    focusAfterNextRender(this.injector, () => this.messageComposer()?.nativeElement);
  }
  async selectConversation(conversationId: string): Promise<void> {
    this.clearComposer();
    this.clearMediaUrls();
    await this.store.select(conversationId);
  }
  backToList(): void {
    this.clearComposer();
    this.clearMediaUrls();
    this.store.activeConversationId.set(null);
  }
  retryActiveState(): void {
    const conversationId = this.store.activeConversationId();
    if (conversationId) void this.store.loadMessages(conversationId);
    else void this.store.loadConversations();
  }
  callActionDisabled(): boolean {
    return (
      !this.store.callsEnabled() ||
      this.callSession.activeCall() !== null ||
      this.connectingGrant() !== null
    );
  }
  callStatusLabel(): string {
    const key =
      this.callSession.status() === 'connecting'
        ? 'chat.callStatus.connecting'
        : this.callSession.status() === 'connected'
          ? 'chat.callStatus.connected'
          : this.callSession.status() === 'failed'
            ? 'chat.callStatus.failed'
            : 'chat.callStatus.idle';
    return this.i18n.t(key);
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
  isOwn(message: ChatMessage): boolean {
    return message.senderUserId === this.auth.user()?.id;
  }
  isGrouped(message: ChatMessage, index: number): boolean {
    const previous = this.store.messages()[index - 1];
    if (!previous || previous.senderUserId !== message.senderUserId) return false;
    return (
      new Date(message.createdAt).getTime() - new Date(previous.createdAt).getTime() < 5 * 60 * 1000
    );
  }
  attachmentsFor(message: ChatMessage): readonly ChatAttachment[] {
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
  isAudio(attachment: ChatAttachment): boolean {
    return attachment.mediaType.startsWith('audio/');
  }
  isVideo(attachment: ChatAttachment): boolean {
    return attachment.mediaType.startsWith('video/');
  }
  mediaUrl(id: string): string {
    return this.mediaUrls().get(id) ?? '';
  }
  async loadMedia(attachment: ChatAttachment): Promise<void> {
    const workspaceId = this.workspace.id();
    const conversationId = this.store.activeConversationId();
    if (
      !workspaceId ||
      !conversationId ||
      this.mediaUrls().has(attachment.id) ||
      this.mediaLoading().has(attachment.id)
    )
      return;
    this.mediaLoading.update((items) => new Set(items).add(attachment.id));
    try {
      const blob = await this.api.downloadAttachment(workspaceId, attachment.id);
      if (
        this.workspace.id() !== workspaceId ||
        this.store.activeConversationId() !== conversationId
      )
        return;
      this.mediaUrls.update((items) =>
        new Map(items).set(attachment.id, URL.createObjectURL(blob)),
      );
    } catch (error) {
      if (
        this.workspace.id() === workspaceId &&
        this.store.activeConversationId() === conversationId
      )
        this.store.error.set(error);
    } finally {
      this.mediaLoading.update((items) => {
        const next = new Set(items);
        next.delete(attachment.id);
        return next;
      });
    }
  }

  @HostListener('document:keydown.escape', ['$event'])
  closeOverlayFromKeyboard(event: Event): void {
    const escapedSelectOverlay = event
      .composedPath()
      .some((node) => node instanceof HTMLElement && node.closest('.cdk-overlay-pane'));
    if (this.memberSelect()?.panelOpen || escapedSelectOverlay) return;
    if (this.createOpen()) this.closeCreate();
    else if (this.open()) void this.closeDock();
  }
  scrollToMessage(messageId: string): void {
    document.getElementById(`chat-message-${messageId}`)?.scrollIntoView({ block: 'center' });
  }

  private clearComposer(): void {
    this.draft.set('');
    this.selectedFile.set(null);
  }
  private clearMediaUrls(): void {
    for (const url of this.mediaUrls().values()) URL.revokeObjectURL(url);
    this.mediaUrls.set(new Map());
    this.mediaLoading.set(new Set());
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
