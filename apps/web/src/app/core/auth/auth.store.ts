import { computed, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../api/api-client.service';
import { ApiError } from '../api/api-error';
import type { MFAChallenge, SessionView } from '../api/api.types';
import { I18nService } from '../i18n/i18n.service';

@Injectable({ providedIn: 'root' })
export class AuthStore {
  private readonly api = inject(ApiClient);
  private readonly i18n = inject(I18nService);
  private readonly sessionState = signal<SessionView | null>(null);
  private readonly pendingState = signal(false);
  private sessionRequest: Promise<boolean> | null = null;

  readonly session = this.sessionState.asReadonly();
  readonly user = computed(() => this.sessionState()?.user ?? null);
  readonly authenticated = computed(() => this.sessionState() !== null);
  readonly pending = this.pendingState.asReadonly();

  async ensureSession(): Promise<boolean> {
    if (this.sessionState()) return true;
    if (this.sessionRequest) return this.sessionRequest;
    this.sessionRequest = this.loadSession();
    try {
      return await this.sessionRequest;
    } finally {
      this.sessionRequest = null;
    }
  }

  async login(email: string, password: string): Promise<MFAChallenge | null> {
    this.pendingState.set(true);
    try {
      const result = await this.api.login(email, password);
      if ('mfaRequired' in result) return result;
      await this.acceptSession(result);
      return null;
    } finally {
      this.pendingState.set(false);
    }
  }

  async verifyMFA(challengeToken: string, code: string): Promise<void> {
    this.pendingState.set(true);
    try {
      await this.acceptSession(await this.api.verifyMFALogin({ challengeToken, code }));
    } finally {
      this.pendingState.set(false);
    }
  }

  async logout(): Promise<void> {
    try {
      await this.api.logout();
    } finally {
      this.sessionState.set(null);
    }
  }

  async updateLocale(locale: 'en' | 'ru'): Promise<void> {
    const previous = this.i18n.locale();
    await this.i18n.setLocale(locale);
    try {
      const user = await this.api.updateLocale(locale);
      const session = this.sessionState();
      if (session) this.sessionState.set({ ...session, user });
    } catch (error) {
      await this.i18n.setLocale(previous);
      throw error;
    }
  }

  async refreshSession(): Promise<boolean> {
    try {
      await this.acceptSession(await this.api.me());
      return true;
    } catch {
      this.sessionState.set(null);
      return false;
    }
  }

  private async loadSession(): Promise<boolean> {
    try {
      const probe = await this.api.probeSession();
      if (!probe.authenticated || !probe.session) {
        this.sessionState.set(null);
        return false;
      }
      await this.acceptSession(probe.session);
      return true;
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        this.sessionState.set(null);
        return false;
      }
      this.sessionState.set(null);
      return false;
    }
  }

  private async acceptSession(session: SessionView): Promise<void> {
    this.sessionState.set(session);
    await this.i18n.applyPreference(
      session.user.preferredLocale,
      session.workspaces[0]?.defaultLocale,
    );
  }
}
