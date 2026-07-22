import { computed, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  CalendarActivity,
  CalendarActivityInput,
  Department,
  WorkspaceMember,
} from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { Permissions } from '../../core/auth/permissions';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';

export type CalendarView = 'day' | 'week' | 'month';

export interface CalendarDay {
  readonly date: Date;
  readonly key: string;
  readonly currentMonth: boolean;
  readonly items: readonly CalendarActivity[];
}

@Injectable()
export class CalendarStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly auth = inject(AuthStore);
  private readonly permissions = inject(Permissions);
  private readonly toasts = inject(ToastService);
  private requestSequence = 0;

  readonly view = signal<CalendarView>('month');
  readonly anchor = signal(startOfDay(new Date()));
  readonly activities = signal<readonly CalendarActivity[]>([]);
  readonly members = signal<readonly WorkspaceMember[]>([]);
  readonly departments = signal<readonly Department[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  readonly range = computed(() => calendarRange(this.anchor(), this.view()));
  readonly days = computed<readonly CalendarDay[]>(() => {
    const range = this.range();
    const anchorMonth = this.anchor().getMonth();
    const result: CalendarDay[] = [];
    for (let date = new Date(range.start); date < range.end; date = addDays(date, 1)) {
      const key = dayKey(date);
      result.push({
        date,
        key,
        currentMonth: this.view() !== 'month' || date.getMonth() === anchorMonth,
        items: this.activities().filter((item) => dayKey(new Date(item.occurredAt)) === key),
      });
    }
    return result;
  });

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const request = ++this.requestSequence;
    const range = this.range();
    this.loading.set(true);
    this.error.set(null);
    try {
      const items = await this.api.listCalendar(workspaceId, range.start, range.end);
      if (request === this.requestSequence) this.activities.set(items);
    } catch (error) {
      if (request === this.requestSequence) this.error.set(error);
    } finally {
      if (request === this.requestSequence) this.loading.set(false);
    }
  }

  async loadAudienceOptions(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    try {
      const [departments, members] = await Promise.all([
        this.api.listDepartments(workspaceId),
        this.permissions.allows('members.read')
          ? this.api.listMembers(workspaceId)
          : Promise.resolve([]),
      ]);
      this.departments.set(departments);
      this.members.set(members.filter((member) => member.status === 'active'));
    } catch (error) {
      this.error.set(error);
    }
  }

  currentUser(): { readonly id: string; readonly displayName: string } | null {
    const user = this.auth.user();
    return user ? { id: user.id, displayName: user.displayName } : null;
  }

  async setView(view: CalendarView): Promise<void> {
    this.view.set(view);
    await this.load();
  }

  async move(direction: -1 | 1): Promise<void> {
    const date = new Date(this.anchor());
    if (this.view() === 'day') date.setDate(date.getDate() + direction);
    else if (this.view() === 'week') date.setDate(date.getDate() + direction * 7);
    else date.setMonth(date.getMonth() + direction);
    this.anchor.set(startOfDay(date));
    await this.load();
  }

  async today(): Promise<void> {
    this.anchor.set(startOfDay(new Date()));
    await this.load();
  }

  async create(body: CalendarActivityInput): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.api.createCalendarActivity(workspaceId, body);
      await this.load();
      this.toasts.show({ messageKey: 'calendar.created', messageParams: {} });
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  icsUrl(): string | null {
    const workspaceId = this.workspace.id();
    const range = this.range();
    return workspaceId ? this.api.calendarIcsUrl(workspaceId, range.start, range.end) : null;
  }
}

export function calendarRange(anchor: Date, view: CalendarView): { start: Date; end: Date } {
  const day = startOfDay(anchor);
  if (view === 'day') return { start: day, end: addDays(day, 1) };
  if (view === 'week') {
    const mondayOffset = (day.getDay() + 6) % 7;
    const start = addDays(day, -mondayOffset);
    return { start, end: addDays(start, 7) };
  }
  const first = new Date(day.getFullYear(), day.getMonth(), 1);
  const mondayOffset = (first.getDay() + 6) % 7;
  const start = addDays(first, -mondayOffset);
  return { start, end: addDays(start, 42) };
}

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}

function dayKey(date: Date): string {
  return [date.getFullYear(), date.getMonth() + 1, date.getDate()]
    .map((value, index) => (index === 0 ? String(value) : String(value).padStart(2, '0')))
    .join('-');
}
