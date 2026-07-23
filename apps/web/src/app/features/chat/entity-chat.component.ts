import type { OnDestroy } from '@angular/core';
import { ChangeDetectionStrategy, Component, effect, inject, input, signal } from '@angular/core';
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
      <header>
        <div>
          <h2 id="entity-chat-title">{{ i18n.t('chat.entityTitle') }}</h2>
          <p>{{ i18n.t('chat.entitySubtitle') }}</p>
        </div>
        @if (recording()) {
          <button mat-flat-button type="button" class="stop" (click)="stopRecording()">
            {{ i18n.t('chat.stopRecording') }}
          </button>
        }
      </header>
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="initialize()" />
      }
      <div class="messages" [attr.aria-busy]="store.loading()">
        @if (store.nextCursor()) {
          <button mat-button type="button" (click)="loadOlder()">
            {{ i18n.t('chat.loadOlder') }}
          </button>
        }
        @for (message of store.messages(); track message.id) {
          <article [class.own]="message.senderUserId === auth.user()?.id">
            <header>
              <strong>{{ message.senderDisplayName }}</strong
              ><time [attr.datetime]="message.createdAt">{{
                i18n.date(message.createdAt, { dateStyle: 'short', timeStyle: 'short' })
              }}</time>
            </header>
            @if (message.body) {
              <p>{{ message.body }}</p>
            }
            @for (attachment of attachmentsFor(message); track attachment.id) {
              @if (isAudio(attachment) && mediaUrl(attachment.id)) {
                <audio controls preload="metadata" [src]="mediaUrl(attachment.id)"></audio>
              } @else if (isVideo(attachment) && mediaUrl(attachment.id)) {
                <video controls preload="metadata" [src]="mediaUrl(attachment.id)"></video>
              } @else if (isAudio(attachment) || isVideo(attachment)) {
                <button type="button" class="attachment" (click)="loadMedia(attachment)">
                  <app-icon name="play" />{{ i18n.t('chat.loadMedia') }}
                </button>
              } @else {
                <button type="button" class="attachment" (click)="store.download(attachment)">
                  <app-icon name="attachment" />{{ attachment.displayName }}
                </button>
              }
            }
            <footer>
              @if (message.pinned) {
                <span>{{ i18n.t('chat.pinned') }}</span>
              }
              <button type="button" (click)="store.react(message.id, '👍')">👍</button>
              <button type="button" (click)="store.react(message.id, '❤')">❤</button>
              <button type="button" (click)="store.pin(message)"><app-icon name="pin" /></button>
            </footer>
          </article>
        } @empty {
          @if (!store.loading()) {
            <div class="empty-state">{{ i18n.t('chat.emptyMessages') }}</div>
          }
        }
      </div>
      <form class="composer" (submit)="send($event)">
        <label class="icon-action" [title]="i18n.t('chat.attach')">
          <input
            type="file"
            [disabled]="!store.activeConversationId() || store.loading()"
            (change)="chooseFile($event)"
          />
          <app-icon name="attachment" />
        </label>
        <button
          class="icon-action"
          type="button"
          [disabled]="!store.activeConversationId() || store.loading()"
          [title]="i18n.t('chat.recordVoice')"
          (click)="startRecording('voice')"
        >
          <app-icon name="microphone" />
        </button>
        <button
          class="icon-action"
          type="button"
          [disabled]="!store.activeConversationId() || store.loading()"
          [title]="i18n.t('chat.recordVideo')"
          (click)="startRecording('video')"
        >
          <app-icon name="video" />
        </button>
        <textarea
          rows="2"
          [value]="draft()"
          (input)="draft.set(textValue($event))"
          [placeholder]="i18n.t('chat.message')"
        ></textarea>
        <button
          mat-flat-button
          type="submit"
          [disabled]="
            !store.activeConversationId() ||
            store.loading() ||
            store.sending() ||
            (!draft().trim() && !selectedFile())
          "
        >
          <app-icon name="send" />{{ i18n.t('chat.send') }}
        </button>
        @if (selectedFile(); as file) {
          <span class="selected-file">{{ file.name }}</span>
        }
      </form>
    </section>
  `,
  styles: `
    .entity-chat {
      overflow: hidden;
    }
    .entity-chat > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 1rem;
      border-bottom: 1px solid var(--border);
    }
    h2,
    p {
      margin: 0;
    }
    header p {
      margin-top: 0.25rem;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    .messages {
      display: flex;
      min-height: 18rem;
      max-height: 34rem;
      flex-direction: column;
      gap: 0.7rem;
      overflow: auto;
      padding: 1rem;
      background: var(--surface-subtle);
    }
    article {
      width: min(78%, 42rem);
      padding: 0.75rem 0.85rem;
      border: 1px solid var(--border);
      border-radius: 0.8rem;
      background: var(--surface-raised);
    }
    article.own {
      align-self: flex-end;
      border-color: color-mix(in srgb, var(--brand) 25%, var(--border));
      background: var(--brand-soft);
    }
    article header,
    article footer {
      display: flex;
      align-items: center;
      gap: 0.55rem;
    }
    article header {
      justify-content: space-between;
      font-size: 0.75rem;
    }
    article time {
      color: var(--text-muted);
    }
    article p {
      margin: 0.55rem 0;
      white-space: pre-wrap;
    }
    article footer {
      justify-content: flex-end;
      margin-top: 0.45rem;
    }
    article footer button {
      border: 0;
      background: transparent;
      cursor: pointer;
    }
    audio,
    video {
      display: block;
      width: min(100%, 28rem);
      margin-top: 0.5rem;
    }
    video {
      max-height: 20rem;
      border-radius: 0.55rem;
      background: #000;
    }
    .attachment {
      display: inline-flex;
      align-items: center;
      gap: 0.4rem;
      margin-top: 0.5rem;
      border: 0;
      color: var(--brand);
      background: transparent;
      cursor: pointer;
    }
    .composer {
      display: grid;
      grid-template-columns: auto auto auto minmax(8rem, 1fr) auto;
      align-items: end;
      gap: 0.5rem;
      padding: 0.8rem;
      border-top: 1px solid var(--border);
    }
    .composer textarea {
      min-height: 2.75rem;
      resize: vertical;
      padding: 0.65rem 0.75rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      background: var(--surface-raised);
      color: var(--text);
    }
    .icon-action {
      display: grid;
      width: 2.6rem;
      height: 2.6rem;
      place-items: center;
      border: 1px solid var(--border);
      border-radius: 0.6rem;
      background: var(--surface-raised);
      cursor: pointer;
    }
    .icon-action input {
      position: absolute;
      width: 1px;
      height: 1px;
      opacity: 0;
    }
    .selected-file {
      grid-column: 4 / -1;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .stop {
      background: var(--danger);
    }
    @media (max-width: 680px) {
      article {
        width: 92%;
      }
      .composer {
        grid-template-columns: repeat(3, auto) 1fr;
      }
      .composer textarea {
        grid-column: 1 / -1;
        grid-row: 1;
      }
      .composer button[mat-flat-button] {
        grid-column: 4;
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
    for (const url of this.mediaUrls().values()) URL.revokeObjectURL(url);
    this.mediaUrls.set(new Map());
    void this.store.openEntity(entityType, entityId);
  });

  ngOnDestroy(): void {
    this.destroyed = true;
    if (this.recorder && this.recorder.state !== 'inactive') {
      this.recorder.onstop = null;
      this.recorder.stop();
    }
    this.stopTracks();
    for (const url of this.mediaUrls().values()) URL.revokeObjectURL(url);
  }
  async initialize(): Promise<void> {
    await this.store.openEntity(this.entityType(), this.entityId());
  }
  async loadOlder(): Promise<void> {
    const id = this.store.activeConversationId();
    if (id) {
      await this.store.loadMessages(id, true);
    }
  }
  async send(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    const generation = this.entityGeneration;
    if (!(await this.store.send(this.draft(), this.selectedFile()))) return;
    if (generation !== this.entityGeneration) return;
    this.draft.set('');
    this.selectedFile.set(null);
  }
  chooseFile(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.selectedFile.set(input.files?.item(0) ?? null);
    input.value = '';
  }
  textValue(event: Event): string {
    return (event.target as HTMLTextAreaElement).value;
  }
  attachmentsFor(message: ChatMessage): readonly ChatAttachment[] {
    return this.store.attachmentsByMessage().get(message.id) ?? [];
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
      if (!this.destroyed && generation === this.entityGeneration) {
        void this.finishRecording(kind, chunks, recorder.mimeType, generation);
      }
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
  async loadMedia(attachment: ChatAttachment): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.mediaUrls().has(attachment.id)) return;
    try {
      const blob = await this.api.downloadAttachment(workspaceId, attachment.id);
      for (const url of this.mediaUrls().values()) URL.revokeObjectURL(url);
      this.mediaUrls.set(new Map([[attachment.id, URL.createObjectURL(blob)]]));
    } catch (error) {
      this.store.error.set(error);
    }
  }
  private clearRecordingTimer(): void {
    if (this.recordingTimer !== null) window.clearTimeout(this.recordingTimer);
    this.recordingTimer = null;
  }
}

const maxRecordingBytes = 10 * 1024 * 1024;
const maxRecordingDurationMs = 5 * 60 * 1000;
