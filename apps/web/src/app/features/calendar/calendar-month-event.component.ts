import { ChangeDetectionStrategy, Component, inject, input } from '@angular/core';

import type { CalendarActivity } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';

type ActivityType = CalendarActivity['type'];

@Component({
  selector: 'app-calendar-month-event',
  template: `
    <article
      [class]="'calendar-event type-' + item().type"
      [class.is-completed]="item().status === 'completed'"
      [attr.aria-label]="eventAriaLabel()"
    >
      <span class="event-type">{{ i18n.t(typeKey(item().type)) }}</span>
      <time [attr.datetime]="item().occurredAt">{{ eventTimeRange() }}</time>
      <strong>{{ item().title }}</strong>
    </article>
  `,
  styleUrl: './calendar-month-event.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarMonthEventComponent {
  readonly item = input.required<CalendarActivity>();
  readonly i18n = inject(I18nService);

  eventTimeRange(): string {
    const item = this.item();
    const start = this.eventTime(item.occurredAt);
    if (!item.endsAt || Date.parse(item.endsAt) <= Date.parse(item.occurredAt)) return start;
    return `${start}–${this.eventTime(item.endsAt)}`;
  }

  eventAriaLabel(): string {
    const item = this.item();
    return [this.i18n.t(this.typeKey(item.type)), item.title, this.eventTimeRange()].join(', ');
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

  private eventTime(value: string): string {
    return this.i18n.date(value, { hour: '2-digit', minute: '2-digit', hour12: false });
  }
}
