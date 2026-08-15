import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  computed,
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
import { CalendarEventLegendComponent } from './calendar-event-legend.component';
import { CalendarMonthEventComponent } from './calendar-month-event.component';
import { CalendarTimelineComponent } from './calendar-timeline.component';
import { CalendarStore, calendarActivityIntersectsDay, type CalendarView } from './calendar.store';

type ActivityType = CalendarActivity['type'];
type VisibilityScope = CalendarActivity['visibilityScope'];

@Component({
  selector: 'app-calendar-page',
  imports: [
    ErrorPanelComponent,
    CalendarEventLegendComponent,
    CalendarMonthEventComponent,
    CalendarTimelineComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  providers: [CalendarStore],
  template: `
    <div class="calendar-page">
      <header class="calendar-header">
        <div class="title-block">
          <h1>{{ i18n.t('common.nav.calendar') }}</h1>
          <p>{{ i18n.t('calendar.subtitle') }}</p>
        </div>

        <div class="calendar-actions" [attr.aria-label]="i18n.t('calendar.controls')">
          <div
            class="segmented-control view-switcher"
            role="group"
            [attr.aria-label]="i18n.t('calendar.view')"
          >
            @for (view of views; track view) {
              <button
                mat-button
                type="button"
                [class.active]="store.view() === view"
                [attr.aria-pressed]="store.view() === view"
                (click)="store.setView(view)"
              >
                {{ i18n.t(viewKey(view)) }}
              </button>
            }
          </div>

          <div class="period-navigation">
            <button mat-stroked-button type="button" (click)="store.today()">
              {{ i18n.t('common.date.today') }}
            </button>
            <button
              mat-button
              type="button"
              class="icon-button"
              (click)="store.move(-1)"
              [attr.aria-label]="i18n.t('calendar.previous')"
            >
              <app-icon name="back" />
            </button>
            <button
              mat-button
              type="button"
              class="icon-button"
              (click)="store.move(1)"
              [attr.aria-label]="i18n.t('calendar.next')"
            >
              <app-icon name="chevron" />
            </button>
          </div>

          <label class="compact-select">
            <span class="visually-hidden">{{ i18n.t('calendar.typeFilter') }}</span>
            <select [value]="typeFilter()" (change)="setTypeFilter($event)">
              <option value="all">{{ i18n.t('calendar.allTypes') }}</option>
              @for (type of activityTypes; track type) {
                <option [value]="type">{{ i18n.t(typeKey(type)) }}</option>
              }
            </select>
          </label>

          @if (store.icsUrl(); as url) {
            <a
              mat-stroked-button
              class="export-action"
              [href]="url"
              download="calendar.ics"
              [attr.aria-label]="i18n.t('calendar.export')"
            >
              <app-icon name="download" />
            </a>
          }
          @if (permissions.allows('records.create')) {
            <button mat-flat-button type="button" class="create-action" (click)="openCreate()">
              <app-icon name="add" />{{ i18n.t('calendar.add') }}
            </button>
          }
        </div>
      </header>

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
            <button
              mat-button
              type="button"
              class="icon-button"
              (click)="closeCreate()"
              [attr.aria-label]="i18n.t('common.action.close')"
            >
              <app-icon name="close" />
            </button>
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

      <div class="calendar-workspace">
        <section class="calendar-surface" [attr.aria-busy]="store.loading()">
          <header class="surface-heading">
            <h2>{{ periodLabel() }}</h2>
            <span>{{ visibleActivities().length }} {{ i18n.t('calendar.eventsCount') }}</span>
          </header>

          @if (store.view() === 'month') {
            <div class="weekday-row" aria-hidden="true">
              @for (weekday of weekdayLabels(); track weekday) {
                <span>{{ weekday }}</span>
              }
            </div>
            <div
              class="calendar-grid month-view"
              role="group"
              [attr.aria-label]="i18n.t('common.nav.calendar')"
            >
              @if (store.loading() && store.activities().length === 0) {
                @for (placeholder of placeholders; track placeholder) {
                  <div class="skeleton day-cell"></div>
                }
              } @else {
                @for (day of visibleDays(); track day.key) {
                  <article
                    class="day-cell"
                    [class.outside]="!day.currentMonth"
                    [class.today]="isToday(day.date)"
                    [attr.aria-label]="dayAriaLabel(day.date, day.items.length)"
                  >
                    <button class="day-number" type="button" (click)="openCreate(day.date)">
                      <time [attr.datetime]="day.key">{{ day.date.getDate() }}</time>
                    </button>
                    <div class="day-events">
                      @for (item of day.items.slice(0, eventLimit()); track item.id) {
                        <app-calendar-month-event [item]="item" />
                      }
                      @if (day.items.length > eventLimit()) {
                        <span class="more-events"
                          >+{{ day.items.length - eventLimit() }}
                          {{ i18n.t('calendar.more') }}</span
                        >
                      }
                    </div>
                  </article>
                }
              }
            </div>
          } @else {
            <app-calendar-timeline
              [days]="visibleDays()"
              [view]="store.view() === 'day' ? 'day' : 'week'"
              (createRequested)="openCreate($event)"
            />
          }
        </section>

        <aside class="calendar-insights" [attr.aria-label]="i18n.t('calendar.insights')">
          <section class="insight-card mini-calendar">
            <header>
              <div>
                <h2>{{ i18n.t('calendar.miniCalendar') }}</h2>
                <p>{{ monthLabel() }}</p>
              </div>
              <div class="mini-navigation">
                <button
                  type="button"
                  (click)="store.moveMonth(-1)"
                  [attr.aria-label]="i18n.t('calendar.previous')"
                >
                  <app-icon name="back" />
                </button>
                <button
                  type="button"
                  (click)="store.moveMonth(1)"
                  [attr.aria-label]="i18n.t('calendar.next')"
                >
                  <app-icon name="chevron" />
                </button>
              </div>
            </header>
            <div class="mini-weekdays" aria-hidden="true">
              @for (weekday of weekdayLabels(); track weekday) {
                <span>{{ weekday }}</span>
              }
            </div>
            <div class="mini-days">
              @for (day of miniDays(); track day.key) {
                <button
                  type="button"
                  [class.outside]="!day.currentMonth"
                  [class.today]="isToday(day.date)"
                  [class.has-events]="day.items.length > 0"
                  (click)="selectDay(day.date)"
                  [attr.aria-label]="dayAriaLabel(day.date, day.items.length)"
                >
                  {{ day.date.getDate() }}
                </button>
              }
            </div>
          </section>

          <section class="insight-card upcoming-card">
            <header>
              <h2>{{ i18n.t('calendar.upcoming') }}</h2>
              <span>{{ upcomingActivities().length }}</span>
            </header>
            <div class="upcoming-list">
              @for (item of upcomingActivities(); track item.id) {
                <article>
                  <i [class]="'type-' + item.type"></i>
                  <div>
                    <span class="upcoming-type">{{ i18n.t(typeKey(item.type)) }}</span>
                    <time [attr.datetime]="item.occurredAt">{{ upcomingDate(item) }}</time>
                    <strong>{{ item.title }}</strong>
                    <span class="upcoming-scope">{{ i18n.t(scopeKey(item.visibilityScope)) }}</span>
                  </div>
                </article>
              } @empty {
                <p class="empty-copy">{{ i18n.t('calendar.noUpcoming') }}</p>
              }
            </div>
          </section>

          <app-calendar-event-legend />

          <section class="insight-card calendars-card">
            <header>
              <h2>{{ i18n.t('calendar.calendars') }}</h2>
            </header>
            <div class="calendar-filters">
              @for (scope of scopes; track scope) {
                <label>
                  <input
                    type="checkbox"
                    [checked]="visibleScopes().has(scope)"
                    (change)="toggleScope(scope)"
                  />
                  <span [class]="'scope-swatch scope-' + scope"></span>
                  <span>{{ i18n.t(scopeKey(scope)) }}</span>
                  <strong>{{ scopeCount(scope) }}</strong>
                </label>
              }
            </div>
          </section>
        </aside>
      </div>
    </div>
  `,
  styleUrl: './calendar.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarPage implements OnInit {
  readonly store = inject(CalendarStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly validationError = signal(false);
  readonly typeFilter = signal<'all' | ActivityType>('all');
  readonly visibleScopes = signal<ReadonlySet<VisibilityScope>>(
    new Set(['workspace', 'department', 'user']),
  );
  readonly views = ['day', 'week', 'month'] as const;
  readonly activityTypes = ['meeting', 'call', 'task', 'note'] as const;
  readonly scopes = ['workspace', 'department', 'user'] as const;
  readonly placeholders = Array.from({ length: 14 }, (_, index) => index);

  readonly visibleActivities = computed(() =>
    this.store.activities().filter((item) => this.matchesFilters(item)),
  );
  readonly visibleDays = computed(() => {
    const activities = this.visibleActivities();
    return this.store.days().map((day) => ({
      ...day,
      items: activities
        .filter((item) =>
          calendarActivityIntersectsDay(item, day.date, this.i18n.currentTimeZone()),
        )
        .sort((left, right) => Date.parse(left.occurredAt) - Date.parse(right.occurredAt)),
    }));
  });
  readonly upcomingActivities = computed(() => {
    const now = Date.now();
    const future = [...this.visibleActivities()]
      .filter(
        (item) => Date.parse(item.endsAt ?? item.occurredAt) >= now && item.status !== 'cancelled',
      )
      .sort((left, right) => Date.parse(left.occurredAt) - Date.parse(right.occurredAt));
    return future.slice(0, 5);
  });
  readonly weekdayLabels = computed(() =>
    Array.from({ length: 7 }, (_, index) =>
      this.i18n.date(new Date(Date.UTC(2026, 0, 5 + index, 12)), {
        weekday: 'short',
        timeZone: 'UTC',
      }),
    ),
  );
  readonly miniDays = computed(() => {
    const anchor = this.store.anchor();
    const first = new Date(anchor.getFullYear(), anchor.getMonth(), 1);
    const start = addDays(first, -((first.getDay() + 6) % 7));
    return Array.from({ length: 42 }, (_, index) => {
      const date = addDays(start, index);
      const key = dateKey(date);
      return {
        date,
        key,
        currentMonth: date.getMonth() === anchor.getMonth(),
        items: this.store
          .activities()
          .filter((item) => calendarActivityIntersectsDay(item, date, this.i18n.currentTimeZone())),
      };
    });
  });

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

  openCreate(date = new Date()): void {
    this.validationError.set(false);
    const start = new Date(date);
    const now = new Date();
    start.setHours(
      dateKey(date) === dateKey(now) ? now.getHours() : 9,
      dateKey(date) === dateKey(now) ? Math.ceil(now.getMinutes() / 30) * 30 : 0,
      0,
      0,
    );
    if (start.getMinutes() === 60) start.setHours(start.getHours() + 1, 0, 0, 0);
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

  periodLabel(): string {
    if (this.store.view() === 'month') return this.monthLabel();
    const range = this.store.range();
    return `${this.calendarDate(range.start, { dateStyle: 'medium' })} – ${this.calendarDate(addDays(range.end, -1), { dateStyle: 'medium' })}`;
  }

  monthLabel(): string {
    return this.calendarDate(this.store.anchor(), { month: 'long', year: 'numeric' });
  }

  eventLimit(): number {
    return this.store.view() === 'month' ? 3 : 12;
  }

  upcomingDate(item: CalendarActivity): string {
    return this.i18n.date(item.occurredAt, {
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    });
  }

  isToday(date: Date): boolean {
    return dateKey(date) === dateKey(new Date());
  }

  dayAriaLabel(date: Date, count: number): string {
    return `${this.calendarDate(date, { dateStyle: 'full' })}, ${count} ${this.i18n.t('calendar.eventsCount')}`;
  }

  viewKey(view: CalendarView): 'calendar.view.day' | 'calendar.view.week' | 'calendar.view.month' {
    return `calendar.view.${view}`;
  }

  typeKey(
    type: ActivityType,
  ):
    | 'activities.activity.task'
    | 'activities.activity.call'
    | 'activities.activity.meeting'
    | 'activities.activity.note' {
    return `activities.activity.${type}`;
  }

  scopeKey(
    scope: VisibilityScope,
  ): 'calendar.audience.workspace' | 'calendar.audience.department' | 'calendar.audience.user' {
    return `calendar.audience.${scope}`;
  }

  setTypeFilter(event: Event): void {
    this.typeFilter.set((event.target as HTMLSelectElement).value as 'all' | ActivityType);
  }

  toggleScope(scope: VisibilityScope): void {
    const next = new Set(this.visibleScopes());
    if (next.has(scope) && next.size > 1) next.delete(scope);
    else next.add(scope);
    this.visibleScopes.set(next);
  }

  scopeCount(scope: VisibilityScope): number {
    return this.store.activities().filter((item) => item.visibilityScope === scope).length;
  }

  async selectDay(date: Date): Promise<void> {
    await this.store.selectDate(date, 'day');
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
    )
      return;
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

  private calendarDate(date: Date, options: Intl.DateTimeFormatOptions): string {
    const value = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), 12));
    return this.i18n.date(value, { ...options, timeZone: 'UTC' });
  }

  private matchesFilters(item: CalendarActivity): boolean {
    return (
      (this.typeFilter() === 'all' || item.type === this.typeFilter()) &&
      this.visibleScopes().has(item.visibilityScope)
    );
  }
}

function localDateTime(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function dateKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}
