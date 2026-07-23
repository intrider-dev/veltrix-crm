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
}

@Injectable({ providedIn: 'root' })
export class ToastService {
  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>();
  readonly items = signal<readonly AppToast[]>([]);

  show(input: Omit<AppToast, 'id'>, durationMs = 6500): string {
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
    this.items.update((items) => items.filter((item) => item.id !== id));
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
