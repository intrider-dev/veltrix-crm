import { calendarRange } from './calendar.store';

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
});
