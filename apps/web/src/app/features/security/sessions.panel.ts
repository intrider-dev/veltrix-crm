import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { Router } from '@angular/router';

import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { identityErrorMessage } from '../auth/identity-error';
import { SecurityStore } from './security.store';

@Component({
  selector: 'app-sessions-panel',
  imports: [MatButtonModule],
  template: `
    <section class="panel security-panel" aria-labelledby="sessions-title">
      <header class="panel-heading">
        <div>
          <h2 id="sessions-title">{{ i18n.t('identity.sessions.title') }}</h2>
          <p>{{ i18n.t('identity.sessions.subtitle') }}</p>
        </div>
      </header>

      @if (error()) {
        <div class="form-error" role="alert">{{ error() }}</div>
      }

      @if (confirming()) {
        <div class="warning-panel" role="alert">
          {{ i18n.t('identity.sessions.confirmTitle') }}
        </div>
        <div class="form-actions">
          <button mat-button type="button" [disabled]="store.pending()" (click)="cancel()">
            {{ i18n.t('common.action.cancel') }}
          </button>
          <button mat-flat-button type="button" [disabled]="store.pending()" (click)="logoutAll()">
            {{ i18n.t('identity.sessions.confirm') }}
          </button>
        </div>
      } @else {
        <div class="form-actions">
          <button mat-stroked-button type="button" (click)="confirming.set(true)">
            {{ i18n.t('identity.sessions.logoutAll') }}
          </button>
        </div>
      }
    </section>
  `,
  styleUrl: './security-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SessionsPanel {
  readonly i18n = inject(I18nService);
  readonly store = inject(SecurityStore);
  readonly confirming = signal(false);
  readonly error = signal<string | null>(null);

  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);

  cancel(): void {
    this.confirming.set(false);
    this.error.set(null);
  }

  async logoutAll(): Promise<void> {
    this.error.set(null);
    try {
      await this.store.logoutAllSessions();
      await this.auth.refreshSession();
      await this.router.navigateByUrl('/login');
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    }
  }
}
