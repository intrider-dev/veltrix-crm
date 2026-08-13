import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, email, form, minLength, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { ActivatedRoute, RouterLink } from '@angular/router';

import { ApiClient } from '../../core/api/api-client.service';
import { I18nService } from '../../core/i18n/i18n.service';
import { AuthShellComponent } from './auth-shell.component';
import { identityErrorMessage } from './identity-error';

@Component({
  selector: 'app-password-reset-page',
  imports: [
    AuthShellComponent,
    FormField,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  template: `
    <app-auth-shell
      [title]="i18n.t(token() ? 'identity.reset.confirmTitle' : 'identity.reset.requestTitle')"
      [subtitle]="
        i18n.t(token() ? 'identity.reset.confirmSubtitle' : 'identity.reset.requestSubtitle')
      "
    >
      @if (complete()) {
        <div class="success-panel" role="status" aria-live="polite">
          {{ i18n.t(token() ? 'identity.reset.confirmSuccess' : 'identity.reset.requestSuccess') }}
        </div>
        <a mat-flat-button class="submit" routerLink="/login">
          {{ i18n.t('identity.action.continueToLogin') }}
        </a>
      } @else if (token()) {
        <form (submit)="confirm($event)" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.newPassword') }}</mat-label>
            <input
              matInput
              type="password"
              autocomplete="new-password"
              [formField]="confirmationForm.newPassword"
            />
            @if (
              confirmationForm.newPassword().touched() && confirmationForm.newPassword().invalid()
            ) {
              <mat-error>{{ i18n.t('identity.validation.passwordLength') }}</mat-error>
            }
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.confirmPassword') }}</mat-label>
            <input
              matInput
              type="password"
              autocomplete="new-password"
              [formField]="confirmationForm.confirmPassword"
            />
            @if (
              confirmationForm.confirmPassword().touched() &&
              (confirmationForm.confirmPassword().invalid() || passwordMismatch())
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
          <button mat-flat-button class="submit" type="submit" [disabled]="pending()">
            {{ i18n.t(pending() ? 'web.form.saving' : 'identity.reset.confirmSubmit') }}
          </button>
        </form>
      } @else {
        <form (submit)="request($event)" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('auth.login.email') }}</mat-label>
            <input
              matInput
              inputmode="email"
              autocomplete="email"
              [formField]="requestForm.email"
            />
            @if (requestForm.email().touched() && requestForm.email().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
            }
          </mat-form-field>
          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }
          <button mat-flat-button class="submit" type="submit" [disabled]="pending()">
            {{ i18n.t(pending() ? 'web.form.saving' : 'identity.reset.requestSubmit') }}
          </button>
          <a mat-button class="footer-link" routerLink="/login">
            {{ i18n.t('identity.action.backToLogin') }}
          </a>
        </form>
      }
    </app-auth-shell>
  `,
  styleUrl: './auth-flow.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PasswordResetPage {
  readonly i18n = inject(I18nService);
  readonly token = signal(inject(ActivatedRoute).snapshot.queryParamMap.get('token')?.trim() ?? '');
  readonly pending = signal(false);
  readonly complete = signal(false);
  readonly error = signal<string | null>(null);
  readonly requestModel = signal({ email: '' });
  readonly confirmationModel = signal({ newPassword: '', confirmPassword: '' });
  readonly requestForm = form(this.requestModel, (schema) => {
    required(schema.email);
    email(schema.email);
  });
  readonly confirmationForm = form(this.confirmationModel, (schema) => {
    required(schema.newPassword);
    minLength(schema.newPassword, 8);
    required(schema.confirmPassword);
  });
  readonly passwordMismatch = () =>
    this.confirmationForm.confirmPassword().touched() &&
    this.confirmationModel().newPassword !== this.confirmationModel().confirmPassword;

  private readonly api = inject(ApiClient);

  async request(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    this.requestForm().markAsTouched();
    if (this.requestForm().invalid()) return;
    this.pending.set(true);
    try {
      await this.api.requestPasswordReset({ email: this.requestModel().email.trim() });
      this.complete.set(true);
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    } finally {
      this.pending.set(false);
    }
  }

  async confirm(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    this.confirmationForm().markAsTouched();
    const value = this.confirmationModel();
    if (this.confirmationForm().invalid() || value.newPassword !== value.confirmPassword) return;
    this.pending.set(true);
    try {
      await this.api.confirmPasswordReset({ token: this.token(), newPassword: value.newPassword });
      this.complete.set(true);
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    } finally {
      this.pending.set(false);
    }
  }
}
