import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { ActivatedRoute, Router } from '@angular/router';

import { ApiClient } from '../../core/api/api-client.service';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { identityErrorMessage } from '../auth/identity-error';

@Component({
  selector: 'app-invitation-accept-page',
  imports: [MatButtonModule],
  template: `
    <div class="page invitation-page">
      <section class="panel invitation-card" aria-labelledby="invitation-title">
        <div class="invitation-mark" aria-hidden="true">+</div>
        <div>
          <h1 id="invitation-title">{{ i18n.t('identity.invitation.title') }}</h1>
          <p>{{ i18n.t('identity.invitation.subtitle') }}</p>
        </div>

        @if (!token()) {
          <div class="form-error" role="alert">{{ i18n.t('identity.invitation.missing') }}</div>
          <button mat-stroked-button type="button" (click)="goToDashboard()">
            {{ i18n.t('common.nav.dashboard') }}
          </button>
        } @else if (accepted()) {
          <div class="success-panel" role="status" aria-live="polite">
            {{ i18n.t('identity.invitation.success') }}
          </div>
          <button mat-flat-button type="button" (click)="goToDashboard()">
            {{ i18n.t('identity.action.continueToWorkspace') }}
          </button>
        } @else {
          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }
          <div class="actions">
            <button mat-button type="button" [disabled]="pending()" (click)="goToDashboard()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="button" [disabled]="pending()" (click)="accept()">
              {{
                i18n.t(pending() ? 'identity.invitation.accepting' : 'identity.invitation.accept')
              }}
            </button>
          </div>
        }
      </section>
    </div>
  `,
  styles: `
    .invitation-page {
      min-height: min(34rem, calc(100dvh - 8rem));
      place-items: center;
    }
    .invitation-card {
      display: grid;
      width: min(100%, 32rem);
      gap: 1rem;
      padding: clamp(1.25rem, 4vw, 2rem);
      text-align: center;
    }
    .invitation-mark {
      display: grid;
      width: 3rem;
      height: 3rem;
      margin-inline: auto;
      place-items: center;
      border-radius: 0.8rem;
      color: var(--brand);
      background: var(--brand-soft);
      font-size: 1.5rem;
      font-weight: 700;
    }
    h1 {
      margin: 0;
      font-size: 1.45rem;
      letter-spacing: -0.025em;
    }
    p {
      margin: 0.45rem 0 0;
      color: var(--text-muted);
    }
    .form-error,
    .success-panel {
      padding: 0.8rem 1rem;
      border-radius: 0.55rem;
    }
    .form-error {
      color: var(--danger);
      background: var(--danger-surface);
    }
    .success-panel {
      color: var(--brand);
      background: var(--brand-soft);
    }
    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 0.5rem;
    }
    @media (max-width: 520px) {
      .actions {
        align-items: stretch;
        flex-direction: column-reverse;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InvitationAcceptPage {
  readonly i18n = inject(I18nService);
  readonly token = signal(inject(ActivatedRoute).snapshot.queryParamMap.get('token')?.trim() ?? '');
  readonly pending = signal(false);
  readonly accepted = signal(false);
  readonly error = signal<string | null>(null);

  private readonly api = inject(ApiClient);
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);

  async accept(): Promise<void> {
    if (!this.token() || this.pending()) return;
    this.pending.set(true);
    this.error.set(null);
    try {
      await this.api.acceptInvitation({ token: this.token() });
      const authenticated = await this.auth.refreshSession();
      if (!authenticated) {
        const returnUrl = `/invitations/accept?token=${encodeURIComponent(this.token())}`;
        await this.router.navigate(['/login'], {
          queryParams: { returnUrl },
        });
        return;
      }
      this.accepted.set(true);
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    } finally {
      this.pending.set(false);
    }
  }

  async goToDashboard(): Promise<void> {
    await this.router.navigateByUrl('/dashboard');
  }
}
