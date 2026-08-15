import { Injectable, signal } from '@angular/core';

import { ApiError } from '../../core/api/api-error';
import type { AppMessageKey } from '../../core/i18n/app-message-key';

export interface AppToast {
  readonly id: string;
  readonly messageKey?: string;
  readonly messageParams: Readonly<Record<string, unknown>>;
  readonly problemCode?: string;
  readonly requestId?: string;
  readonly tone?: 'info' | 'success' | 'error';
  readonly href?: string;
  readonly actionLabelKey?: AppMessageKey;
  readonly action?: () => void;
  readonly exiting?: boolean;
}

@Injectable({ providedIn: 'root' })
export class ToastService {
  private static readonly exitDurationMs = 140;
  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly exitTimers = new Map<string, ReturnType<typeof setTimeout>>();
  readonly items = signal<readonly AppToast[]>([]);

  show(input: Omit<AppToast, 'id' | 'exiting'>, durationMs = 6500): string {
    const id = crypto.randomUUID();
    const next = [{ id, ...input }, ...this.items()];
    for (const dropped of next.slice(3)) this.clearTimer(dropped.id);
    this.items.set(next.slice(0, 3));
    if (durationMs > 0) {
      const timer = setTimeout(() => this.dismiss(id), durationMs);
      this.timers.set(id, timer);
    }
    return id;
  }

  showError(error: unknown, durationMs = 8000): string {
    const apiError = error instanceof ApiError ? error : null;
    return this.show(
      {
        messageParams: {},
        problemCode: apiError?.problem?.code ?? 'network',
        requestId: apiError?.problem?.requestId,
        tone: 'error',
      },
      durationMs,
    );
  }

  dismiss(id: string): void {
    this.clearTimer(id);
    const item = this.items().find((candidate) => candidate.id === id);
    if (!item || item.exiting) return;
    this.items.update((items) =>
      items.map((candidate) => (candidate.id === id ? { ...candidate, exiting: true } : candidate)),
    );
    const timer = setTimeout(() => {
      this.items.update((items) => items.filter((candidate) => candidate.id !== id));
      this.exitTimers.delete(id);
    }, ToastService.exitDurationMs);
    this.exitTimers.set(id, timer);
  }

  invokeAction(toast: AppToast): void {
    toast.action?.();
    this.dismiss(toast.id);
  }

  private clearTimer(id: string): void {
    const timer = this.timers.get(id);
    if (timer) clearTimeout(timer);
    this.timers.delete(id);
  }
}
