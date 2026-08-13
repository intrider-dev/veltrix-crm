import { Injectable, inject, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { MFASetup, MFAStatus } from '../../core/api/api.types';

@Injectable()
export class SecurityStore {
  readonly status = signal<MFAStatus | null>(null);
  readonly setup = signal<MFASetup | null>(null);
  readonly recoveryCodes = signal<readonly string[] | null>(null);
  readonly sessionsRevoked = signal(false);
  readonly loading = signal(false);
  readonly pending = signal(false);
  readonly loadError = signal<unknown>(null);

  private readonly api = inject(ApiClient);

  async load(): Promise<void> {
    this.loading.set(true);
    this.loadError.set(null);
    try {
      this.status.set(await this.api.mfaStatus());
    } catch (error) {
      this.loadError.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async changePassword(currentPassword: string, newPassword: string): Promise<void> {
    await this.run(() => this.api.changePassword({ currentPassword, newPassword }));
  }

  async beginMFASetup(currentPassword: string): Promise<void> {
    const setup = await this.run(() => this.api.beginMFASetup({ currentPassword }));
    this.setup.set(setup);
  }

  async confirmMFASetup(code: string): Promise<void> {
    const result = await this.run(() => this.api.confirmMFASetup({ code }));
    this.status.set({ enabled: true });
    this.setup.set(null);
    this.recoveryCodes.set(result.recoveryCodes);
    this.sessionsRevoked.set(result.sessionsRevoked);
  }

  async regenerateRecoveryCodes(currentPassword: string, code: string): Promise<void> {
    const result = await this.run(() =>
      this.api.regenerateRecoveryCodes({ currentPassword, code }),
    );
    this.recoveryCodes.set(result.recoveryCodes);
    this.sessionsRevoked.set(result.sessionsRevoked);
  }

  async disableMFA(currentPassword: string, code: string): Promise<void> {
    await this.run(() => this.api.disableMFA({ currentPassword, code }));
    this.status.set({ enabled: false });
    this.setup.set(null);
    this.recoveryCodes.set(null);
    this.sessionsRevoked.set(true);
  }

  async logoutAllSessions(): Promise<void> {
    await this.run(() => this.api.logoutAllSessions());
    this.sessionsRevoked.set(true);
  }

  clearSetup(): void {
    this.setup.set(null);
  }

  private async run<T>(operation: () => Promise<T>): Promise<T> {
    this.pending.set(true);
    try {
      return await operation();
    } finally {
      this.pending.set(false);
    }
  }
}
