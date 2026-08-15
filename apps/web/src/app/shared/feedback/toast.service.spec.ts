import { TestBed } from '@angular/core/testing';

import { ApiError } from '../../core/api/api-error';
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
    expect(service.items().find((item) => item.id === ids[2])?.exiting).toBe(true);
    vi.advanceTimersByTime(140);
    expect(service.items().map((item) => item.messageKey)).toEqual(['test.3', 'test.1']);
  });

  it('expires transient items after the configured duration', () => {
    vi.useFakeTimers();
    const service = TestBed.inject(ToastService);
    service.show({ messageKey: 'test.expiring', messageParams: {} }, 1000);

    vi.advanceTimersByTime(1000);
    expect(service.items()[0]?.exiting).toBe(true);
    vi.advanceTimersByTime(140);
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
    vi.advanceTimersByTime(140);
    expect(service.items()).toEqual([]);
  });

  it('keeps an API problem code for locale-aware error rendering', () => {
    const service = TestBed.inject(ToastService);

    service.showError(
      new ApiError(422, {
        type: 'https://veltrix.local/problems/validation',
        title: 'Validation failed',
        status: 422,
        code: 'validation.failed',
        requestId: 'request-1',
      }),
      0,
    );

    expect(service.items()[0]).toMatchObject({
      problemCode: 'validation.failed',
      requestId: 'request-1',
      tone: 'error',
    });
  });
});
