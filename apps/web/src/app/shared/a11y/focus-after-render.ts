import type { Injector } from '@angular/core';
import { afterNextRender } from '@angular/core';

interface FocusTarget {
  focus(): void;
}

/**
 * Schedules focus after Angular has rendered the state change that exposes the
 * target. Passing the component injector keeps this safe when called from an
 * event handler or an RxJS callback, both of which run outside an injection
 * context.
 */
export function focusAfterNextRender(
  injector: Injector,
  target: () => FocusTarget | null | undefined,
): void {
  afterNextRender(() => target()?.focus(), { injector });
}
