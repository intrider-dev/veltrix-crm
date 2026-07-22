import { TestBed } from '@angular/core/testing';

import { ToastService } from './toast.service';

describe('ToastService', () => {
  afterEach(() => vi.useRealTimers());

  it('keeps a bounded newest-first queue and dismisses an item', () => {
    vi.useFakeTimers();
    const service = TestBed.inject(ToastService);
    const ids = Array.from({ length: 4 }, (_, index) =>
      service.show({ messageKey: `test.${index}`, messageParams: {} }),
    );

    expect(service.items().map((item) => item.messageKey)).toEqual(['test.3', 'test.2', 'test.1']);
    service.dismiss(ids[2]);
    expect(service.items().map((item) => item.messageKey)).toEqual(['test.3', 'test.1']);
  });

  it('expires transient items after the configured duration', () => {
    vi.useFakeTimers();
    const service = TestBed.inject(ToastService);
    service.show({ messageKey: 'test.expiring', messageParams: {} }, 1000);

    vi.advanceTimersByTime(1000);
    expect(service.items()).toEqual([]);
  });

  it('keeps persistent actionable items until the action is invoked', () => {
    vi.useFakeTimers();
    const service = TestBed.inject(ToastService);
    const action = vi.fn();
    service.show(
      {
        messageKey: 'test.update',
        messageParams: {},
        actionLabelKey: 'pwa.reload',
        action,
      },
      0,
    );

    vi.runAllTimers();
    const toast = service.items()[0];
    if (!toast) throw new Error('Expected the persistent toast to be queued');
    service.invokeAction(toast);

    expect(action).toHaveBeenCalledOnce();
    expect(service.items()).toEqual([]);
  });
});
