import { DOCUMENT } from '@angular/common';
import { DestroyRef, Injectable, inject } from '@angular/core';
import { SwUpdate } from '@angular/service-worker';
import { Router } from '@angular/router';
import { filter } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

import { ToastService } from '../../shared/feedback/toast.service';

const AUTH_ROUTE = /^\/(?:login|register|password-reset)(?:[/?#]|$)/;

@Injectable({ providedIn: 'root' })
export class AppUpdateService {
  private readonly updates = inject(SwUpdate);
  private readonly router = inject(Router);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);
  private readonly toasts = inject(ToastService);
  private started = false;

  start(): void {
    if (this.started || !this.updates.isEnabled) return;
    this.started = true;

    this.updates.versionUpdates
      .pipe(
        filter((event) => event.type === 'VERSION_READY'),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(() => this.handleReadyVersion());

    this.updates.unrecoverable.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      if (AUTH_ROUTE.test(this.router.url)) {
        void this.activateAndReload();
        return;
      }
      this.showReloadToast('pwa.unrecoverable');
    });

    void this.updates.checkForUpdate().catch(() => {
      // Offline startup is expected for the app shell; the next navigation
      // or browser service-worker check will retry without blocking the UI.
    });
  }

  private handleReadyVersion(): void {
    if (AUTH_ROUTE.test(this.router.url)) {
      void this.activateAndReload();
      return;
    }
    this.showReloadToast('pwa.updateAvailable');
  }

  private showReloadToast(messageKey: string): void {
    this.toasts.show(
      {
        messageKey,
        messageParams: {},
        actionLabelKey: 'pwa.reload',
        action: () => void this.activateAndReload(),
      },
      0,
    );
  }

  private async activateAndReload(): Promise<void> {
    try {
      await this.updates.activateUpdate();
    } finally {
      this.document.defaultView?.location.reload();
    }
  }
}
