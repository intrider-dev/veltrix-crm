import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router } from '@angular/router';

import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { identityErrorMessage } from '../auth/identity-error';
import { SecurityStore } from './security.store';

@Component({
  selector: 'app-mfa-panel',
  imports: [FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  template: `
    <section class="panel security-panel" aria-labelledby="mfa-title">
      <header class="panel-heading">
        <div>
          <h2 id="mfa-title">{{ i18n.t('identity.mfa.title') }}</h2>
          <p>{{ i18n.t('identity.mfa.subtitle') }}</p>
        </div>
        @if (!store.recoveryCodes()) {
          <span class="status-pill" [class.disabled]="!store.status()?.enabled">
            {{
              i18n.t(
                store.status()?.enabled
                  ? 'identity.mfa.statusEnabled'
                  : 'identity.mfa.statusDisabled'
              )
            }}
          </span>
        }
      </header>

      @if (store.recoveryCodes(); as recoveryCodes) {
        <div class="recovery-panel" role="status" aria-live="polite">
          <h3>{{ i18n.t('identity.mfa.recoveryTitle') }}</h3>
          <p>{{ i18n.t('identity.mfa.recoveryDescription') }}</p>
          <ul class="recovery-list" [attr.aria-label]="i18n.t('identity.mfa.recoveryTitle')">
            @for (code of recoveryCodes; track code) {
              <li>
                <code>{{ code }}</code>
              </li>
            }
          </ul>
          @if (copyError()) {
            <div class="form-error" role="alert">{{ copyError() }}</div>
          }
          <div class="form-actions">
            <button mat-stroked-button type="button" (click)="copyRecoveryCodes()">
              {{ i18n.t(copied() === 'codes' ? 'identity.mfa.copied' : 'identity.mfa.copyCodes') }}
            </button>
            <button mat-flat-button type="button" (click)="continueToLogin()">
              {{ i18n.t('identity.action.continueToLogin') }}
            </button>
          </div>
        </div>
      } @else if (store.setup(); as setup) {
        <div class="setup-details">
          <div>
            <strong>{{ i18n.t('identity.mfa.secret') }}</strong>
            <div class="secret-row">
              <code>{{ setup.secret }}</code>
              <button mat-button type="button" (click)="copySecret(setup.secret)">
                {{ i18n.t(copied() === 'secret' ? 'identity.mfa.copied' : 'identity.mfa.copy') }}
              </button>
            </div>
          </div>
          <details>
            <summary>{{ i18n.t('identity.mfa.provisioningUri') }}</summary>
            <code class="uri">{{ setup.provisioningUri }}</code>
          </details>
          <p class="supporting-copy">{{ i18n.t('identity.mfa.secretHelp') }}</p>
        </div>

        <form class="security-form" (submit)="confirmSetup($event)" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.code') }}</mat-label>
            <input
              matInput
              inputmode="numeric"
              autocomplete="one-time-code"
              [formField]="confirmForm.code"
            />
            @if (confirmForm.code().touched() && confirmForm.code().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }
          <div class="form-actions">
            <button mat-button type="button" [disabled]="store.pending()" (click)="cancelSetup()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="submit" [disabled]="store.pending()">
              {{ i18n.t('identity.mfa.confirm') }}
            </button>
          </div>
        </form>
      } @else if (!store.status()?.enabled) {
        <form class="security-form" (submit)="beginSetup($event)" novalidate>
          <p class="supporting-copy">{{ i18n.t('identity.mfa.enableSubtitle') }}</p>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.currentPassword') }}</mat-label>
            <input
              matInput
              type="password"
              autocomplete="current-password"
              [formField]="beginForm.currentPassword"
            />
            @if (beginForm.currentPassword().touched() && beginForm.currentPassword().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }
          <div class="form-actions">
            <button mat-flat-button type="submit" [disabled]="store.pending()">
              {{ i18n.t('identity.mfa.enable') }}
            </button>
          </div>
        </form>
      } @else {
        <form class="security-form" (submit)="regenerate($event)" novalidate>
          <p class="supporting-copy">{{ i18n.t('identity.mfa.manageSubtitle') }}</p>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.currentPassword') }}</mat-label>
            <input
              matInput
              type="password"
              autocomplete="current-password"
              [formField]="protectedForm.currentPassword"
            />
            @if (
              protectedForm.currentPassword().touched() && protectedForm.currentPassword().invalid()
            ) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.code') }}</mat-label>
            <input matInput autocomplete="one-time-code" [formField]="protectedForm.code" />
            @if (protectedForm.code().touched() && protectedForm.code().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }
          @if (confirmingDisable()) {
            <div class="warning-panel" role="alert">
              {{ i18n.t('identity.mfa.disableConfirm') }}
            </div>
          }
          <div class="form-actions">
            @if (confirmingDisable()) {
              <button
                mat-button
                type="button"
                [disabled]="store.pending()"
                (click)="cancelDisable()"
              >
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button
                mat-flat-button
                type="button"
                [disabled]="store.pending()"
                (click)="disable()"
              >
                {{ i18n.t('identity.mfa.disable') }}
              </button>
            } @else {
              <button mat-stroked-button type="button" (click)="requestDisable()">
                {{ i18n.t('identity.mfa.disable') }}
              </button>
              <button mat-flat-button type="submit" [disabled]="store.pending()">
                {{ i18n.t('identity.mfa.regenerate') }}
              </button>
            }
          </div>
        </form>
      }
    </section>
  `,
  styleUrl: './security-panel.scss',
  styles: `
    .recovery-panel,
    .setup-details {
      display: grid;
      gap: 0.85rem;
    }
    .recovery-panel h3,
    .recovery-panel p {
      margin: 0;
    }
    .recovery-list {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.45rem;
      margin: 0;
      padding: 0;
      list-style: none;
    }
    .recovery-list li,
    .secret-row,
    details {
      border: 1px solid var(--border);
      border-radius: 0.5rem;
      background: var(--surface-subtle);
    }
    .recovery-list li {
      padding: 0.55rem 0.7rem;
      text-align: center;
    }
    .secret-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 0.75rem;
      margin-top: 0.4rem;
      padding: 0.4rem 0.45rem 0.4rem 0.75rem;
    }
    .secret-row code,
    .uri {
      overflow-wrap: anywhere;
    }
    details {
      padding: 0.7rem 0.8rem;
    }
    summary {
      cursor: pointer;
      font-weight: 600;
    }
    .uri {
      display: block;
      margin-top: 0.75rem;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    @media (max-width: 520px) {
      .recovery-list {
        grid-template-columns: 1fr;
      }
      .secret-row {
        align-items: stretch;
        flex-direction: column;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MFAPanel {
  readonly i18n = inject(I18nService);
  readonly store = inject(SecurityStore);
  readonly error = signal<string | null>(null);
  readonly copyError = signal<string | null>(null);
  readonly copied = signal<'secret' | 'codes' | null>(null);
  readonly confirmingDisable = signal(false);
  readonly beginModel = signal({ currentPassword: '' });
  readonly confirmModel = signal({ code: '' });
  readonly protectedModel = signal({ currentPassword: '', code: '' });
  readonly beginForm = form(this.beginModel, (schema) => required(schema.currentPassword));
  readonly confirmForm = form(this.confirmModel, (schema) => required(schema.code));
  readonly protectedForm = form(this.protectedModel, (schema) => {
    required(schema.currentPassword);
    required(schema.code);
  });

  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);

  async beginSetup(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    this.beginForm().markAsTouched();
    if (this.beginForm().invalid()) return;
    try {
      await this.store.beginMFASetup(this.beginModel().currentPassword);
      this.beginModel.set({ currentPassword: '' });
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    }
  }

  async confirmSetup(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    this.confirmForm().markAsTouched();
    if (this.confirmForm().invalid()) return;
    try {
      await this.store.confirmMFASetup(this.confirmModel().code.trim());
      this.confirmModel.set({ code: '' });
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    }
  }

  cancelSetup(): void {
    this.store.clearSetup();
    this.confirmModel.set({ code: '' });
    this.error.set(null);
  }

  async regenerate(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    if (!this.validateProtectedForm()) return;
    const value = this.protectedModel();
    try {
      await this.store.regenerateRecoveryCodes(value.currentPassword, value.code.trim());
      this.protectedModel.set({ currentPassword: '', code: '' });
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    }
  }

  requestDisable(): void {
    this.error.set(null);
    if (this.validateProtectedForm()) this.confirmingDisable.set(true);
  }

  cancelDisable(): void {
    this.confirmingDisable.set(false);
  }

  async disable(): Promise<void> {
    this.error.set(null);
    if (!this.validateProtectedForm()) return;
    const value = this.protectedModel();
    try {
      await this.store.disableMFA(value.currentPassword, value.code.trim());
      await this.auth.refreshSession();
      await this.router.navigateByUrl('/login');
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
      this.confirmingDisable.set(false);
    }
  }

  async copySecret(secret: string): Promise<void> {
    await this.copy(secret, 'secret');
  }

  async copyRecoveryCodes(): Promise<void> {
    await this.copy((this.store.recoveryCodes() ?? []).join('\n'), 'codes');
  }

  async continueToLogin(): Promise<void> {
    await this.auth.refreshSession();
    await this.router.navigateByUrl('/login');
  }

  private validateProtectedForm(): boolean {
    this.protectedForm().markAsTouched();
    return this.protectedForm().valid();
  }

  private async copy(value: string, target: 'secret' | 'codes'): Promise<void> {
    this.copyError.set(null);
    try {
      await navigator.clipboard.writeText(value);
      this.copied.set(target);
    } catch {
      this.copyError.set(this.i18n.t('identity.mfa.copyFailed'));
    }
  }
}
