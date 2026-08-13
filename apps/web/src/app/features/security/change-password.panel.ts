import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, minLength, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router } from '@angular/router';

import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { identityErrorMessage } from '../auth/identity-error';
import { SecurityStore } from './security.store';

@Component({
  selector: 'app-change-password-panel',
  imports: [FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  template: `
    <section class="panel security-panel" aria-labelledby="change-password-title">
      <header class="panel-heading">
        <div>
          <h2 id="change-password-title">{{ i18n.t('identity.password.title') }}</h2>
          <p>{{ i18n.t('identity.password.subtitle') }}</p>
        </div>
      </header>

      <form class="security-form" (submit)="submit($event)" novalidate>
        <mat-form-field appearance="outline">
          <mat-label>{{ i18n.t('identity.field.currentPassword') }}</mat-label>
          <input
            matInput
            type="password"
            autocomplete="current-password"
            [formField]="passwordForm.currentPassword"
          />
          @if (
            passwordForm.currentPassword().touched() && passwordForm.currentPassword().invalid()
          ) {
            <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
          }
        </mat-form-field>

        <mat-form-field appearance="outline">
          <mat-label>{{ i18n.t('identity.field.newPassword') }}</mat-label>
          <input
            matInput
            type="password"
            autocomplete="new-password"
            [formField]="passwordForm.newPassword"
          />
          @if (passwordForm.newPassword().touched() && passwordForm.newPassword().invalid()) {
            <mat-error>{{ i18n.t('identity.validation.passwordLength') }}</mat-error>
          }
        </mat-form-field>

        <mat-form-field appearance="outline">
          <mat-label>{{ i18n.t('identity.field.confirmPassword') }}</mat-label>
          <input
            matInput
            type="password"
            autocomplete="new-password"
            [formField]="passwordForm.confirmPassword"
          />
          @if (
            passwordForm.confirmPassword().touched() &&
            (passwordForm.confirmPassword().invalid() || passwordMismatch())
          ) {
            <mat-error>{{
              i18n.t(
                passwordMismatch()
                  ? 'identity.validation.passwordMismatch'
                  : 'auth.validation.required'
              )
            }}</mat-error>
          }
        </mat-form-field>

        @if (error()) {
          <div class="form-error" role="alert">{{ error() }}</div>
        }

        <div class="form-actions">
          <button mat-flat-button type="submit" [disabled]="store.pending()">
            {{ i18n.t(store.pending() ? 'web.form.saving' : 'identity.password.submit') }}
          </button>
        </div>
      </form>
    </section>
  `,
  styleUrl: './security-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChangePasswordPanel {
  readonly i18n = inject(I18nService);
  readonly store = inject(SecurityStore);
  readonly error = signal<string | null>(null);
  readonly model = signal({ currentPassword: '', newPassword: '', confirmPassword: '' });
  readonly passwordForm = form(this.model, (schema) => {
    required(schema.currentPassword);
    required(schema.newPassword);
    minLength(schema.newPassword, 8);
    required(schema.confirmPassword);
  });
  readonly passwordMismatch = () =>
    this.passwordForm.confirmPassword().touched() &&
    this.model().newPassword !== this.model().confirmPassword;

  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);

  async submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    this.passwordForm().markAsTouched();
    const value = this.model();
    if (this.passwordForm().invalid() || value.newPassword !== value.confirmPassword) return;
    try {
      await this.store.changePassword(value.currentPassword, value.newPassword);
      await this.auth.refreshSession();
      await this.router.navigateByUrl('/login');
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    }
  }
}
