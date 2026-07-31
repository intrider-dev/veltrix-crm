import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';

import type { CalendarActivity, CalendarActivityInput } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { trimmedOrNull } from '../../shared/forms/feature-validation';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { CalendarStore, type CalendarView } from './calendar.store';

@Component({
  selector: 'app-calendar-page',
  imports: [
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  providers: [CalendarStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.calendar') }}</h1>
          <p>{{ rangeLabel() }}</p>
        </div>
        <div class="header-actions">
          @if (store.icsUrl(); as url) {
            <a mat-stroked-button [href]="url" download="calendar.ics">{{
              i18n.t('calendar.export')
            }}</a>
          }
          @if (permissions.allows('records.create')) {
            <button mat-flat-button type="button" (click)="openCreate()">
              <app-icon name="add" />{{ i18n.t('calendar.add') }}
            </button>
          }
        </div>
      </header>

      <section class="panel calendar-toolbar" [attr.aria-label]="i18n.t('calendar.controls')">
        <div
          class="segmented segmented-control"
          role="group"
          [attr.aria-label]="i18n.t('calendar.view')"
        >
          @for (view of views; track view) {
            <button
              mat-button
              type="button"
              [class.active]="store.view() === view"
              (click)="store.setView(view)"
            >
              {{ i18n.t(viewKey(view)) }}
            </button>
          }
        </div>
        <div class="date-navigation">
          <button
            mat-button
            type="button"
            (click)="store.move(-1)"
            [attr.aria-label]="i18n.t('calendar.previous')"
          >
            <app-icon name="back" />
          </button>
          <button mat-stroked-button type="button" (click)="store.today()">
            {{ i18n.t('common.date.today') }}
          </button>
          <button
            mat-button
            type="button"
            (click)="store.move(1)"
            [attr.aria-label]="i18n.t('calendar.next')"
          >
            <app-icon name="chevron" />
          </button>
        </div>
      </section>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      @if (validationError()) {
        <div class="error-panel" role="alert">{{ i18n.t('calendar.endAfterStart') }}</div>
      }

      @if (createOpen()) {
        <section class="panel editor" aria-labelledby="calendar-create-title">
          <header>
            <h2 id="calendar-create-title">{{ i18n.t('calendar.createTitle') }}</h2>
          </header>
          <form class="feature-form" (submit)="create($event)" novalidate>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('activities.activity.title') }}</mat-label>
              <input #titleInput matInput [formField]="activityForm.title" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('web.activity.type') }}</mat-label>
              <mat-select [formField]="activityForm.type">
                <mat-option value="task">{{ i18n.t('activities.activity.task') }}</mat-option>
                <mat-option value="call">{{ i18n.t('activities.activity.call') }}</mat-option>
                <mat-option value="meeting">{{ i18n.t('activities.activity.meeting') }}</mat-option>
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('calendar.start') }}</mat-label>
              <input matInput type="datetime-local" [formField]="activityForm.start" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('calendar.end') }}</mat-label>
              <input matInput type="datetime-local" [formField]="activityForm.end" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('calendar.location') }}</mat-label>
              <input matInput [formField]="activityForm.location" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('calendar.audience') }}</mat-label>
              <mat-select
                [formField]="activityForm.visibilityScope"
                (selectionChange)="scopeChanged()"
              >
                <mat-option value="workspace">{{
                  i18n.t('calendar.audience.workspace')
                }}</mat-option>
                <mat-option value="user">{{ i18n.t('calendar.audience.user') }}</mat-option>
                @if (store.departments().length) {
                  <mat-option value="department">{{
                    i18n.t('calendar.audience.department')
                  }}</mat-option>
                }
              </mat-select>
            </mat-form-field>
            @if (model().visibilityScope === 'user') {
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('calendar.targetUser') }}</mat-label>
                <mat-select [formField]="activityForm.scopeUserId">
                  @if (store.currentUser(); as currentUser) {
                    <mat-option [value]="currentUser.id">{{ currentUser.displayName }}</mat-option>
                  }
                  @for (member of store.members(); track member.id) {
                    @if (member.userId !== store.currentUser()?.id) {
                      <mat-option [value]="member.userId">{{ member.displayName }}</mat-option>
                    }
                  }
                </mat-select>
              </mat-form-field>
            } @else if (model().visibilityScope === 'department') {
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('calendar.targetDepartment') }}</mat-label>
                <mat-select [formField]="activityForm.scopeDepartmentId">
                  @for (department of store.departments(); track department.id) {
                    <mat-option [value]="department.id">{{ department.name }}</mat-option>
                  }
                </mat-select>
              </mat-form-field>
            }
            <div class="form-actions">
              <button mat-button type="button" (click)="closeCreate()">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button mat-flat-button type="submit" [disabled]="store.saving()">
                {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
              </button>
            </div>
          </form>
        </section>
      }

      <section
        class="calendar-grid"
        [class.day-view]="store.view() === 'day'"
        [attr.aria-busy]="store.loading()"
        [attr.aria-label]="i18n.t('common.nav.calendar')"
        tabindex="0"
      >
        @if (store.loading() && store.activities().length === 0) {
          @for (placeholder of placeholders; track placeholder) {
            <div class="panel skeleton day-card"></div>
          }
        } @else {
          @for (day of store.days(); track day.key) {
            <article class="panel day-card" [class.outside]="!day.currentMonth">
              <header>
                <time [attr.datetime]="day.key">{{
                  i18n.date(day.date, {
                    weekday: 'short',
                    day: 'numeric',
                    month: store.view() === 'month' ? undefined : 'short',
                  })
                }}</time>
                @if (day.items.length) {
                  <span>{{ day.items.length }}</span>
                }
              </header>
              <div class="day-events">
                @for (item of day.items; track item.id) {
                  <div class="calendar-event" [class.completed]="item.status === 'completed'">
                    <time [attr.datetime]="item.occurredAt">{{
                      i18n.date(item.occurredAt, { hour: '2-digit', minute: '2-digit' })
                    }}</time>
                    <strong>{{ item.title }}</strong>
                    <span class="scope-badge">{{ i18n.t(scopeKey(item.visibilityScope)) }}</span>
                    @if (item.location) {
                      <small>{{ item.location }}</small>
                    }
                  </div>
                } @empty {
                  <span class="no-events">{{ i18n.t('calendar.noEvents') }}</span>
                }
              </div>
            </article>
          }
        }
      </section>
    </div>
  `,
  styles: `
    .header-actions,
    .calendar-toolbar,
    .date-navigation {
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .calendar-toolbar {
      justify-content: space-between;
      padding: 0.55rem;
    }
    .segmented {
      display: flex;
      gap: 0.15rem;
      padding: 0.15rem;
      border-radius: 0.55rem;
      background: var(--surface-subtle);
    }
    .segmented .active {
      color: var(--brand);
      background: var(--surface-raised);
    }
    .editor {
      padding: 1rem;
    }
    .editor h2 {
      margin: 0 0 1rem;
      font-size: 1rem;
    }
    .feature-form {
      grid-template-columns: repeat(3, minmax(12rem, 1fr));
    }
    .calendar-grid {
      display: grid;
      grid-template-columns: repeat(7, minmax(8rem, 1fr));
      gap: 0.5rem;
      overflow-x: auto;
    }
    .calendar-grid:focus-visible {
      outline: 3px solid var(--brand);
      outline-offset: 2px;
    }
    .calendar-grid.day-view {
      grid-template-columns: minmax(18rem, 1fr);
    }
    .day-card {
      min-height: 8.5rem;
      overflow: hidden;
    }
    .day-card > header {
      display: flex;
      justify-content: space-between;
      padding: 0.55rem 0.65rem;
      border-bottom: 1px solid var(--border);
      color: var(--text-muted);
      font-size: 0.72rem;
    }
    .day-card.outside {
      border-style: dashed;
    }
    .day-events {
      display: grid;
      gap: 0.35rem;
      padding: 0.45rem;
    }
    .calendar-event {
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 0.25rem 0.45rem;
      min-width: 0;
      padding: 0.4rem;
      border-radius: 0.4rem;
      background: var(--brand-soft);
      font-size: 0.72rem;
    }
    .calendar-event time {
      color: var(--brand);
    }
    .calendar-event strong {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .scope-badge {
      grid-column: 2;
      color: var(--text-muted);
      font-size: 0.65rem;
    }
    .calendar-event small {
      grid-column: 2;
      color: var(--text-muted);
    }
    .calendar-event.completed {
      border-left-color: var(--border-strong);
      background: var(--surface-subtle);
    }
    .calendar-event.completed strong {
      text-decoration: line-through;
    }
    .no-events {
      color: var(--text-faint);
      font-size: 0.68rem;
    }
    @media (max-width: 760px) {
      .header-actions,
      .calendar-toolbar {
        align-items: stretch;
        flex-direction: column;
      }
      .feature-form {
        grid-template-columns: 1fr;
      }
      .calendar-grid {
        grid-template-columns: 1fr;
        overflow: visible;
      }
      .day-card.outside {
        display: none;
      }
    }

    :host {
      --workspace-surface: var(--color-surface, var(--surface-raised));
      --workspace-subtle: var(--color-surface-subtle, var(--surface-subtle));
      --workspace-hover: var(--color-surface-hover, var(--surface-selected));
      --workspace-border: var(--color-border, var(--border));
      --workspace-anchor: var(--color-anchor, var(--brand));
    }
    .page-header {
      align-items: flex-end;
      margin-bottom: 0.25rem;
    }
    .header-actions {
      flex-wrap: wrap;
      justify-content: flex-end;
    }
    .calendar-toolbar {
      min-height: 4rem;
      padding: 0.75rem;
      border-radius: var(--radius-panel, 0.875rem);
      background: var(--workspace-surface);
    }
    .segmented {
      border-radius: var(--radius-control, 0.625rem);
      background: var(--workspace-subtle);
    }
    .segmented .active {
      color: var(--workspace-anchor);
      background: var(--workspace-surface);
      box-shadow: 0 1px 2px rgb(18 36 29 / 10%);
    }
    .date-navigation {
      justify-content: flex-end;
    }
    .date-navigation > button:first-child,
    .date-navigation > button:last-child {
      min-width: 2.5rem;
      padding-inline: 0.5rem;
    }
    .editor {
      overflow: hidden;
      padding: 0;
      border-radius: var(--radius-panel, 0.875rem);
      background: var(--workspace-surface);
    }
    .editor > header {
      min-height: 3.5rem;
      padding: 1rem 1.25rem;
      border-bottom: 1px solid var(--workspace-border);
      background: var(--workspace-subtle);
    }
    .editor h2 {
      margin: 0;
    }
    .feature-form {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.25rem 1rem;
      padding: 1.25rem;
    }
    .form-actions {
      grid-column: 1 / -1;
    }
    .calendar-grid {
      gap: 0;
      overflow: auto;
      border: 1px solid var(--workspace-border);
      border-radius: var(--radius-panel, 0.875rem);
      background: var(--workspace-surface);
    }
    .calendar-grid:focus-visible {
      outline: 2px solid var(--workspace-anchor);
      outline-offset: 2px;
    }
    .day-card {
      min-height: 9.5rem;
      border: 0;
      border-right: 1px solid color-mix(in srgb, var(--workspace-border) 72%, transparent);
      border-bottom: 1px solid color-mix(in srgb, var(--workspace-border) 72%, transparent);
      border-radius: 0;
      background: var(--workspace-surface);
    }
    .day-card > header {
      min-height: 2.5rem;
      align-items: center;
      padding: 0.625rem 0.75rem;
      border-color: color-mix(in srgb, var(--workspace-border) 72%, transparent);
      background: var(--workspace-subtle);
    }
    .day-card > header span {
      display: grid;
      min-width: 1.5rem;
      min-height: 1.5rem;
      place-items: center;
      border-radius: var(--radius-pill, 999px);
      background: var(--workspace-surface);
    }
    .day-card.outside {
      border-style: solid;
      color: var(--text-muted);
      background: color-mix(in srgb, var(--workspace-subtle) 54%, var(--workspace-surface));
    }
    .day-events {
      gap: 0.375rem;
      padding: 0.5rem;
    }
    .calendar-event {
      gap: 0.25rem 0.5rem;
      padding: 0.5rem;
      border-left: 3px solid var(--workspace-anchor);
      border-radius: var(--radius-control, 0.625rem);
      background: var(--color-signal-soft, var(--brand-soft));
    }
    .calendar-event time {
      color: var(--workspace-anchor);
    }
    @media (max-width: 760px) {
      .page-header {
        align-items: stretch;
      }
      .header-actions,
      .calendar-toolbar {
        align-items: stretch;
      }
      .header-actions > *,
      .segmented,
      .date-navigation {
        width: 100%;
      }
      .segmented button,
      .date-navigation button {
        flex: 1;
      }
      .feature-form {
        grid-template-columns: 1fr;
        padding: 1rem;
      }
      .form-actions {
        grid-column: 1;
      }
      .calendar-grid {
        gap: 0.625rem;
        overflow: visible;
        border: 0;
        background: transparent;
      }
      .day-card {
        min-height: 7rem;
        border: 1px solid var(--workspace-border);
        border-radius: var(--radius-panel, 0.875rem);
      }
    }
    @media (forced-colors: active) {
      .segmented .active,
      .calendar-event,
      .day-card > header span {
        border: 1px solid CanvasText;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarPage implements OnInit {
  readonly store = inject(CalendarStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly validationError = signal(false);
  readonly views = ['day', 'week', 'month'] as const;
  readonly placeholders = [1, 2, 3, 4, 5, 6, 7] as const;
  readonly model = signal<{
    readonly type: CalendarActivityInput['type'];
    readonly title: string;
    readonly start: string;
    readonly end: string;
    readonly location: string;
    readonly visibilityScope: NonNullable<CalendarActivityInput['visibilityScope']>;
    readonly scopeUserId: string;
    readonly scopeDepartmentId: string;
  }>({
    type: 'meeting',
    title: '',
    start: '',
    end: '',
    location: '',
    visibilityScope: 'workspace',
    scopeUserId: '',
    scopeDepartmentId: '',
  });
  readonly activityForm = form(this.model, (schema) => {
    required(schema.title);
    required(schema.start);
    required(schema.end);
  });
  readonly titleInput = viewChild<ElementRef<HTMLInputElement>>('titleInput');
  private readonly injector = inject(Injector);

  ngOnInit(): void {
    void this.store.load();
    void this.store.loadAudienceOptions();
  }

  openCreate(): void {
    this.validationError.set(false);
    const start = new Date();
    start.setMinutes(Math.ceil(start.getMinutes() / 30) * 30, 0, 0);
    const end = new Date(start.getTime() + 3_600_000);
    this.model.update((value) => ({
      ...value,
      start: localDateTime(start),
      end: localDateTime(end),
      visibilityScope: 'workspace',
      scopeUserId: '',
      scopeDepartmentId: '',
    }));
    this.createOpen.set(true);
    focusAfterNextRender(this.injector, () => this.titleInput()?.nativeElement);
  }

  closeCreate(): void {
    this.createOpen.set(false);
    this.validationError.set(false);
  }

  rangeLabel(): string {
    const range = this.store.range();
    return `${this.i18n.date(range.start, { dateStyle: 'medium' })} – ${this.i18n.date(new Date(range.end.getTime() - 1), { dateStyle: 'medium' })}`;
  }

  viewKey(view: CalendarView): 'calendar.view.day' | 'calendar.view.week' | 'calendar.view.month' {
    return `calendar.view.${view}`;
  }

  scopeKey(
    scope: CalendarActivity['visibilityScope'],
  ): 'calendar.audience.workspace' | 'calendar.audience.department' | 'calendar.audience.user' {
    return `calendar.audience.${scope}`;
  }

  scopeChanged(): void {
    const scope = this.model().visibilityScope;
    this.model.update((value) => ({
      ...value,
      scopeUserId: scope === 'user' ? (this.store.currentUser()?.id ?? '') : '',
      scopeDepartmentId: '',
    }));
  }

  async create(event: Event): Promise<void> {
    event.preventDefault();
    if (this.activityForm().invalid()) {
      this.activityForm().markAsTouched();
      return;
    }
    const value = this.model();
    const start = new Date(value.start);
    const end = new Date(value.end);
    if (end <= start) {
      this.validationError.set(true);
      return;
    }
    if (
      (value.visibilityScope === 'user' && !value.scopeUserId) ||
      (value.visibilityScope === 'department' && !value.scopeDepartmentId)
    ) {
      return;
    }
    this.validationError.set(false);
    await this.store.create({
      type: value.type,
      title: value.title.trim(),
      status: 'open',
      priority: 'normal',
      occurredAt: start.toISOString(),
      endsAt: end.toISOString(),
      location: trimmedOrNull(value.location),
      visibilityScope: value.visibilityScope,
      scopeUserId: value.visibilityScope === 'user' ? value.scopeUserId : null,
      scopeDepartmentId: value.visibilityScope === 'department' ? value.scopeDepartmentId : null,
    });
    this.model.set({
      type: 'meeting',
      title: '',
      start: '',
      end: '',
      location: '',
      visibilityScope: 'workspace',
      scopeUserId: '',
      scopeDepartmentId: '',
    });
    this.closeCreate();
  }
}

function localDateTime(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}
