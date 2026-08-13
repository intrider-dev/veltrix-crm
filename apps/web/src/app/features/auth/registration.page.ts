import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormField, email, form, minLength, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type { DevelopmentRegistrationRequest, RegisteredUser } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { AuthShellComponent } from './auth-shell.component';
import { identityErrorMessage } from './identity-error';

@Component({
  selector: 'app-registration-page',
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
      [title]="i18n.t('identity.registration.title')"
      [subtitle]="i18n.t('identity.registration.subtitle')"
    >
      @if (registered(); as user) {
        <div class="success-panel" role="status" aria-live="polite">
          {{ i18n.t('identity.registration.success', { email: user.email }) }}
        </div>
        <a mat-flat-button class="submit" routerLink="/login">
          {{ i18n.t('identity.action.continueToLogin') }}
        </a>
      } @else {
        <form (submit)="submit($event)" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.registration.displayName') }}</mat-label>
            <input matInput autocomplete="name" [formField]="registrationForm.displayName" />
            @if (
              registrationForm.displayName().touched() && registrationForm.displayName().invalid()
            ) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('auth.login.email') }}</mat-label>
            <input
              matInput
              autocomplete="email"
              inputmode="email"
              [formField]="registrationForm.email"
              [attr.aria-invalid]="emailServerError() ? 'true' : null"
              [attr.aria-describedby]="emailServerError() ? 'registration-email-error' : null"
              (input)="duplicateEmailError.set(false)"
            />
            @if (registrationForm.email().touched() && registrationForm.email().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
            }
          </mat-form-field>
          @if (emailServerError()) {
            <p class="field-error" id="registration-email-error" role="alert">
              {{ emailServerError() }}
            </p>
          }
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('auth.login.password') }}</mat-label>
            <input
              matInput
              type="password"
              autocomplete="new-password"
              [formField]="registrationForm.password"
            />
            @if (registrationForm.password().touched() && registrationForm.password().invalid()) {
              <mat-error>{{ i18n.t('identity.validation.passwordLength') }}</mat-error>
            }
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.field.confirmPassword') }}</mat-label>
            <input
              matInput
              type="password"
              autocomplete="new-password"
              [formField]="registrationForm.confirmPassword"
            />
            @if (
              registrationForm.confirmPassword().touched() &&
              (registrationForm.confirmPassword().invalid() || passwordMismatch())
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
            {{
              i18n.t(pending() ? 'identity.action.creatingAccount' : 'identity.registration.submit')
            }}
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
export class RegistrationPage {
  readonly i18n = inject(I18nService);
  readonly pending = signal(false);
  private readonly submissionError = signal<unknown>(null);
  readonly error = computed(() => {
    const error = this.submissionError();
    return error ? identityErrorMessage(this.i18n, error) : null;
  });
  readonly duplicateEmailError = signal(false);
  readonly emailServerError = computed(() =>
    this.duplicateEmailError() ? this.i18n.t('identity.validation.emailAlreadyUsed') : null,
  );
  readonly registered = signal<RegisteredUser | null>(null);
  readonly model = signal<{
    email: string;
    displayName: string;
    password: string;
    confirmPassword: string;
    locale: SupportedLocale;
  }>({
    email: '',
    displayName: '',
    password: '',
    confirmPassword: '',
    locale: this.i18n.locale(),
  });
  readonly registrationForm = form(this.model, (schema) => {
    required(schema.email);
    email(schema.email);
    required(schema.displayName);
    required(schema.password);
    minLength(schema.password, 8);
    required(schema.confirmPassword);
  });
  readonly passwordMismatch = () =>
    this.registrationForm.confirmPassword().touched() &&
    this.model().password !== this.model().confirmPassword;

  private readonly api = inject(ApiClient);

  async submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.submissionError.set(null);
    this.duplicateEmailError.set(false);
    this.model.update((value) => ({
      ...value,
      email: value.email.trim(),
      displayName: value.displayName.trim(),
    }));
    this.registrationForm.email().markAsTouched();
    this.registrationForm.displayName().markAsTouched();
    this.registrationForm.password().markAsTouched();
    this.registrationForm.confirmPassword().markAsTouched();
    if (this.registrationForm().invalid() || this.model().password !== this.model().confirmPassword)
      return;
    this.pending.set(true);
    try {
      const value = this.model();
      const body: DevelopmentRegistrationRequest = {
        email: value.email.trim(),
        displayName: value.displayName.trim(),
        password: value.password,
        locale: this.i18n.locale(),
      };
      this.registered.set(await this.api.registerDevelopmentUser(body));
    } catch (error) {
      const emailError = this.registrationEmailError(error);
      if (emailError) this.duplicateEmailError.set(true);
      else this.submissionError.set(error);
    } finally {
      this.pending.set(false);
    }
  }

  private registrationEmailError(error: unknown): boolean {
    if (!(error instanceof ApiError)) return false;
    const field = error.problem?.fieldErrors?.find(({ pointer }) => pointer === '/email');
    return field?.code === 'validation.email.alreadyUsed';
  }
}
