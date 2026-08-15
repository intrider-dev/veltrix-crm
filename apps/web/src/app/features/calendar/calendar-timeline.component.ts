import { ChangeDetectionStrategy, Component, inject, input, output } from '@angular/core';

import type { CalendarActivity } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { calendarEventLayout, type CalendarDay, type CalendarView } from './calendar.store';

type ActivityType = CalendarActivity['type'];
type VisibilityScope = CalendarActivity['visibilityScope'];

@Component({
  selector: 'app-calendar-timeline',
  template: `
    <div
      class="timeline-shell"
      [class.week-view]="view() === 'week'"
      [class.day-view]="view() === 'day'"
      role="group"
      [attr.aria-label]="i18n.t('common.nav.calendar')"
    >
      <div class="timeline-header">
        <span class="time-gutter-spacer" aria-hidden="true"></span>
        <div class="timeline-day-headings">
          @for (day of days(); track day.key) {
            <button type="button" (click)="createRequested.emit(day.date)">
              <span>{{ dayWeekday(day.date) }}</span>
              <strong [class.today]="isToday(day.date)">{{ day.date.getDate() }}</strong>
            </button>
          }
        </div>
      </div>
      <div class="timeline-body">
        <div class="time-gutter" aria-hidden="true">
          @for (hour of timelineHours; track hour) {
            <time [style.top.px]="hour * hourHeight">{{ hourLabel(hour) }}</time>
          }
        </div>
        <div class="timeline-days">
          @for (day of days(); track day.key) {
            <section
              class="timeline-day"
              [class.today]="isToday(day.date)"
              [attr.aria-label]="dayAriaLabel(day.date, day.items.length)"
            >
              @for (item of day.items; track item.id) {
                <article
                  [class]="'timeline-event type-' + item.type"
                  [class.is-completed]="item.status === 'completed'"
                  [style.top.px]="eventTop(item, day.date)"
                  [style.height.px]="eventHeight(item, day.date)"
                  [attr.aria-label]="eventAriaLabel(item)"
                >
                  <div class="event-heading">
                    <span class="event-type">{{ i18n.t(typeKey(item.type)) }}</span>
                    <time [attr.datetime]="item.occurredAt">{{ eventTimeRange(item) }}</time>
                  </div>
                  <div class="event-details">
                    <strong>{{ item.title }}</strong>
                    <span class="event-context">
                      <span class="event-meta">{{ i18n.t(scopeKey(item.visibilityScope)) }}</span>
                      @if (item.location) {
                        <small>{{ item.location }}</small>
                      }
                    </span>
                  </div>
                </article>
              }
            </section>
          }
        </div>
      </div>
    </div>
  `,
  styleUrl: './calendar-timeline.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarTimelineComponent {
  readonly days = input.required<readonly CalendarDay[]>();
  readonly view = input.required<Exclude<CalendarView, 'month'>>();
  readonly createRequested = output<Date>();
  readonly i18n = inject(I18nService);
  readonly timelineHours = Array.from({ length: 24 }, (_, hour) => hour);
  readonly hourHeight = 48;

  eventTop(item: CalendarActivity, day: Date): number {
    return calendarEventLayout(item, day, this.hourHeight, this.i18n.currentTimeZone()).top;
  }

  eventHeight(item: CalendarActivity, day: Date): number {
    return calendarEventLayout(item, day, this.hourHeight, this.i18n.currentTimeZone()).height;
  }

  eventTimeRange(item: CalendarActivity): string {
    const start = this.eventTime(item.occurredAt);
    if (!item.endsAt || Date.parse(item.endsAt) <= Date.parse(item.occurredAt)) return start;
    return `${start}–${this.eventTime(item.endsAt)}`;
  }

  eventAriaLabel(item: CalendarActivity): string {
    const parts = [
      this.i18n.t(this.typeKey(item.type)),
      item.title,
      this.eventTimeRange(item),
      this.i18n.t(this.scopeKey(item.visibilityScope)),
    ];
    if (item.location) parts.push(item.location);
    return parts.join(', ');
  }

  hourLabel(hour: number): string {
    return `${String(hour).padStart(2, '0')}:00`;
  }

  dayWeekday(date: Date): string {
    return this.calendarDate(date, { weekday: 'short' });
  }

  isToday(date: Date): boolean {
    const today = new Date();
    return (
      date.getFullYear() === today.getFullYear() &&
      date.getMonth() === today.getMonth() &&
      date.getDate() === today.getDate()
    );
  }

  dayAriaLabel(date: Date, count: number): string {
    return `${this.calendarDate(date, { dateStyle: 'full' })}, ${count} ${this.i18n.t('calendar.eventsCount')}`;
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

  private eventTime(value: string): string {
    return this.i18n.date(value, { hour: '2-digit', minute: '2-digit', hour12: false });
  }

  private calendarDate(date: Date, options: Intl.DateTimeFormatOptions): string {
    const value = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), 12));
    return this.i18n.date(value, { ...options, timeZone: 'UTC' });
  }
}
