import type { OnDestroy } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { ApiClient } from '../../core/api/api-client.service';
import type { ChatAttachment, ChatMessage } from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { CallSessionService } from './call-session.service';
import { ChatDockStore } from './chat-dock.store';

@Component({
  selector: 'app-entity-chat',
  imports: [ErrorPanelComponent, IconComponent, MatButtonModule],
  providers: [CallSessionService, ChatDockStore],
  template: `
    <section class="entity-chat panel" aria-labelledby="entity-chat-title">
      <header class="entity-header">
        <div>
          <h2 id="entity-chat-title">{{ i18n.t('chat.entityTitle') }}</h2>
          <p>{{ i18n.t('chat.entitySubtitle') }}</p>
        </div>
        @if (recording()) {
          <button mat-flat-button type="button" class="stop-recording" (click)="stopRecording()">
            <app-icon name="pause" />{{ i18n.t('chat.stopRecording') }}
          </button>
        }
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="initialize()" />
      }

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

      <div
        class="messages"
        role="log"
        aria-live="polite"
        aria-relevant="additions"
        [attr.aria-busy]="store.loading()"
      >
        @if (store.nextCursor()) {
          <button mat-button type="button" class="load-older" (click)="loadOlder()">
            {{ i18n.t('chat.loadOlder') }}
          </button>
        }
        @if (store.loading() && store.messages().length === 0) {
          <div class="loading-state" aria-hidden="true"><app-icon name="clock" /></div>
        } @else {
          @for (message of store.messages(); track message.id; let index = $index) {
            <article
              class="message-row"
              [id]="'entity-chat-message-' + message.id"
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
                      i18n.date(message.createdAt, { dateStyle: 'short', timeStyle: 'short' })
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
                        <audio controls preload="metadata" [src]="mediaUrl(attachment.id)"></audio>
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
                        <span><app-icon [name]="isVideo(attachment) ? 'video' : 'play'" /></span>
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
            <div class="empty-state">
              <span><app-icon name="chat" /></span>
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
              [disabled]="!store.activeConversationId() || store.loading()"
              [attr.aria-label]="i18n.t('chat.attach')"
              (change)="chooseFile($event)"
            />
            <app-icon name="attachment" />
          </label>
          <button
            class="composer-action"
            type="button"
            [disabled]="!store.activeConversationId() || store.loading() || !!recording()"
            [title]="i18n.t('chat.recordVoice')"
            (click)="startRecording('voice')"
          >
            <app-icon name="microphone" />
          </button>
          <button
            class="composer-action"
            type="button"
            [disabled]="!store.activeConversationId() || store.loading() || !!recording()"
            [title]="i18n.t('chat.recordVideo')"
            (click)="startRecording('video')"
          >
            <app-icon name="video" />
          </button>
          <textarea
            rows="1"
            [value]="draft()"
            (input)="draft.set(textValue($event))"
            (keydown)="composerKeydown($event)"
            [placeholder]="i18n.t('chat.message')"
          ></textarea>
          <button
            class="send-action"
            type="submit"
            [disabled]="
              !store.activeConversationId() ||
              store.loading() ||
              store.sending() ||
              (!draft().trim() && !selectedFile())
            "
            [attr.aria-label]="i18n.t('chat.send')"
          >
            <app-icon [name]="store.sending() ? 'clock' : 'send'" />
          </button>
        </div>
      </form>
    </section>
  `,
  styles: `
    .entity-chat {
      --chat-ease: cubic-bezier(0.23, 1, 0.32, 1);
      overflow: hidden;
      background: var(--surface-raised);
    }
    .entity-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 1rem 1.1rem;
      border-bottom: 1px solid var(--border);
    }
    h2,
    .entity-header p {
      margin: 0;
    }
    h2 {
      font-size: 1rem;
    }
    .entity-header p {
      max-width: 48rem;
      margin-top: 0.25rem;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .stop-recording {
      --mdc-filled-button-container-color: var(--danger);
      --mdc-filled-button-label-text-color: white;
    }
    .stop-recording app-icon {
      margin-right: 0.35rem;
    }
    .pinned-strip {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto;
      width: 100%;
      align-items: center;
      gap: 0.6rem;
      padding: 0.52rem 1rem;
      border: 0;
      border-bottom: 1px solid var(--border);
      color: var(--text);
      background: var(--surface-raised);
      text-align: left;
      cursor: pointer;
    }
    .pinned-strip > span:first-child {
      display: grid;
      width: 1.9rem;
      height: 1.9rem;
      place-items: center;
      border-radius: 50%;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .pinned-strip > span:nth-child(2) {
      min-width: 0;
    }
    .pinned-strip strong,
    .pinned-strip small {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .pinned-strip strong {
      color: var(--brand);
      font-size: 0.66rem;
    }
    .pinned-strip small {
      margin-top: 0.08rem;
      color: var(--text-muted);
      font-size: 0.7rem;
    }
    .pinned-strip b {
      display: grid;
      min-width: 1.4rem;
      height: 1.4rem;
      place-items: center;
      border-radius: 999px;
      background: var(--surface-subtle);
      font-size: 0.62rem;
    }
    .messages {
      display: flex;
      min-height: 20rem;
      max-height: 38rem;
      flex-direction: column;
      gap: 0.7rem;
      overflow: auto;
      padding: 1rem;
      background: var(--surface-subtle);
      scroll-behavior: smooth;
    }
    .load-older {
      align-self: center;
    }
    .message-row {
      display: grid;
      grid-template-columns: 2.15rem minmax(0, 1fr);
      align-self: flex-start;
      width: min(76%, 42rem);
      gap: 0.5rem;
    }
    .message-row.own {
      grid-template-columns: minmax(0, 1fr);
      align-self: flex-end;
    }
    .message-row.grouped {
      margin-top: -0.42rem;
    }
    .message-row.grouped:not(.own) {
      padding-left: 2.65rem;
      grid-template-columns: minmax(0, 1fr);
    }
    .message-avatar {
      display: grid;
      width: 2.15rem;
      height: 2.15rem;
      place-items: center;
      margin-top: 1.2rem;
      border-radius: 50%;
      color: var(--brand);
      background: var(--brand-soft);
      font-size: 0.62rem;
      font-weight: 780;
    }
    .message-content {
      min-width: 0;
    }
    .message-content > header {
      display: flex;
      min-height: 1.2rem;
      align-items: center;
      gap: 0.6rem;
      padding: 0 0.15rem 0.2rem;
    }
    .message-row.own .message-content > header {
      justify-content: flex-end;
    }
    .message-content > header strong {
      font-size: 0.68rem;
    }
    .message-content > header time {
      margin-left: auto;
      color: var(--text-faint);
      font-size: 0.6rem;
    }
    .message-row.own .message-content > header time {
      margin-left: 0;
    }
    .message-bubble {
      overflow: hidden;
      padding: 0.68rem 0.78rem;
      border: 1px solid var(--border);
      border-radius: 0.3rem 0.85rem 0.85rem;
      background: var(--surface-raised);
    }
    .message-row.own .message-bubble {
      border-color: color-mix(in srgb, var(--brand) 16%, var(--border));
      border-radius: 0.85rem 0.3rem 0.85rem 0.85rem;
      background: var(--brand-soft);
    }
    .message-row.grouped .message-bubble {
      border-radius: 0.75rem;
    }
    .message-bubble p {
      margin: 0;
      line-height: 1.48;
      overflow-wrap: anywhere;
      white-space: pre-wrap;
    }
    .message-content > footer {
      display: flex;
      min-height: 1.75rem;
      align-items: center;
      gap: 0.28rem;
      padding-top: 0.22rem;
    }
    .message-meta {
      display: inline-flex;
      gap: 0.15rem;
      color: var(--text-faint);
    }
    .message-row.own .message-meta {
      margin-left: auto;
    }
    .message-meta app-icon {
      width: 0.75rem;
      height: 0.75rem;
    }
    .reaction-chip {
      display: inline-flex;
      height: 1.55rem;
      align-items: center;
      gap: 0.2rem;
      padding: 0 0.42rem;
      border: 1px solid var(--border);
      border-radius: 999px;
      color: var(--text);
      background: var(--surface-raised);
      cursor: pointer;
    }
    .reaction-chip span {
      color: var(--text-muted);
      font-size: 0.64rem;
    }
    .message-actions {
      display: flex;
      margin-left: auto;
      padding: 0.12rem;
      border: 1px solid var(--border);
      border-radius: 0.55rem;
      background: var(--surface-raised);
      box-shadow: var(--shadow-sm);
      opacity: 0;
      transform: translateY(0.15rem);
      pointer-events: none;
      transition:
        opacity 140ms ease,
        transform 140ms var(--chat-ease);
    }
    .message-row:hover .message-actions,
    .message-row:focus-within .message-actions {
      opacity: 1;
      transform: none;
      pointer-events: auto;
    }
    .message-actions button {
      display: grid;
      width: 1.7rem;
      height: 1.7rem;
      place-items: center;
      padding: 0;
      border: 0;
      border-radius: 0.4rem;
      color: var(--text-muted);
      background: transparent;
      cursor: pointer;
    }
    .message-actions button:hover {
      color: var(--text);
      background: var(--surface-subtle);
    }
    .message-actions app-icon {
      width: 0.86rem;
      height: 0.86rem;
    }
    .loading-state {
      display: grid;
      min-height: 8rem;
      place-items: center;
      color: var(--text-faint);
    }
    .empty-state {
      display: grid;
      min-height: 18rem;
      place-content: center;
      justify-items: center;
      gap: 0.7rem;
      color: var(--text-muted);
    }
    .empty-state > span {
      display: grid;
      width: 3rem;
      height: 3rem;
      place-items: center;
      border-radius: 50%;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .empty-state strong {
      color: var(--text);
      font-size: 0.85rem;
    }
    .file-attachment,
    .media-placeholder {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto;
      width: min(100%, 28rem);
      align-items: center;
      gap: 0.55rem;
      margin-top: 0.55rem;
      padding: 0.55rem;
      border: 1px solid var(--border);
      border-radius: 0.65rem;
      color: var(--text);
      background: var(--surface-raised);
      text-align: left;
      cursor: pointer;
    }
    .file-attachment > span:first-child,
    .media-placeholder > span:first-child,
    .media-kind {
      display: grid;
      width: 2.1rem;
      height: 2.1rem;
      place-items: center;
      border-radius: 0.5rem;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .file-attachment strong,
    .file-attachment small,
    .media-placeholder strong,
    .media-placeholder small {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .file-attachment strong,
    .media-placeholder strong {
      font-size: 0.7rem;
    }
    .file-attachment small,
    .media-placeholder small {
      color: var(--text-faint);
      font-size: 0.62rem;
    }
    .media-card {
      position: relative;
      width: min(100%, 32rem);
      margin-top: 0.55rem;
      overflow: hidden;
      border: 1px solid var(--border);
      border-radius: 0.7rem;
      background: var(--surface-raised);
    }
    .voice-card {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.5rem;
      padding: 0.48rem;
    }
    .voice-card audio {
      width: 100%;
      min-width: 0;
      height: 2.3rem;
    }
    .video-card video {
      display: block;
      width: 100%;
      max-height: 24rem;
      background: #090d12;
      object-fit: contain;
    }
    .media-download {
      display: grid;
      width: 2rem;
      height: 2rem;
      place-items: center;
      padding: 0;
      border: 0;
      border-radius: 50%;
      color: var(--text-muted);
      background: var(--surface-subtle);
      cursor: pointer;
    }
    .media-download.overlay {
      position: absolute;
      top: 0.55rem;
      right: 0.55rem;
      color: white;
      background: rgb(9 13 18 / 0.72);
    }
    .composer {
      padding: 0.7rem;
      border-top: 1px solid var(--border);
      background: var(--surface-raised);
    }
    .composer-box {
      display: grid;
      grid-template-columns: auto auto auto minmax(8rem, 1fr) auto;
      align-items: end;
      gap: 0.3rem;
      padding: 0.3rem;
      border: 1px solid var(--border);
      border-radius: 0.85rem;
      background: var(--surface-subtle);
    }
    .composer-box:focus-within {
      border-color: var(--brand);
      outline: 2px solid var(--focus);
      outline-offset: 1px;
    }
    .composer textarea {
      min-height: 2.35rem;
      max-height: 7rem;
      resize: none;
      padding: 0.53rem 0.4rem;
      border: 0;
      outline: 0;
      color: var(--text);
      background: transparent;
      font: inherit;
    }
    .composer-action,
    .send-action {
      display: grid;
      width: 2.35rem;
      height: 2.35rem;
      place-items: center;
      padding: 0;
      border: 0;
      border-radius: 50%;
      cursor: pointer;
      transition:
        transform 140ms var(--chat-ease),
        background-color 140ms ease;
    }
    .composer-action {
      color: var(--text-muted);
      background: transparent;
    }
    .composer-action:hover:not(:disabled) {
      background: var(--surface-raised);
    }
    .send-action {
      color: white;
      background: var(--brand);
    }
    .composer-action:disabled,
    .send-action:disabled {
      opacity: 0.38;
      cursor: not-allowed;
    }
    .composer-action:active:not(:disabled),
    .send-action:active:not(:disabled) {
      transform: scale(0.94);
    }
    .composer-action input {
      position: absolute;
      width: 1px;
      height: 1px;
      clip-path: inset(50%);
    }
    .selected-file {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto auto;
      align-items: center;
      gap: 0.5rem;
      margin-bottom: 0.5rem;
      padding: 0.42rem 0.52rem;
      border: 1px solid var(--border);
      border-radius: 0.65rem;
      background: var(--surface-subtle);
    }
    .selected-file > span {
      display: grid;
      width: 1.9rem;
      height: 1.9rem;
      place-items: center;
      border-radius: 0.45rem;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .selected-file strong {
      overflow: hidden;
      font-size: 0.7rem;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .selected-file small {
      color: var(--text-faint);
      font-size: 0.63rem;
    }
    .selected-file button {
      display: grid;
      width: 1.9rem;
      height: 1.9rem;
      place-items: center;
      padding: 0;
      border: 0;
      border-radius: 50%;
      color: var(--text-muted);
      background: transparent;
      cursor: pointer;
    }
    @media (hover: none) {
      .message-actions {
        opacity: 1;
        transform: none;
        pointer-events: auto;
        box-shadow: none;
      }
    }
    @media (max-width: 680px) {
      .entity-header {
        align-items: flex-start;
        flex-direction: column;
      }
      .message-row {
        width: 92%;
      }
      .messages {
        max-height: 70dvh;
        padding-inline: 0.65rem;
      }
      .composer-box {
        grid-template-columns: auto auto auto minmax(4rem, 1fr) auto;
      }
    }
    @media (prefers-reduced-motion: reduce) {
      .messages {
        scroll-behavior: auto;
      }
      .message-actions {
        transform: none;
        transition: opacity 120ms ease;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EntityChatComponent implements OnDestroy {
  readonly entityType = input.required<'lead' | 'deal'>();
  readonly entityId = input.required<string>();
  readonly store = inject(ChatDockStore);
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly draft = signal('');
  readonly selectedFile = signal<File | null>(null);
  readonly recording = signal<'voice' | 'video' | null>(null);
  readonly mediaUrls = signal<ReadonlyMap<string, string>>(new Map());
  readonly mediaLoading = signal<ReadonlySet<string>>(new Set());
  readonly pinnedMessages = computed(() =>
    this.store.messages().filter((message) => message.pinned),
  );
  readonly likeEmoji = '\u{1F44D}';
  readonly heartEmoji = '\u2764\uFE0F';
  private recorder: MediaRecorder | null = null;
  private recordingStream: MediaStream | null = null;
  private recordingTimer: number | null = null;
  private recordingBytes = 0;
  private destroyed = false;
  private entityKey = '';
  private entityGeneration = 0;
  private readonly entityReload = effect(() => {
    const entityType = this.entityType();
    const entityId = this.entityId();
    const key = `${entityType}:${entityId}`;
    if (key === this.entityKey) return;
    this.abortRecording();
    this.entityKey = key;
    this.entityGeneration += 1;
    this.draft.set('');
    this.selectedFile.set(null);
    this.clearMediaUrls();
    void this.store.openEntity(entityType, entityId);
  });

  ngOnDestroy(): void {
    this.destroyed = true;
    if (this.recorder && this.recorder.state !== 'inactive') {
      this.recorder.onstop = null;
      this.recorder.stop();
    }
    this.stopTracks();
    this.clearMediaUrls();
  }
  async initialize(): Promise<void> {
    await this.store.openEntity(this.entityType(), this.entityId());
  }
  async loadOlder(): Promise<void> {
    const id = this.store.activeConversationId();
    if (id) await this.store.loadMessages(id, true);
  }
  async send(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    const generation = this.entityGeneration;
    if (!(await this.store.send(this.draft(), this.selectedFile()))) return;
    if (generation !== this.entityGeneration) return;
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
    const fileInput = event.target as HTMLInputElement;
    this.selectedFile.set(fileInput.files?.item(0) ?? null);
    fileInput.value = '';
  }
  textValue(event: Event): string {
    return (event.target as HTMLTextAreaElement).value;
  }
  attachmentsFor(message: ChatMessage): readonly ChatAttachment[] {
    return this.store.attachmentsByMessage().get(message.id) ?? [];
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
  initials(name: string): string {
    return name
      .split(/\s+/u)
      .slice(0, 2)
      .map((part) => part.slice(0, 1).toUpperCase())
      .join('');
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

  async startRecording(kind: 'voice' | 'video'): Promise<void> {
    if (this.recording() || !navigator.mediaDevices || typeof MediaRecorder === 'undefined') return;
    const generation = this.entityGeneration;
    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: kind === 'video' });
    } catch (error) {
      this.store.error.set(error);
      return;
    }
    if (this.destroyed || generation !== this.entityGeneration) {
      stream.getTracks().forEach((track) => track.stop());
      return;
    }
    const chunks: Blob[] = [];
    const recorder = new MediaRecorder(stream);
    this.recordingBytes = 0;
    recorder.ondataavailable = (event) => {
      if (event.data.size <= 0) return;
      if (this.recordingBytes + event.data.size > maxRecordingBytes) {
        if (recorder.state !== 'inactive') recorder.stop();
        return;
      }
      this.recordingBytes += event.data.size;
      chunks.push(event.data);
    };
    recorder.onstop = () => {
      this.clearRecordingTimer();
      if (!this.destroyed && generation === this.entityGeneration)
        void this.finishRecording(kind, chunks, recorder.mimeType, generation);
    };
    this.recordingStream = stream;
    this.recorder = recorder;
    this.recording.set(kind);
    recorder.start(1000);
    this.recordingTimer = window.setTimeout(() => {
      if (recorder.state !== 'inactive') recorder.stop();
    }, maxRecordingDurationMs);
  }
  stopRecording(): void {
    this.recorder?.stop();
  }
  async loadMedia(attachment: ChatAttachment): Promise<void> {
    const workspaceId = this.workspace.id();
    const generation = this.entityGeneration;
    if (
      !workspaceId ||
      this.mediaUrls().has(attachment.id) ||
      this.mediaLoading().has(attachment.id)
    )
      return;
    this.mediaLoading.update((items) => new Set(items).add(attachment.id));
    try {
      const blob = await this.api.downloadAttachment(workspaceId, attachment.id);
      if (this.workspace.id() !== workspaceId || generation !== this.entityGeneration) return;
      this.mediaUrls.update((items) =>
        new Map(items).set(attachment.id, URL.createObjectURL(blob)),
      );
    } catch (error) {
      if (this.workspace.id() === workspaceId && generation === this.entityGeneration)
        this.store.error.set(error);
    } finally {
      this.mediaLoading.update((items) => {
        const next = new Set(items);
        next.delete(attachment.id);
        return next;
      });
    }
  }
  scrollToMessage(messageId: string): void {
    document
      .getElementById(`entity-chat-message-${messageId}`)
      ?.scrollIntoView({ block: 'center' });
  }

  private async finishRecording(
    kind: 'voice' | 'video',
    chunks: Blob[],
    mimeType: string,
    generation: number,
  ): Promise<void> {
    if (generation !== this.entityGeneration) return;
    this.recording.set(null);
    this.stopTracks();
    const extension = kind === 'video' ? 'webm' : mimeType.includes('ogg') ? 'ogg' : 'webm';
    const timestamp = new Date().toISOString().replaceAll(':', '-');
    const file = new File(chunks, `${kind}-${timestamp}.${extension}`, {
      type: mimeType || `${kind === 'voice' ? 'audio' : 'video'}/webm`,
    });
    const body =
      kind === 'voice' ? this.i18n.t('chat.voiceMessage') : this.i18n.t('chat.videoMessage');
    if (!(await this.store.send(body, file, kind))) {
      if (generation !== this.entityGeneration) return;
      this.draft.set(body);
      this.selectedFile.set(file);
    }
  }
  private stopTracks(): void {
    this.clearRecordingTimer();
    this.recordingStream?.getTracks().forEach((track) => track.stop());
    this.recordingStream = null;
    this.recorder = null;
  }
  private abortRecording(): void {
    if (this.recorder && this.recorder.state !== 'inactive') {
      this.recorder.onstop = null;
      this.recorder.stop();
    }
    this.recording.set(null);
    this.stopTracks();
  }
  private clearRecordingTimer(): void {
    if (this.recordingTimer !== null) window.clearTimeout(this.recordingTimer);
    this.recordingTimer = null;
  }
  private clearMediaUrls(): void {
    for (const url of this.mediaUrls().values()) URL.revokeObjectURL(url);
    this.mediaUrls.set(new Map());
    this.mediaLoading.set(new Set());
  }
}

const maxRecordingBytes = 10 * 1024 * 1024;
const maxRecordingDurationMs = 5 * 60 * 1000;
