import { A11yModule } from '@angular/cdk/a11y';
import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { RouterLink } from '@angular/router';

import type { Activity, CreateActivity } from '../../core/api/api.types';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ActivitiesStore } from './activities.store';

type ActivityTypeFilter = 'all' | Activity['type'];
type ActivityStatusFilter = 'all' | Activity['status'];
type ActivityPriorityFilter = 'all' | NonNullable<Activity['priority']>;

@Component({
  selector: 'app-activities-page',
  imports: [
    A11yModule,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    RecordAssignmentsComponent,
    RouterLink,
  ],
  providers: [ActivitiesStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div class="page-heading">
          <div class="title-line">
            <h1>{{ i18n.t('activities.page.title') }}</h1>
            <span class="count-badge">{{ taskCount() }}</span>
          </div>
          <p>{{ i18n.t('activities.page.subtitle') }}</p>
        </div>
        <label class="header-search">
          <span class="visually-hidden">{{ i18n.t('activities.filters.search') }}</span>
          <app-icon name="search" />
          <input
            type="search"
            [placeholder]="i18n.t('activities.filters.search')"
            [value]="query()"
            (input)="query.set($any($event.target).value)"
          />
          <kbd>Ctrl K</kbd>
        </label>
        <button class="create-task" mat-flat-button type="button" (click)="openCreate()">
          <app-icon name="add" />{{ i18n.t('activities.activity.createTask') }}
        </button>
      </header>

      <section
        class="summary-grid"
        tabindex="0"
        [attr.aria-label]="i18n.t('activities.summary.title')"
      >
        <article class="summary-card summary-card--violet">
          <span class="summary-icon"><app-icon name="activity" /></span>
          <div>
            <small>{{ i18n.t('activities.summary.total') }}</small
            ><strong>{{ taskCount() }}</strong>
          </div>
        </article>
        <article class="summary-card summary-card--blue">
          <span class="summary-icon"><app-icon name="clock" /></span>
          <div>
            <small>{{ i18n.t('activities.summary.open') }}</small
            ><strong>{{ openTaskCount() }}</strong>
          </div>
        </article>
        <article class="summary-card summary-card--green">
          <span class="summary-icon"><app-icon name="check" /></span>
          <div>
            <small>{{ i18n.t('activities.summary.completed') }}</small
            ><strong>{{ completedTaskCount() }}</strong>
          </div>
        </article>
        <article class="summary-card summary-card--red">
          <span class="summary-icon"><app-icon name="notification" /></span>
          <div>
            <small>{{ i18n.t('activities.summary.overdue') }}</small
            ><strong>{{ overdueTasks().length }}</strong>
          </div>
        </article>
        <article class="summary-card summary-card--orange">
          <span class="summary-icon"><app-icon name="pin" /></span>
          <div>
            <small>{{ i18n.t('activities.summary.highPriority') }}</small
            ><strong>{{ highPriorityTaskCount() }}</strong>
          </div>
        </article>
        <article class="summary-card summary-card--teal">
          <span class="summary-icon"><app-icon name="calendar" /></span>
          <div>
            <small>{{ i18n.t('activities.summary.dueToday') }}</small
            ><strong>{{ dueTodayCount() }}</strong>
          </div>
        </article>
      </section>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      <div class="task-workspace">
        <section class="panel activity-list" [attr.aria-busy]="store.loading()">
          <div
            class="type-tabs segmented-control"
            role="group"
            [attr.aria-label]="i18n.t('activities.filters.type')"
          >
            @for (filter of typeFilters; track filter.value) {
              <button
                type="button"
                [class.selected]="typeFilter() === filter.value"
                (click)="typeFilter.set(filter.value)"
              >
                {{ i18n.t(filter.label) }}
                @if (filter.value === 'task') {
                  <span>{{ taskCount() }}</span>
                }
              </button>
            }
          </div>

          <div class="filter-toolbar" [attr.aria-label]="i18n.t('activities.filters.title')">
            <label
              ><span>{{ i18n.t('common.field.status') }}</span
              ><mat-select
                panelClass="crm-select-panel"
                class="crm-select"
                [aria-label]="i18n.t('common.field.status')"
                [value]="statusFilter()"
                (selectionChange)="statusFilter.set($event.value)"
              >
                <mat-option value="all">{{ i18n.t('activities.filters.allStatuses') }}</mat-option>
                <mat-option value="open">{{ i18n.t('common.status.open') }}</mat-option>
                <mat-option value="completed">{{ i18n.t('common.status.completed') }}</mat-option>
                <mat-option value="cancelled">{{
                  i18n.t('activities.status.cancelled')
                }}</mat-option>
              </mat-select></label
            >
            <label
              ><span>{{ i18n.t('activities.activity.priority') }}</span
              ><mat-select
                panelClass="crm-select-panel"
                class="crm-select"
                [aria-label]="i18n.t('activities.activity.priority')"
                [value]="priorityFilter()"
                (selectionChange)="priorityFilter.set($event.value)"
              >
                <mat-option value="all">{{
                  i18n.t('activities.filters.allPriorities')
                }}</mat-option>
                <mat-option value="high">{{ i18n.t('activities.priority.high') }}</mat-option>
                <mat-option value="normal">{{ i18n.t('activities.priority.normal') }}</mat-option>
                <mat-option value="low">{{ i18n.t('activities.priority.low') }}</mat-option>
              </mat-select></label
            >
            <label
              ><span>{{ i18n.t('activities.filters.due') }}</span
              ><mat-select
                panelClass="crm-select-panel"
                class="crm-select"
                [aria-label]="i18n.t('activities.filters.due')"
                [value]="dueFilter()"
                (selectionChange)="dueFilter.set($event.value)"
              >
                <mat-option value="all">{{ i18n.t('activities.filters.anyDate') }}</mat-option>
                <mat-option value="today">{{ i18n.t('common.date.today') }}</mat-option>
                <mat-option value="overdue">{{ i18n.t('activities.summary.overdue') }}</mat-option>
                <mat-option value="none">{{ i18n.t('activities.filters.noDueDate') }}</mat-option>
              </mat-select></label
            >
            <button mat-button type="button" (click)="resetFilters()">
              {{ i18n.t('activities.filters.reset') }}
            </button>
          </div>

          <div class="activity-columns" aria-hidden="true">
            <span></span><span>{{ i18n.t('activities.columns.activity') }}</span
            ><span>{{ i18n.t('activities.filters.type') }}</span
            ><span>{{ i18n.t('common.field.status') }}</span
            ><span>{{ i18n.t('activities.activity.priority') }}</span
            ><span>{{ i18n.t('activities.activity.due') }}</span
            ><span></span>
          </div>

          @if (store.loading()) {
            <div class="list-skeleton">
              <div class="skeleton"></div>
              <div class="skeleton"></div>
              <div class="skeleton"></div>
            </div>
          } @else if (filteredActivities().length === 0) {
            <div class="empty-state">{{ i18n.t('activities.empty.filtered') }}</div>
          } @else {
            @for (activity of filteredActivities(); track activity.id) {
              <article
                [class.is-completed]="activity.status === 'completed'"
                [class.is-overdue-row]="isOverdue(activity)"
              >
                <span
                  class="row-check"
                  [class.checked]="activity.status === 'completed'"
                  aria-hidden="true"
                  ><app-icon [name]="activity.status === 'completed' ? 'check' : 'activity'"
                /></span>
                <div class="activity-summary">
                  <h2>{{ activity.title }}</h2>
                  @if (activity.body) {
                    <p>{{ activity.body }}</p>
                  }
                </div>
                <span class="type-pill">{{ i18n.t(typeKey(activity)) }}</span>
                <span class="status-pill" [attr.data-status]="activity.status">{{
                  i18n.t(statusKey(activity))
                }}</span>
                <span class="priority-pill" [attr.data-priority]="activity.priority || 'normal'"
                  ><app-icon name="pin" />{{ i18n.t(priorityKey(activity)) }}</span
                >
                <time
                  [class.is-overdue]="isOverdue(activity)"
                  [attr.datetime]="activity.dueAt || activity.occurredAt"
                  >{{
                    activity.dueAt
                      ? i18n.date(activity.dueAt, { dateStyle: 'medium', timeStyle: 'short' })
                      : i18n.t('activities.filters.noDueDate')
                  }}</time
                >
                <div class="row-actions">
                  @if (activity.type === 'task' && activity.status === 'open') {
                    <button mat-button type="button" (click)="store.complete(activity)">
                      {{ i18n.t('activities.activity.complete') }}
                    </button>
                  }
                  @if (activity.type === 'task') {
                    <button mat-button type="button" (click)="toggleAssignments(activity)">
                      {{ i18n.t('assignments.manage') }}
                    </button>
                  }
                </div>
                @if (selectedTaskId() === activity.id) {
                  <div class="task-assignments">
                    <app-record-assignments
                      resourceType="task"
                      [resourceId]="activity.id"
                      [version]="activity.version"
                      (versionChange)="store.setVersion(activity.id, $event)"
                    />
                  </div>
                }
              </article>
            }
          }
        </section>

        <aside class="task-insights">
          <section class="insight-card mini-calendar">
            <header>
              <h2>{{ i18n.t('common.nav.calendar') }}</h2>
              <a [routerLink]="['/calendar']">{{ i18n.t('common.action.viewAll') }}</a>
            </header>
            <strong>{{ monthLabel() }}</strong>
            <div class="weekdays" aria-hidden="true">
              @for (day of weekdayLabels(); track day) {
                <span>{{ day }}</span>
              }
            </div>
            <div class="month-grid">
              @for (day of calendarDays(); track day.key) {
                <span
                  [class.outside]="!day.currentMonth"
                  [class.today]="day.today"
                  [class.has-task]="day.hasTask"
                  >{{ day.day }}</span
                >
              }
            </div>
          </section>

          <section class="insight-card priority-card">
            <h2>{{ i18n.t('activities.insights.priority') }}</h2>
            <div class="priority-chart">
              <div class="donut" [style.background]="priorityGradient()">
                <span
                  ><small>{{ i18n.t('activities.summary.total') }}</small
                  ><strong>{{ taskCount() }}</strong></span
                >
              </div>
              <ul>
                <li>
                  <i class="high"></i>{{ i18n.t('activities.priority.high')
                  }}<strong>{{ priorityCounts().high }}</strong>
                </li>
                <li>
                  <i class="normal"></i>{{ i18n.t('activities.priority.normal')
                  }}<strong>{{ priorityCounts().normal }}</strong>
                </li>
                <li>
                  <i class="low"></i>{{ i18n.t('activities.priority.low')
                  }}<strong>{{ priorityCounts().low }}</strong>
                </li>
              </ul>
            </div>
          </section>

          <section class="insight-card overdue-card">
            <header>
              <h2>{{ i18n.t('activities.insights.overdue') }}</h2>
              <span>{{ overdueTasks().length }}</span>
            </header>
            @if (overdueTasks().length === 0) {
              <p class="empty-copy">{{ i18n.t('activities.insights.noOverdue') }}</p>
            } @else {
              <ul>
                @for (task of overdueTasks().slice(0, 4); track task.id) {
                  <li>
                    <span></span>
                    <div>
                      <strong>{{ task.title }}</strong
                      ><small>{{
                        i18n.date(task.dueAt!, { dateStyle: 'medium', timeStyle: 'short' })
                      }}</small>
                    </div>
                  </li>
                }
              </ul>
            }
          </section>
        </aside>
      </div>
    </div>

    @if (createOpen()) {
      <button
        class="drawer-scrim"
        type="button"
        (click)="closeCreate()"
        [attr.aria-label]="i18n.t('common.action.close')"
      ></button>
      <aside
        class="create-panel"
        role="dialog"
        aria-modal="true"
        cdkTrapFocus
        [cdkTrapFocusAutoCapture]="true"
        [attr.aria-labelledby]="'new-task-title'"
      >
        <header>
          <div>
            <h2 id="new-task-title">{{ i18n.t('activities.create.title') }}</h2>
            <p>{{ i18n.t('activities.create.subtitle') }}</p>
          </div>
          <button mat-button type="button" (click)="closeCreate()">
            {{ i18n.t('common.action.close') }}
          </button>
        </header>
        <form (submit)="create($event)">
          <label
            >{{ i18n.t('web.activity.type')
            }}<mat-select
              panelClass="crm-select-panel"
              class="crm-select"
              [aria-label]="i18n.t('web.activity.type')"
              [formField]="activityForm.type"
            >
              <mat-option value="task">{{ i18n.t('activities.activity.task') }}</mat-option>
              <mat-option value="call">{{ i18n.t('activities.activity.call') }}</mat-option>
              <mat-option value="meeting">{{ i18n.t('activities.activity.meeting') }}</mat-option>
              <mat-option value="note">{{ i18n.t('activities.activity.note') }}</mat-option>
            </mat-select></label
          >
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('activities.activity.title') }}</mat-label
            ><input matInput [formField]="activityForm.title"
          /></mat-form-field>
          <mat-form-field appearance="outline" class="full-width"
            ><mat-label>{{ i18n.t('activities.activity.body') }}</mat-label
            ><textarea matInput rows="4" [formField]="activityForm.body"></textarea>
          </mat-form-field>
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('activities.activity.due') }}</mat-label
            ><input matInput type="datetime-local" [formField]="activityForm.dueAt"
          /></mat-form-field>
          <label
            >{{ i18n.t('activities.activity.priority')
            }}<mat-select
              panelClass="crm-select-panel"
              class="crm-select"
              [aria-label]="i18n.t('activities.activity.priority')"
              [formField]="activityForm.priority"
            >
              <mat-option value="low">{{ i18n.t('activities.priority.low') }}</mat-option>
              <mat-option value="normal">{{ i18n.t('activities.priority.normal') }}</mat-option>
              <mat-option value="high">{{ i18n.t('activities.priority.high') }}</mat-option>
            </mat-select></label
          >
          <div class="form-actions">
            <button mat-button type="button" (click)="closeCreate()">
              {{ i18n.t('common.action.cancel') }}</button
            ><button mat-flat-button type="submit" [disabled]="store.saving()">
              {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
            </button>
          </div>
        </form>
      </aside>
    }
  `,
  styleUrl: './activities.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ActivitiesPage implements OnInit {
  readonly store = inject(ActivitiesStore);
  readonly i18n = inject(I18nService);
  readonly createOpen = signal(false);
  readonly selectedTaskId = signal<string | null>(null);
  readonly query = signal('');
  readonly typeFilter = signal<ActivityTypeFilter>('task');
  readonly statusFilter = signal<ActivityStatusFilter>('all');
  readonly priorityFilter = signal<ActivityPriorityFilter>('all');
  readonly dueFilter = signal<'all' | 'today' | 'overdue' | 'none'>('all');
  readonly typeFilters: readonly { value: ActivityTypeFilter; label: AppMessageKey }[] = [
    { value: 'task', label: 'activities.tabs.tasks' },
    { value: 'all', label: 'activities.tabs.all' },
    { value: 'call', label: 'activities.tabs.calls' },
    { value: 'meeting', label: 'activities.tabs.meetings' },
    { value: 'note', label: 'activities.tabs.notes' },
  ];
  readonly tasks = computed(() => this.store.activities().filter((item) => item.type === 'task'));
  readonly taskCount = computed(() => this.tasks().length);
  readonly openTaskCount = computed(
    () => this.tasks().filter((item) => item.status === 'open').length,
  );
  readonly completedTaskCount = computed(
    () => this.tasks().filter((item) => item.status === 'completed').length,
  );
  readonly highPriorityTaskCount = computed(
    () => this.tasks().filter((item) => item.priority === 'high').length,
  );
  readonly overdueTasks = computed(() =>
    this.tasks()
      .filter((item) => this.isOverdue(item))
      .sort((a, b) => (a.dueAt ?? '').localeCompare(b.dueAt ?? '')),
  );
  readonly dueTodayCount = computed(
    () => this.tasks().filter((item) => this.isDueToday(item)).length,
  );
  readonly priorityCounts = computed(() => ({
    high: this.tasks().filter((item) => item.priority === 'high').length,
    normal: this.tasks().filter((item) => (item.priority ?? 'normal') === 'normal').length,
    low: this.tasks().filter((item) => item.priority === 'low').length,
  }));
  readonly priorityGradient = computed(() => {
    const total = Math.max(this.taskCount(), 1);
    const high = (this.priorityCounts().high / total) * 100;
    const normal = high + (this.priorityCounts().normal / total) * 100;
    return `conic-gradient(#ff5964 0 ${high}%, #ffb43f ${high}% ${normal}%, #3ecf8e ${normal}% 100%)`;
  });
  readonly filteredActivities = computed(() => {
    const query = this.query().trim().toLocaleLowerCase(this.i18n.locale());
    return this.store.activities().filter((item) => {
      if (this.typeFilter() !== 'all' && item.type !== this.typeFilter()) return false;
      if (this.statusFilter() !== 'all' && item.status !== this.statusFilter()) return false;
      if (this.priorityFilter() !== 'all' && (item.priority ?? 'normal') !== this.priorityFilter())
        return false;
      if (this.dueFilter() === 'today' && !this.isDueToday(item)) return false;
      if (this.dueFilter() === 'overdue' && !this.isOverdue(item)) return false;
      if (this.dueFilter() === 'none' && item.dueAt) return false;
      return (
        !query ||
        `${item.title} ${item.body ?? ''}`.toLocaleLowerCase(this.i18n.locale()).includes(query)
      );
    });
  });
  readonly monthLabel = computed(() =>
    this.i18n.date(new Date(), { month: 'long', year: 'numeric' }),
  );
  readonly weekdayLabels = computed(() => {
    const base = new Date(Date.UTC(2024, 0, 1));
    return Array.from({ length: 7 }, (_, index) =>
      this.i18n.date(new Date(base.getTime() + index * 86_400_000), { weekday: 'short' }),
    );
  });
  readonly calendarDays = computed(() => {
    const now = new Date();
    const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
    const mondayOffset = (monthStart.getDay() + 6) % 7;
    const first = new Date(now.getFullYear(), now.getMonth(), 1 - mondayOffset);
    return Array.from({ length: 42 }, (_, index) => {
      const date = new Date(first.getFullYear(), first.getMonth(), first.getDate() + index);
      return {
        key: date.toISOString(),
        day: date.getDate(),
        currentMonth: date.getMonth() === now.getMonth(),
        today: this.sameDay(date, now),
        hasTask: this.tasks().some(
          (task) => task.dueAt && this.sameDay(new Date(task.dueAt), date),
        ),
      };
    });
  });
  readonly model = signal<{
    type: CreateActivity['type'];
    title: string;
    body: string;
    dueAt: string;
    priority: CreateActivity['priority'];
  }>({ type: 'task', title: '', body: '', dueAt: '', priority: 'normal' });
  readonly activityForm = form(this.model, (schema) => required(schema.title));

  ngOnInit(): void {
    void this.store.load();
  }
  typeKey(activity: Activity): AppMessageKey {
    return `activities.activity.${activity.type}` as AppMessageKey;
  }
  statusKey(activity: Activity): AppMessageKey {
    return activity.status === 'cancelled'
      ? 'activities.status.cancelled'
      : activity.status === 'completed'
        ? 'common.status.completed'
        : 'common.status.open';
  }
  priorityKey(activity: Activity): AppMessageKey {
    return `activities.priority.${activity.priority ?? 'normal'}` as AppMessageKey;
  }
  isOverdue(activity: Activity): boolean {
    return (
      activity.status === 'open' &&
      Boolean(activity.dueAt) &&
      new Date(activity.dueAt!).getTime() < Date.now()
    );
  }
  isDueToday(activity: Activity): boolean {
    return Boolean(activity.dueAt) && this.sameDay(new Date(activity.dueAt!), new Date());
  }
  private sameDay(left: Date, right: Date): boolean {
    return (
      left.getFullYear() === right.getFullYear() &&
      left.getMonth() === right.getMonth() &&
      left.getDate() === right.getDate()
    );
  }
  resetFilters(): void {
    this.statusFilter.set('all');
    this.priorityFilter.set('all');
    this.dueFilter.set('all');
  }
  openCreate(): void {
    this.model.set({ type: 'task', title: '', body: '', dueAt: '', priority: 'normal' });
    this.createOpen.set(true);
  }
  closeCreate(): void {
    this.createOpen.set(false);
  }
  toggleAssignments(activity: Activity): void {
    this.selectedTaskId.update((current) => (current === activity.id ? null : activity.id));
  }
  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.activityForm().invalid()) {
      this.activityForm().markAsTouched();
      return;
    }
    const value = this.model();
    await this.store.create({
      type: value.type,
      title: value.title.trim(),
      body: value.body.trim() || null,
      priority: value.priority,
      dueAt: value.dueAt ? new Date(value.dueAt).toISOString() : null,
    });
    this.closeCreate();
  }
}
