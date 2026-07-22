import { Injectable, signal } from '@angular/core';

export interface AppToast {
  readonly id: string;
  readonly messageKey: string;
  readonly messageParams: Readonly<Record<string, unknown>>;
  readonly href?: string;
}

@Injectable({ providedIn: 'root' })
export class ToastService {
  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>();
  readonly items = signal<readonly AppToast[]>([]);

  show(input: Omit<AppToast, 'id'>, durationMs = 6500): string {
    const id = crypto.randomUUID();
    this.items.update((items) => [{ id, ...input }, ...items].slice(0, 3));
    const timer = setTimeout(() => this.dismiss(id), durationMs);
    this.timers.set(id, timer);
    return id;
  }

  dismiss(id: string): void {
    const timer = this.timers.get(id);
    if (timer) clearTimeout(timer);
    this.timers.delete(id);
    this.items.update((items) => items.filter((item) => item.id !== id));
  }
}
