import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import type { Attachment, AttachmentEntityType } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../state/error-panel.component';
import { AttachmentStore } from './attachment.store';

@Component({
  selector: 'app-attachment-panel',
  imports: [ErrorPanelComponent, MatButtonModule],
  providers: [AttachmentStore],
  template: `
    <section class="panel attachment-panel" aria-labelledby="attachments-title">
      <header>
        <div>
          <h2 id="attachments-title">{{ i18n.t('files.title') }}</h2>
          <p>{{ i18n.t('files.subtitle') }}</p>
        </div>
        @if (permissions.allows('records.create')) {
          <label class="upload-action" [class.disabled]="store.uploading()">
            <input type="file" [disabled]="store.uploading()" (change)="upload($event)" />
            <span>{{ i18n.t(store.uploading() ? 'files.uploading' : 'files.upload') }}</span>
          </label>
        }
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="load()" />
      }

      <div class="attachment-body" [attr.aria-busy]="store.loading() || store.uploading()">
        @if (store.loading() && store.items().length === 0) {
          <div class="skeleton attachment-skeleton"></div>
        } @else if (store.items().length === 0) {
          <div class="empty-state">{{ i18n.t('files.empty') }}</div>
        } @else {
          <ul>
            @for (attachment of store.items(); track attachment.id) {
              <li>
                <div class="file-mark" aria-hidden="true">{{ extension(attachment) }}</div>
                <div class="file-info">
                  <strong>{{ attachment.displayName }}</strong>
                  <span>
                    {{ formatBytes(attachment.sizeBytes) }} ·
                    {{ i18n.date(attachment.createdAt) }} ·
                    {{ i18n.t(scanStateKey(attachment.scanState)) }}
                  </span>
                </div>
                <div class="file-actions">
                  <button
                    mat-button
                    type="button"
                    [disabled]="attachment.scanState === 'rejected'"
                    (click)="download(attachment)"
                  >
                    {{ i18n.t('files.download') }}
                  </button>
                  @if (permissions.allows('records.delete')) {
                    @if (deleteConfirmId() === attachment.id) {
                      <button mat-button type="button" (click)="deleteConfirmId.set(null)">
                        {{ i18n.t('common.action.cancel') }}
                      </button>
                      <button
                        mat-flat-button
                        type="button"
                        class="danger-button"
                        [disabled]="store.deleting() === attachment.id"
                        (click)="remove(attachment.id)"
                      >
                        {{ i18n.t('files.confirmDelete') }}
                      </button>
                    } @else {
                      <button
                        mat-button
                        type="button"
                        class="danger-action"
                        (click)="deleteConfirmId.set(attachment.id)"
                      >
                        {{ i18n.t('common.action.delete') }}
                      </button>
                    }
                  }
                </div>
              </li>
            }
          </ul>
        }
      </div>
    </section>
  `,
  styles: `
    :host {
      display: block;
      min-width: 0;
    }
    .attachment-panel {
      overflow: hidden;
    }
    .attachment-panel > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 0.85rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    h2,
    header p {
      margin: 0;
    }
    h2 {
      font-size: 1rem;
    }
    header p {
      margin-top: 0.25rem;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    .upload-action {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-height: 2.5rem;
      padding-inline: 1rem;
      border-radius: 999px;
      color: var(--mat-sys-on-primary, white);
      background: var(--brand);
      font-size: 0.82rem;
      font-weight: 650;
      cursor: pointer;
    }
    .upload-action.disabled {
      opacity: 0.6;
      cursor: wait;
    }
    .upload-action input {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
    ul {
      margin: 0;
      padding: 0;
      list-style: none;
    }
    li {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.75rem;
      min-height: 4.25rem;
      padding: 0.65rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    li:last-child {
      border: 0;
    }
    .file-mark {
      display: grid;
      width: 2.4rem;
      height: 2.4rem;
      place-items: center;
      border-radius: 0.55rem;
      color: var(--brand);
      background: var(--brand-soft);
      font-size: 0.62rem;
      font-weight: 750;
      text-transform: uppercase;
    }
    .file-info {
      min-width: 0;
    }
    .file-info strong,
    .file-info span {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .file-info strong {
      font-size: 0.86rem;
    }
    .file-info span {
      margin-top: 0.2rem;
      color: var(--text-muted);
      font-size: 0.72rem;
    }
    .file-actions {
      display: flex;
      align-items: center;
      gap: 0.2rem;
    }
    .danger-action,
    .danger-button {
      color: var(--danger);
    }
    .attachment-skeleton {
      min-height: 5rem;
      margin: 0.75rem;
    }
    @media (max-width: 680px) {
      .attachment-panel > header {
        align-items: stretch;
        flex-direction: column;
      }
      li {
        grid-template-columns: auto minmax(0, 1fr);
      }
      .file-actions {
        grid-column: 1 / -1;
        justify-content: flex-end;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AttachmentPanelComponent implements OnInit {
  readonly entityType = input.required<AttachmentEntityType>();
  readonly entityId = input.required<string>();
  readonly store = inject(AttachmentStore);
  readonly permissions = inject(Permissions);
  readonly i18n = inject(I18nService);
  readonly deleteConfirmId = signal<string | null>(null);

  ngOnInit(): void {
    void this.load();
  }

  load(): Promise<void> {
    return this.store.load(this.entityType(), this.entityId());
  }

  async upload(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.item(0);
    if (!file) return;
    await this.store.upload(this.entityType(), this.entityId(), file);
    input.value = '';
  }

  async download(attachment: Attachment): Promise<void> {
    const blob = await this.store.download(attachment.id);
    if (!blob) return;
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = attachment.displayName;
    link.rel = 'noopener';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }

  async remove(attachmentId: string): Promise<void> {
    if (await this.store.remove(attachmentId)) this.deleteConfirmId.set(null);
  }

  extension(attachment: Attachment): string {
    const extension = attachment.displayName.split('.').pop();
    return extension && extension !== attachment.displayName ? extension.slice(0, 4) : 'file';
  }

  formatBytes(bytes: number): string {
    const units = ['B', 'KB', 'MB', 'GB'] as const;
    let value = Math.max(0, bytes);
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit++;
    }
    return `${new Intl.NumberFormat(this.i18n.locale(), { maximumFractionDigits: 1 }).format(value)} ${units[unit]}`;
  }

  scanStateKey(
    state: Attachment['scanState'],
  ): 'files.scan.clean' | 'files.scan.rejected' | 'files.scan.unavailable' {
    return `files.scan.${state}`;
  }
}
