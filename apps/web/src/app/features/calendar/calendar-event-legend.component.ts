import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import type { CalendarActivity } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';

type ActivityType = CalendarActivity['type'];

@Component({
  selector: 'app-calendar-event-legend',
  template: `
    <section>
      <header>
        <h2>{{ i18n.t('calendar.typeFilter') }}</h2>
      </header>
      <div class="event-type-legend">
        @for (type of activityTypes; track type) {
          <span [class]="'type-' + type">
            <i></i>
            {{ i18n.t(typeKey(type)) }}
          </span>
        }
      </div>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }
    section {
      overflow: hidden;
      border: 1px solid color-mix(in srgb, var(--border) 78%, transparent);
      border-radius: var(--radius-panel, 0.875rem);
      background: color-mix(in srgb, var(--surface-raised) 93%, #07111f);
    }
    header {
      display: flex;
      min-height: 3.5rem;
      align-items: center;
      padding: 0.875rem 1rem;
      border-bottom: 1px solid color-mix(in srgb, var(--border) 78%, transparent);
    }
    h2 {
      margin: 0;
      font-size: 0.9375rem;
    }
    .event-type-legend {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.5rem;
      padding: 0.875rem 1rem;
    }
    span {
      --legend-color: #38bdf8;
      display: flex;
      min-width: 0;
      align-items: center;
      gap: 0.5rem;
      color: var(--text);
      font-size: 0.6875rem;
    }
    .type-call {
      --legend-color: #34d399;
    }
    .type-task {
      --legend-color: #a78bfa;
    }
    .type-note {
      --legend-color: #fbbf24;
    }
    i {
      width: 0.5rem;
      height: 0.5rem;
      flex: none;
      border-radius: 50%;
      background: var(--legend-color);
      box-shadow: 0 0 0 0.2rem color-mix(in srgb, var(--legend-color) 14%, transparent);
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarEventLegendComponent {
  readonly i18n = inject(I18nService);
  readonly activityTypes = ['meeting', 'call', 'task', 'note'] as const;

  typeKey(
    type: ActivityType,
  ):
    | 'activities.activity.task'
    | 'activities.activity.call'
    | 'activities.activity.meeting'
    | 'activities.activity.note' {
    return `activities.activity.${type}`;
  }
}
