import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import type { ElementRef } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  AssignmentSubjectOption,
  RecordAssignment,
  RecordAssignmentInput,
  RecordAssignmentSet,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../feedback/toast.service';
import { IconComponent } from '../icon/icon.component';
import { ErrorPanelComponent } from '../state/error-panel.component';

export type AssignmentResourceType = 'lead' | 'deal' | 'task';

@Component({
  selector: 'app-record-assignments',
  imports: [ErrorPanelComponent, IconComponent, MatButtonModule],
  template: `
    <section class="assignments" [attr.aria-busy]="loading()">
      <header class="assignment-heading">
        <div>
          <h3>{{ i18n.t('assignments.title') }}</h3>
          <p>{{ i18n.t('assignments.subtitle') }}</p>
        </div>
        @if (canEdit()) {
          <button
            #addTrigger
            mat-stroked-button
            type="button"
            class="add-participant"
            aria-controls="assignment-editor"
            [attr.aria-expanded]="editorOpen()"
            (click)="toggleEditor()"
          >
            <app-icon name="add" />{{ i18n.t(addParticipantKey) }}
          </button>
        }
      </header>

      @if (error()) {
        <app-error-panel [error]="error()" (retry)="load()" />
      }

      <div class="assignment-list" [attr.role]="assignmentSet().items.length > 0 ? 'list' : null">
        @for (item of assignmentSet().items; track item.id) {
          <div class="assignment-row" role="listitem">
            <span class="subject-icon" aria-hidden="true">{{
              item.subjectType === 'user' ? 'U' : 'D'
            }}</span>
            <span class="assignment-identity">
              <strong>{{ item.displayName }}</strong>
              <small>
                {{
                  i18n.t(
                    item.subjectType === 'user' ? 'assignments.user' : 'assignments.department'
                  )
                }}
                · {{ i18n.t(kindKey(item.kind)) }}
              </small>
            </span>
            @if (item.isPrimary) {
              <span class="status-pill">{{ i18n.t('assignments.primary') }}</span>
            }
            @if (canEdit()) {
              <button
                mat-button
                type="button"
                [disabled]="saving()"
                [attr.aria-label]="i18n.t('assignments.removeLabel', { name: item.displayName })"
                (click)="remove(item)"
              >
                {{ i18n.t('common.action.delete') }}
              </button>
            }
          </div>
        } @empty {
          <p class="empty">{{ i18n.t('assignments.empty') }}</p>
        }
      </div>

      @if (canEdit() && editorOpen()) {
        <section
          id="assignment-editor"
          class="assignment-editor"
          role="dialog"
          aria-modal="false"
          aria-labelledby="assignment-editor-title"
          (keydown.escape)="closeEditor()"
        >
          <header>
            <h4 id="assignment-editor-title">{{ i18n.t(addParticipantKey) }}</h4>
          </header>
          <div class="editor-fields">
            <label class="native-field subject-select">
              <span>{{ i18n.t('assignments.subject') }}</span>
              <select #subjectSelect [value]="subjectValue()" (change)="setSubject($event)">
                <option value="">{{ i18n.t('assignments.chooseSubject') }}</option>
                @for (option of options(); track option.type + ':' + option.id) {
                  <option [value]="option.type + ':' + option.id">
                    {{
                      option.type === 'user'
                        ? i18n.t('assignments.user')
                        : i18n.t('assignments.department')
                    }}
                    · {{ option.name }}
                  </option>
                }
              </select>
            </label>
            <label class="native-field">
              <span>{{ i18n.t('assignments.kind') }}</span>
              <select [value]="kind()" (change)="setKind($event)">
                <option value="responsible">{{ i18n.t('assignments.kind.responsible') }}</option>
                <option value="watcher">{{ i18n.t('assignments.kind.watcher') }}</option>
              </select>
            </label>
          </div>
          @if (kind() === 'responsible') {
            <label class="primary-toggle">
              <input type="checkbox" [checked]="primary()" (change)="setPrimary($event)" />
              <span>{{ i18n.t('assignments.makePrimary') }}</span>
            </label>
          }
          <footer>
            <button mat-button type="button" [disabled]="saving()" (click)="closeEditor()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button
              mat-flat-button
              type="button"
              [disabled]="saving() || !subjectValue()"
              (click)="add()"
            >
              {{ i18n.t(addParticipantKey) }}
            </button>
          </footer>
        </section>
      }
    </section>
  `,
  styles: `
    .assignments {
      display: grid;
      gap: 0.8rem;
    }
    .assignment-heading {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 1rem;
    }
    header h3,
    .assignment-editor h4 {
      margin: 0;
      font-size: 0.9rem;
    }
    header p,
    .empty {
      margin: 0.2rem 0 0;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .assignment-list {
      display: grid;
      gap: 0.4rem;
    }
    .assignment-row {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto auto;
      align-items: center;
      gap: 0.65rem;
      min-height: 3.25rem;
      padding: 0.4rem 0.45rem 0.4rem 0.55rem;
      border-radius: var(--control-radius);
      background: var(--surface-subtle);
    }
    .assignment-identity {
      display: grid;
      min-width: 0;
    }
    .assignment-row strong {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.78rem;
    }
    .assignment-row small {
      color: var(--text-muted);
      font-size: 0.68rem;
    }
    .subject-icon {
      display: grid;
      width: 1.75rem;
      height: 1.75rem;
      place-items: center;
      border-radius: 50%;
      color: var(--brand);
      background: var(--brand-soft);
      font-size: 0.65rem;
      font-weight: 700;
    }
    .assignment-editor {
      display: grid;
      gap: 0.85rem;
      padding: 1rem;
      border: 1px solid var(--border);
      border-radius: var(--panel-radius);
      background: var(--surface-raised);
      transform-origin: top right;
    }
    .editor-fields {
      display: grid;
      grid-template-columns: minmax(0, 1.4fr) minmax(9rem, 0.6fr);
      gap: 0.75rem;
    }
    .assignment-editor footer {
      display: flex;
      justify-content: flex-end;
      gap: 0.5rem;
    }
    .add-participant {
      flex: 0 0 auto;
    }
    .add-participant app-icon {
      margin-inline-end: 0.45rem;
    }
    .primary-toggle {
      display: flex;
      align-items: center;
      gap: 0.45rem;
      min-height: var(--control-height);
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .primary-toggle input {
      width: 1.5rem;
      height: 1.5rem;
      flex: 0 0 1.5rem;
      margin: 0;
      accent-color: var(--brand);
    }
    button {
      transition: transform 140ms cubic-bezier(0.23, 1, 0.32, 1);
    }
    button:active {
      transform: scale(0.98);
    }
    @media (max-width: 700px) {
      .assignment-heading {
        align-items: stretch;
        flex-direction: column;
      }
      .add-participant {
        width: 100%;
      }
      .editor-fields {
        grid-template-columns: 1fr;
      }
      .assignment-row {
        grid-template-columns: auto minmax(0, 1fr) auto;
      }
      .assignment-row > button {
        grid-column: 2 / -1;
        justify-self: start;
      }
    }
    @media (prefers-reduced-motion: reduce) {
      button {
        transition: none;
      }
      button:active {
        transform: none;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RecordAssignmentsComponent {
  readonly resourceType = input.required<AssignmentResourceType>();
  readonly resourceId = input.required<string>();
  readonly version = input.required<number>();
  readonly versionChange = output<number>();

  readonly i18n = inject(I18nService);
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly permissions = inject(Permissions);
  private readonly toasts = inject(ToastService);
  private requestSequence = 0;

  readonly assignmentSet = signal<RecordAssignmentSet>({ items: [], version: 1 });
  readonly options = signal<readonly AssignmentSubjectOption[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  readonly kind = signal<'responsible' | 'watcher'>('responsible');
  readonly subjectValue = signal('');
  readonly primary = signal(false);
  readonly editorOpen = signal(false);
  readonly addParticipantKey = 'assignments.addParticipant' as AppMessageKey;
  private readonly addTrigger = viewChild<ElementRef<HTMLButtonElement>>('addTrigger');
  private readonly subjectSelect = viewChild<ElementRef<HTMLSelectElement>>('subjectSelect');

  constructor() {
    effect(() => {
      this.resourceType();
      this.resourceId();
      this.version();
      void this.load();
    });
    effect(() => {
      if (this.editorOpen()) this.subjectSelect()?.nativeElement.focus();
    });
  }

  toggleEditor(): void {
    if (this.editorOpen()) this.closeEditor();
    else this.editorOpen.set(true);
  }

  closeEditor(): void {
    this.editorOpen.set(false);
    this.subjectValue.set('');
    this.primary.set(false);
    queueMicrotask(() => this.addTrigger()?.nativeElement.focus());
  }

  canEdit(): boolean {
    switch (this.resourceType()) {
      case 'lead':
        return this.permissions.allows('leads.update');
      case 'deal':
        return this.permissions.allows('deals.update');
      case 'task':
        return this.permissions.allows('records.update');
    }
  }

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    const resourceId = this.resourceId();
    if (!workspaceId || !resourceId) return;
    const request = ++this.requestSequence;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [assignments, options] = await Promise.all([
        this.loadAssignments(workspaceId, resourceId),
        this.canEdit() ? this.api.listAssignmentSubjects(workspaceId) : Promise.resolve([]),
      ]);
      if (request !== this.requestSequence) return;
      this.assignmentSet.set(assignments);
      this.options.set(options);
    } catch (error) {
      if (request === this.requestSequence) this.error.set(error);
    } finally {
      if (request === this.requestSequence) this.loading.set(false);
    }
  }

  setKind(event: Event): void {
    const kind = (event.target as HTMLSelectElement).value as 'responsible' | 'watcher';
    this.kind.set(kind);
    if (kind === 'watcher') this.primary.set(false);
  }

  setSubject(event: Event): void {
    this.subjectValue.set((event.target as HTMLSelectElement).value);
  }

  setPrimary(event: Event): void {
    this.primary.set((event.target as HTMLInputElement).checked);
  }

  async add(): Promise<void> {
    const [subjectType, subjectId] = this.subjectValue().split(':', 2);
    if ((subjectType !== 'user' && subjectType !== 'department') || !subjectId) return;
    const current = this.assignmentSet().items;
    if (
      current.some(
        (item) =>
          item.kind === this.kind() &&
          item.subjectType === subjectType &&
          item.subjectId === subjectId,
      )
    )
      return;
    const next: RecordAssignmentInput[] = current.map((item) => ({
      kind: item.kind,
      subjectType: item.subjectType,
      subjectId: item.subjectId,
      isPrimary: this.primary() ? false : item.isPrimary,
    }));
    next.push({
      kind: this.kind(),
      subjectType,
      subjectId,
      isPrimary: this.kind() === 'responsible' && this.primary(),
    });
    await this.save(next);
    if (!this.error()) this.closeEditor();
  }

  async remove(item: RecordAssignment): Promise<void> {
    await this.save(
      this.assignmentSet()
        .items.filter((candidate) => candidate.id !== item.id)
        .map((candidate) => ({
          kind: candidate.kind,
          subjectType: candidate.subjectType,
          subjectId: candidate.subjectId,
          isPrimary: candidate.isPrimary,
        })),
    );
  }

  kindKey(
    kind: RecordAssignment['kind'],
  ): 'assignments.kind.responsible' | 'assignments.kind.watcher' {
    return `assignments.kind.${kind}`;
  }

  private async save(items: readonly RecordAssignmentInput[]): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.saving()) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const current = this.assignmentSet();
      const updated = await this.replaceAssignments(workspaceId, current.version, items);
      this.assignmentSet.set(updated);
      this.versionChange.emit(updated.version);
      this.toasts.show({ messageKey: 'assignments.saved', messageParams: {} });
    } catch (error) {
      this.error.set(error);
    } finally {
      this.saving.set(false);
    }
  }

  private loadAssignments(workspaceId: string, resourceId: string): Promise<RecordAssignmentSet> {
    switch (this.resourceType()) {
      case 'lead':
        return this.api.listLeadAssignments(workspaceId, resourceId);
      case 'deal':
        return this.api.listDealAssignments(workspaceId, resourceId);
      case 'task':
        return this.api.listTaskAssignments(workspaceId, resourceId);
    }
  }

  private replaceAssignments(
    workspaceId: string,
    version: number,
    items: readonly RecordAssignmentInput[],
  ): Promise<RecordAssignmentSet> {
    const resourceId = this.resourceId();
    switch (this.resourceType()) {
      case 'lead':
        return this.api.replaceLeadAssignments(workspaceId, resourceId, version, items);
      case 'deal':
        return this.api.replaceDealAssignments(workspaceId, resourceId, version, items);
      case 'task':
        return this.api.replaceTaskAssignments(workspaceId, resourceId, version, items);
    }
  }
}
