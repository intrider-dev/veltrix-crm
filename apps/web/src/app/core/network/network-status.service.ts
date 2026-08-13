import { DOCUMENT } from '@angular/common';
import { DestroyRef, Injectable, inject, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class NetworkStatusService {
  private readonly window = inject(DOCUMENT).defaultView;
  private readonly onlineState = signal(this.window?.navigator.onLine ?? true);

  readonly online = this.onlineState.asReadonly();

  constructor() {
    const setOnline = () => this.onlineState.set(true);
    const setOffline = () => this.onlineState.set(false);
    this.window?.addEventListener('online', setOnline);
    this.window?.addEventListener('offline', setOffline);
    inject(DestroyRef).onDestroy(() => {
      this.window?.removeEventListener('online', setOnline);
      this.window?.removeEventListener('offline', setOffline);
    });
  }
}
