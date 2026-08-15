import {
  calendarActivityIntersectsDay,
  calendarEventLayout,
  calendarRange,
} from './calendar.store';

describe('calendarRange', () => {
  it('uses a Monday-first seven-day range for the week view', () => {
    const range = calendarRange(new Date(2026, 6, 21), 'week');
    expect(range.start.getDay()).toBe(1);
    expect((range.end.getTime() - range.start.getTime()) / 86_400_000).toBe(7);
  });

  it('returns a bounded six-week month grid', () => {
    const range = calendarRange(new Date(2026, 6, 21), 'month');
    expect((range.end.getTime() - range.start.getTime()) / 86_400_000).toBe(42);
  });

  it('scales timed event height from its duration', () => {
    const day = new Date(2026, 7, 15);
    const oneHour = calendarEventLayout(
      { occurredAt: '2026-08-15T09:00:00', endsAt: '2026-08-15T10:00:00' },
      day,
    );
    const twoHours = calendarEventLayout(
      { occurredAt: '2026-08-15T09:00:00', endsAt: '2026-08-15T11:00:00' },
      day,
    );

    expect(oneHour.top).toBe(9 * 48);
    expect(oneHour.height).toBe(48);
    expect(twoHours.height).toBe(96);
  });

  it('clips an overnight event to each visible day', () => {
    const event = { occurredAt: '2026-08-15T23:00:00', endsAt: '2026-08-16T01:00:00' };
    const nextDay = new Date(2026, 7, 16);

    expect(calendarActivityIntersectsDay(event, nextDay)).toBe(true);
    expect(calendarEventLayout(event, nextDay)).toEqual({ top: 0, height: 48 });
  });

  it('positions events using the workspace timezone wall clock', () => {
    const event = {
      occurredAt: '2026-08-13T01:00:00Z',
      endsAt: '2026-08-13T02:30:00Z',
    };
    const workspaceDay = new Date(2026, 7, 12);

    expect(calendarActivityIntersectsDay(event, workspaceDay, 'America/Los_Angeles')).toBe(true);
    expect(calendarEventLayout(event, workspaceDay, 48, 'America/Los_Angeles')).toEqual({
      top: 18 * 48,
      height: 72,
    });
  });
});
