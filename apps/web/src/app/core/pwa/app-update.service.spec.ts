import { TestBed } from '@angular/core/testing';
import { SwUpdate, type UnrecoverableStateEvent, type VersionEvent } from '@angular/service-worker';
import { Router } from '@angular/router';
import { Subject } from 'rxjs';

import { ToastService } from '../../shared/feedback/toast.service';
import { AppUpdateService } from './app-update.service';

describe('AppUpdateService', () => {
  const versionUpdates = new Subject<VersionEvent>();
  const unrecoverable = new Subject<UnrecoverableStateEvent>();
  const updates = {
    isEnabled: true,
    versionUpdates,
    unrecoverable,
    checkForUpdate: vi.fn().mockResolvedValue(false),
    activateUpdate: vi.fn().mockResolvedValue(true),
  };
  const toasts = { show: vi.fn<ToastService['show']>() };

  beforeEach(() => {
    vi.clearAllMocks();
    TestBed.configureTestingModule({
      providers: [
        AppUpdateService,
        { provide: SwUpdate, useValue: updates },
        { provide: Router, useValue: { url: '/dashboard' } },
        { provide: ToastService, useValue: toasts },
      ],
    });
  });

  it('checks for an update and offers a persistent localized reload action', () => {
    TestBed.inject(AppUpdateService).start();
    versionUpdates.next({
      type: 'VERSION_READY',
      currentVersion: { hash: 'current' },
      latestVersion: { hash: 'latest' },
    });

    expect(updates.checkForUpdate).toHaveBeenCalledOnce();
    expect(toasts.show).toHaveBeenCalledOnce();
    const [toast, duration] = toasts.show.mock.calls[0];
    expect(toast.messageKey).toBe('pwa.updateAvailable');
    expect(toast.actionLabelKey).toBe('pwa.reload');
    expect(typeof toast.action).toBe('function');
    expect(duration).toBe(0);
  });

  it('does not subscribe or check when service workers are disabled', () => {
    updates.isEnabled = false;
    TestBed.inject(AppUpdateService).start();

    expect(updates.checkForUpdate).not.toHaveBeenCalled();
    updates.isEnabled = true;
  });
});
