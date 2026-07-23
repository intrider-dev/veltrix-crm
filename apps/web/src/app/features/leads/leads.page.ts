import {
  CdkDrag,
  CdkDragHandle,
  CdkDragPlaceholder,
  CdkDropList,
  CdkDropListGroup,
  type CdkDragDrop,
} from '@angular/cdk/drag-drop';
import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { FormField, email, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';

import type { Lead, LeadStage, LeadStatus } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { trimmedOrNull } from '../../shared/forms/feature-validation';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { LeadsStore } from './leads.store';

@Component({
  selector: 'app-leads-page',
  imports: [
    CdkDrag,
    CdkDragHandle,
    CdkDragPlaceholder,
    CdkDropList,
    CdkDropListGroup,
    ErrorPanelComponent,
    FormField,
    FormsModule,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RecordAssignmentsComponent,
    RouterLink,
  ],
  providers: [LeadsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.leads') }}</h1>
          <p>{{ i18n.t('leads.subtitle') }}</p>
        </div>
        @if (permissions.allows('leads.create')) {
          <button mat-flat-button type="button" (click)="openCreate()">
            <app-icon name="add" />{{ i18n.t('leads.add') }}
          </button>
        }
      </header>

      <nav
        class="view-switcher segmented-control"
        [attr.aria-label]="i18n.t('leads.view.switcher')"
      >
        @for (mode of viewModes; track mode) {
          <button
            mat-button
            type="button"
            [class.active]="store.viewMode() === mode"
            [attr.aria-pressed]="store.viewMode() === mode"
            (click)="store.setViewMode(mode)"
          >
            {{
              i18n.t(
                mode === 'list'
                  ? 'leads.view.list'
                  : mode === 'gantt'
                    ? 'leads.view.gantt'
                    : 'leads.view.kanban'
              )
            }}
          </button>
        }
      </nav>

      <form class="panel feature-toolbar" (submit)="filter($event)" role="search">
        <label class="native-field grow">
          <span>{{ i18n.t('leads.search') }}</span>
          <input
            type="search"
            [value]="store.query()"
            (input)="store.query.set(inputValue($event))"
          />
        </label>
        <label class="native-field">
          <span>{{ i18n.t('common.field.status') }}</span>
          <select [value]="store.status()" (change)="setFilterStatus($event)">
            <option value="">{{ i18n.t('leads.status.all') }}</option>
            @for (status of statuses; track status) {
              <option [value]="status">{{ i18n.t(statusKey(status)) }}</option>
            }
            <option value="converted">{{ i18n.t('leads.status.converted') }}</option>
          </select>
        </label>
        <button mat-stroked-button type="submit">{{ i18n.t('common.action.search') }}</button>
      </form>

      @if (store.loadError()) {
        <app-error-panel [error]="store.loadError()" (retry)="store.load()" />
      }
      @if (store.stageError()) {
        <app-error-panel [error]="store.stageError()" (retry)="retryStages()" />
      }

      @if (createOpen()) {
        <section class="panel editor" aria-labelledby="new-lead-title">
          <header>
            <h2 id="new-lead-title">{{ i18n.t('leads.createTitle') }}</h2>
            <button mat-button type="button" (click)="createOpen.set(false)">
              {{ i18n.t('common.action.close') }}
            </button>
          </header>
          <form class="feature-form" (submit)="create($event)" novalidate>
            @if (store.formError()) {
              <app-error-panel class="form-error" [error]="store.formError()" [retryable]="false" />
            }
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
              <input #nameInput matInput [formField]="leadForm.name" autocomplete="off" />
              @if (leadForm.name().touched() && leadForm.name().invalid()) {
                <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
              }
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.email') }}</mat-label>
              <input matInput type="email" [formField]="leadForm.email" inputmode="email" />
              @if (leadForm.email().touched() && leadForm.email().invalid()) {
                <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
              }
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.phone') }}</mat-label>
              <input matInput [formField]="leadForm.phone" inputmode="tel" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('leads.company') }}</mat-label>
              <input matInput [formField]="leadForm.companyName" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('leads.jobTitle') }}</mat-label>
              <input matInput [formField]="leadForm.jobTitle" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('leads.source') }}</mat-label>
              <input matInput [formField]="leadForm.source" />
            </mat-form-field>
            <label class="native-field">
              <span>{{ i18n.t('leads.stage') }}</span>
              <select [formField]="leadForm.stageId">
                @for (stage of editableStages(); track stage.id) {
                  <option [value]="stage.id">{{ stageLabel(stage) }}</option>
                }
              </select>
            </label>
            <div class="form-actions">
              <button mat-button type="button" (click)="createOpen.set(false)">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button
                mat-flat-button
                type="submit"
                [disabled]="store.saving() || leadForm().invalid()"
              >
                {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
              </button>
            </div>
          </form>
        </section>
      }

      @if (store.viewMode() === 'list') {
        <section class="panel record-list" [attr.aria-busy]="store.loading()">
          @if (store.loading() && store.leads().length === 0) {
            <div class="list-skeleton" [attr.aria-label]="i18n.t('common.result.loading')">
              <div class="skeleton"></div>
              <div class="skeleton"></div>
              <div class="skeleton"></div>
            </div>
          } @else if (store.leads().length === 0) {
            <div class="empty-state">{{ i18n.t('leads.empty') }}</div>
          } @else {
            @for (lead of store.leads(); track lead.id) {
              <article>
                <div class="record-main">
                  <div class="record-avatar" aria-hidden="true">{{ lead.name.charAt(0) }}</div>
                  <div>
                    <h2>
                      <a [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
                    </h2>
                    <p>{{ lead.companyName || lead.email || i18n.t('leads.noDetails') }}</p>
                  </div>
                </div>
                <div class="record-meta">
                  @if (permissions.allows('leads.update') && lead.status !== 'converted') {
                    <label class="visually-hidden" [for]="'lead-status-' + lead.id">{{
                      i18n.t('leads.stage')
                    }}</label>
                    <select
                      [id]="'lead-status-' + lead.id"
                      [value]="lead.stageId"
                      [disabled]="store.isMoving(lead.id)"
                      (change)="changeStage(lead, $event)"
                    >
                      @for (stage of editableStages(); track stage.id) {
                        <option [value]="stage.id">{{ stageLabel(stage) }}</option>
                      }
                    </select>
                    <button
                      mat-stroked-button
                      type="button"
                      [disabled]="store.saving()"
                      (click)="store.convert(lead)"
                    >
                      {{ i18n.t('leads.markWon') }}
                    </button>
                  } @else {
                    <span class="status-pill">{{ leadStageLabel(lead) }}</span>
                  }
                  <button mat-button type="button" (click)="selectedLead.set(lead)">
                    {{ i18n.t('assignments.manage') }}
                  </button>
                </div>
              </article>
            }
          }
        </section>
      } @else if (store.viewMode() === 'kanban') {
        <section class="lead-kanban" cdkDropListGroup [attr.aria-busy]="store.loading()">
          @for (stage of store.boardStages(); track stage.id) {
            <article class="lead-stage">
              <header>
                <h2>{{ stageLabel(stage) }}</h2>
                <span>{{ store.leadsFor(stage.id).length }}</span>
              </header>
              <div
                class="lead-drop-zone"
                cdkDropList
                [id]="stage.id"
                [cdkDropListData]="store.leadsFor(stage.id)"
                [cdkDropListDisabled]="stage.category === 'converted'"
                (cdkDropListDropped)="drop($event, stage)"
              >
                @for (lead of store.leadsFor(stage.id); track lead.id) {
                  <article
                    class="lead-card"
                    cdkDrag
                    [cdkDragData]="lead"
                    [cdkDragDisabled]="
                      !permissions.allows('leads.update') || lead.status === 'converted'
                    "
                  >
                    <button
                      class="drag-handle"
                      type="button"
                      cdkDragHandle
                      tabindex="-1"
                      aria-hidden="true"
                    >
                      &#8942;&#8942;
                    </button>
                    <h3>
                      <a [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
                    </h3>
                    <p>{{ lead.companyName || lead.email || i18n.t('leads.noDetails') }}</p>
                    @if (lead.expectedCloseDate) {
                      <time [attr.datetime]="lead.expectedCloseDate">{{
                        i18n.date(lead.expectedCloseDate)
                      }}</time>
                    }
                    @if (lead.status !== 'converted') {
                      <label class="visually-hidden" [for]="'kanban-stage-' + lead.id">{{
                        i18n.t('leads.stage')
                      }}</label>
                      <select
                        [id]="'kanban-stage-' + lead.id"
                        [ngModel]="lead.stageId"
                        [disabled]="!permissions.allows('leads.update')"
                        (ngModelChange)="moveFromSelect(lead, $event)"
                      >
                        @for (target of editableStages(); track target.id) {
                          <option [value]="target.id">{{ stageLabel(target) }}</option>
                        }
                      </select>
                    } @else {
                      <span class="status-pill">{{ leadStageLabel(lead) }}</span>
                    }
                    <div class="lead-placeholder" *cdkDragPlaceholder></div>
                  </article>
                } @empty {
                  <p class="stage-empty">{{ i18n.t('leads.emptyStage') }}</p>
                }
                @if (store.nextCursorByStage()[stage.id]) {
                  <button
                    mat-button
                    type="button"
                    class="load-more"
                    [disabled]="store.loading()"
                    (click)="store.loadMoreStage(stage.id)"
                  >
                    {{ i18n.t('leads.loadMore') }}
                  </button>
                }
              </div>
            </article>
          }
        </section>
      } @else {
        <section class="panel lead-gantt" [attr.aria-busy]="store.loading()">
          <header class="gantt-range">
            <time>{{ i18n.date(timelineBounds().start) }}</time
            ><span>{{ store.leads().length }}</span
            ><time>{{ i18n.date(timelineBounds().end) }}</time>
          </header>
          <div class="gantt-grid">
            @for (lead of store.scheduledLeads(); track lead.id) {
              <a class="gantt-label" [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
              <div class="gantt-track">
                <a
                  class="gantt-bar"
                  [routerLink]="['/leads', lead.id]"
                  [style.inset-inline-start.%]="ganttStart(lead)"
                  [style.width.%]="ganttWidth(lead)"
                  ><span>{{ leadStageLabel(lead) }}</span></a
                >
              </div>
            } @empty {
              <p class="gantt-empty">{{ i18n.t('leads.unscheduled') }}</p>
            }
          </div>
          @if (unscheduledLeads().length) {
            <details class="unscheduled">
              <summary>{{ i18n.t('leads.unscheduled') }} · {{ unscheduledLeads().length }}</summary>
              <ul>
                @for (lead of unscheduledLeads(); track lead.id) {
                  <li>
                    <a [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
                  </li>
                }
              </ul>
            </details>
          }
        </section>
      }
      @if (selectedLead(); as lead) {
        <section class="panel assignment-panel">
          <app-record-assignments
            resourceType="lead"
            [resourceId]="lead.id"
            [version]="lead.version"
            (versionChange)="assignmentVersionChanged(lead, $event)"
          />
        </section>
      }
      @if (store.nextCursor()) {
        <button
          mat-stroked-button
          type="button"
          [disabled]="store.loading()"
          (click)="store.load(false)"
        >
          {{ i18n.t('leads.loadMore') }}
        </button>
      }
    </div>
  `,
  styles: `
    .editor > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0.75rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .editor h2 {
      margin: 0;
      font-size: 1rem;
    }
    .feature-form {
      grid-template-columns: repeat(3, minmax(11rem, 1fr));
      min-width: 0;
      padding: 1rem;
    }
    .form-error {
      grid-column: 1 / -1;
    }
    .record-list article {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 0.85rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .record-list article:last-child {
      border: 0;
    }
    .assignment-panel {
      padding: 1rem;
    }
    .view-switcher {
      width: fit-content;
    }
    .lead-kanban {
      display: grid;
      grid-auto-columns: minmax(17rem, 1fr);
      grid-auto-flow: column;
      gap: 1rem;
      overflow-x: auto;
      padding-bottom: 0.5rem;
    }
    .lead-stage {
      min-width: 17rem;
      border: 1px solid var(--border);
      border-radius: var(--panel-radius);
      background: var(--surface-subtle);
    }
    .lead-stage > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0.8rem;
    }
    .lead-drop-zone {
      min-height: 10rem;
      padding: 0 0.65rem 0.65rem;
    }
    .lead-card {
      position: relative;
      margin-bottom: 0.6rem;
      padding: 0.8rem;
      border: 1px solid var(--border);
      border-radius: 0.65rem;
      background: var(--surface-raised);
    }
    .lead-card h3 {
      margin: 0;
      font-size: 0.9rem;
    }
    .lead-card p {
      margin: 0.35rem 0;
    }
    .lead-card time {
      display: block;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .lead-card select {
      width: 100%;
      margin-top: 0.65rem;
    }
    .drag-handle {
      position: absolute;
      inset: 0.25rem 0.25rem auto auto;
      border: 0;
      color: var(--text-muted);
      background: transparent;
      cursor: grab;
    }
    .lead-placeholder {
      min-height: 6rem;
      border: 1px dashed var(--brand);
      border-radius: 0.65rem;
    }
    .stage-empty {
      padding: 1rem;
      color: var(--text-muted);
      text-align: center;
    }
    .lead-gantt {
      padding: 1rem;
      overflow: hidden;
    }
    .gantt-range {
      display: flex;
      justify-content: space-between;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .gantt-grid {
      display: grid;
      grid-template-columns: minmax(9rem, 15rem) minmax(30rem, 1fr);
      align-items: center;
      gap: 0.5rem 1rem;
      margin-top: 1rem;
      overflow-x: auto;
    }
    .gantt-label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .gantt-track {
      position: relative;
      height: 2rem;
      border-radius: 0.4rem;
      background: var(--surface-subtle);
    }
    .gantt-bar {
      position: absolute;
      inset-block: 0.2rem;
      display: flex;
      align-items: center;
      min-width: 2%;
      overflow: hidden;
      padding: 0 0.5rem;
      border-radius: 0.35rem;
      color: var(--brand-contrast);
      background: var(--brand);
      font-size: 0.72rem;
      text-decoration: none;
    }
    .gantt-bar span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .gantt-empty {
      grid-column: 1 / -1;
      color: var(--text-muted);
    }
    .unscheduled {
      margin-top: 1rem;
    }
    .record-main,
    .record-meta {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      min-width: 0;
    }
    .record-avatar {
      display: grid;
      width: 2.25rem;
      height: 2.25rem;
      flex: 0 0 auto;
      place-items: center;
      border-radius: 0.6rem;
      color: var(--brand);
      background: var(--brand-soft);
      font-weight: 700;
    }
    h2 {
      margin: 0;
      font-size: 0.9rem;
    }
    article p {
      margin: 0.2rem 0 0;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    article select {
      min-height: 2.25rem;
      appearance: none;
      padding-inline: 0.7rem 2.1rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      color: var(--text);
      background: var(--surface-raised);
      background-image:
        linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
        linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
      background-position:
        calc(100% - 0.9rem) 50%,
        calc(100% - 0.65rem) 50%;
      background-size:
        0.28rem 0.28rem,
        0.28rem 0.28rem;
      background-repeat: no-repeat;
    }
    @media (max-width: 760px) {
      .feature-form {
        grid-template-columns: 1fr;
      }
      .record-list article {
        align-items: stretch;
        flex-direction: column;
      }
      .record-meta {
        justify-content: space-between;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LeadsPage implements OnInit {
  readonly store = inject(LeadsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly selectedLead = signal<Lead | null>(null);
  readonly statuses = ['new', 'qualified', 'disqualified'] as const;
  readonly viewModes = ['list', 'kanban', 'gantt'] as const;
  readonly model = signal({
    name: '',
    email: '',
    phone: '',
    companyName: '',
    jobTitle: '',
    source: '',
    stageId: '',
  });
  readonly leadForm = form(this.model, (schema) => {
    required(schema.name);
    email(schema.email);
  });
  readonly nameInput = viewChild<ElementRef<HTMLInputElement>>('nameInput');
  private readonly injector = inject(Injector);

  ngOnInit(): void {
    void this.initialize();
  }

  openCreate(): void {
    this.store.formError.set(null);
    this.createOpen.set(true);
    focusAfterNextRender(this.injector, () => this.nameInput()?.nativeElement);
  }

  filter(event: Event): void {
    event.preventDefault();
    void this.store.load();
  }

  async retryStages(): Promise<void> {
    await this.store.loadStages();
    this.model.update((value) => ({ ...value, stageId: this.defaultStageId() }));
    if (!this.store.stageError()) await this.store.load();
  }

  inputValue(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }

  setFilterStatus(event: Event): void {
    this.store.status.set((event.target as HTMLSelectElement).value as LeadStatus | '');
  }

  changeStage(lead: Lead, event: Event): void {
    const stage = this.store
      .stages()
      .find((candidate) => candidate.id === (event.target as HTMLSelectElement).value);
    if (stage) void this.store.changeStage(lead, stage);
  }

  drop(event: CdkDragDrop<readonly Lead[]>, stage: LeadStage): void {
    const lead = event.item.data as Lead;
    if (lead.stageId !== stage.id) void this.store.changeStage(lead, stage);
  }

  moveFromSelect(lead: Lead, stageId: string): void {
    const stage = this.store.stages().find((candidate) => candidate.id === stageId);
    if (stage) void this.store.changeStage(lead, stage);
  }

  unscheduledLeads(): readonly Lead[] {
    return this.store.leads().filter((lead) => !lead.plannedStartDate && !lead.expectedCloseDate);
  }

  timelineBounds(): { start: string; end: string } {
    const dates = this.store
      .scheduledLeads()
      .flatMap((lead) => [lead.plannedStartDate, lead.expectedCloseDate])
      .filter((value): value is string => Boolean(value))
      .map((value) => new Date(`${value}T00:00:00`).getTime());
    const now = new Date();
    const start = dates.length ? Math.min(...dates) : now.getTime();
    const end = dates.length ? Math.max(...dates) : start + 86400000;
    return {
      start: new Date(start).toISOString(),
      end: new Date(Math.max(end, start + 86400000)).toISOString(),
    };
  }

  ganttStart(lead: Lead): number {
    const bounds = this.timelineBounds();
    const start = new Date(`${lead.plannedStartDate ?? lead.expectedCloseDate}T00:00:00`).getTime();
    return (
      ((start - new Date(bounds.start).getTime()) /
        (new Date(bounds.end).getTime() - new Date(bounds.start).getTime())) *
      100
    );
  }

  ganttWidth(lead: Lead): number {
    const bounds = this.timelineBounds();
    const start = new Date(`${lead.plannedStartDate ?? lead.expectedCloseDate}T00:00:00`).getTime();
    const end = new Date(`${lead.expectedCloseDate ?? lead.plannedStartDate}T00:00:00`).getTime();
    return Math.max(
      2,
      ((Math.max(end, start) - start + 86400000) /
        (new Date(bounds.end).getTime() - new Date(bounds.start).getTime())) *
        100,
    );
  }

  editableStages(): readonly LeadStage[] {
    return this.store
      .stages()
      .filter((stage) => stage.category !== 'converted' && stage.systemKey !== 'converted');
  }

  stageLabel(stage: LeadStage): string {
    return stage.systemKey ? this.i18n.t(this.statusKey(stage.systemKey)) : stage.displayName;
  }

  leadStageLabel(lead: Lead): string {
    const stage = this.store.stages().find((candidate) => candidate.id === lead.stageId);
    return stage ? this.stageLabel(stage) : this.i18n.t(this.statusKey(lead.status));
  }

  assignmentVersionChanged(lead: Lead, version: number): void {
    this.store.setVersion(lead.id, version);
    this.selectedLead.update((current) =>
      current?.id === lead.id ? { ...current, version } : current,
    );
  }

  statusKey(
    status: LeadStatus,
  ):
    | 'leads.status.new'
    | 'leads.status.qualified'
    | 'leads.status.disqualified'
    | 'leads.status.converted' {
    return `leads.status.${status}`;
  }

  async create(event: Event): Promise<void> {
    event.preventDefault();
    this.leadForm().markAsTouched();
    if (this.leadForm().invalid()) return;
    const value = this.model();
    const stage = this.store.stages().find((candidate) => candidate.id === value.stageId);
    if (!stage || stage.category === 'converted') return;
    const created = await this.store.create({
      name: value.name.trim(),
      email: trimmedOrNull(value.email),
      phone: trimmedOrNull(value.phone),
      companyName: trimmedOrNull(value.companyName),
      jobTitle: trimmedOrNull(value.jobTitle),
      source: trimmedOrNull(value.source),
      status: stage.category,
      stageId: stage.id,
      customFields: {},
    });
    if (!created) return;
    this.model.set({
      name: '',
      email: '',
      phone: '',
      companyName: '',
      jobTitle: '',
      source: '',
      stageId: this.defaultStageId(),
    });
    this.createOpen.set(false);
  }

  private async initialize(): Promise<void> {
    await this.store.loadStages();
    this.model.update((value) => ({ ...value, stageId: this.defaultStageId() }));
    await this.store.load();
  }

  private defaultStageId(): string {
    return (
      this.store.stages().find((stage) => stage.category === 'new' && stage.isDefault)?.id ??
      this.editableStages()[0]?.id ??
      ''
    );
  }
}
