import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormField, email, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { ApiError } from '../../core/api/api-error';
import { safeLocalReturnUrl } from '../../core/auth/auth.guard';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { IconComponent } from '../../shared/icon/icon.component';
import { AuthShellComponent } from './auth-shell.component';

@Component({
  selector: 'app-login-page',
  imports: [
    AuthShellComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  template: `
    <app-auth-shell
      [title]="i18n.t(mfaChallenge() ? 'auth.mfa.title' : 'auth.login.title')"
      [subtitle]="i18n.t(mfaChallenge() ? 'auth.mfa.subtitle' : 'auth.login.subtitle')"
    >
      @if (mfaChallenge()) {
        <form (submit)="verifyMFA($event)" novalidate>
          <mat-form-field appearance="outline" [hideRequiredMarker]="true">
            <mat-label>{{ i18n.t('auth.mfa.code') }}</mat-label>
            <input
              matInput
              autocomplete="one-time-code"
              inputmode="numeric"
              [formField]="mfaForm.code"
            />
            <mat-hint>{{ i18n.t('auth.mfa.recoveryHint') }}</mat-hint>
            @if (mfaForm.code().touched() && mfaForm.code().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>

          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }

          <button mat-flat-button class="submit" type="submit" [disabled]="auth.pending()">
            {{ i18n.t(auth.pending() ? 'web.login.progress' : 'auth.mfa.submit') }}
          </button>
          <button mat-button class="secondary-action" type="button" (click)="backToPassword()">
            {{ i18n.t('auth.login.back') }}
          </button>
        </form>
      } @else {
        <form (submit)="submit($event)" novalidate>
          <label class="field-label" for="login-email">{{ i18n.t('auth.login.email') }}</label>
          <mat-form-field appearance="outline" [hideRequiredMarker]="true">
            <app-icon matPrefix name="mail" />
            <input
              id="login-email"
              matInput
              autocomplete="username"
              inputmode="email"
              [placeholder]="i18n.t('auth.login.email')"
              [formField]="loginForm.email"
            />
            @if (loginForm.email().touched() && loginForm.email().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
            }
          </mat-form-field>

          <label class="field-label" for="login-password">{{
            i18n.t('auth.login.password')
          }}</label>
          <mat-form-field appearance="outline" class="password-field" [hideRequiredMarker]="true">
            <input
              id="login-password"
              matInput
              autocomplete="current-password"
              [placeholder]="i18n.t('auth.login.password')"
              [type]="passwordVisible() ? 'text' : 'password'"
              [formField]="loginForm.password"
            />
            <button
              mat-icon-button
              matSuffix
              type="button"
              class="visibility-button"
              (click)="passwordVisible.set(!passwordVisible())"
              [attr.aria-label]="i18n.t('auth.login.showPassword')"
              [attr.aria-pressed]="passwordVisible()"
            >
              <app-icon [name]="passwordVisible() ? 'eyeOff' : 'eye'" />
            </button>
            @if (loginForm.password().touched() && loginForm.password().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          <a class="forgot-link" routerLink="/password-reset">{{ i18n.t('auth.login.forgot') }}</a>

          @if (error()) {
            <div class="form-error" role="alert">{{ error() }}</div>
          }

          <button mat-flat-button class="submit" type="submit" [disabled]="auth.pending()">
            {{ i18n.t(auth.pending() ? 'web.login.progress' : 'auth.login.submit') }}
          </button>
          <p class="account-switch">
            {{ i18n.t('auth.login.noAccount') }}
            <a routerLink="/register">{{ i18n.t('auth.login.createAccount') }}</a>
          </p>
        </form>
      }
    </app-auth-shell>
  `,
  styleUrl: './auth-flow.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoginPage {
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  readonly passwordVisible = signal(false);
  private readonly problem = signal<{ readonly code: string; readonly requestId: string } | null>(
    null,
  );
  readonly error = computed(() => {
    const problem = this.problem();
    return problem ? this.i18n.problem(problem.code, problem.requestId) : null;
  });
  readonly mfaChallenge = signal<string | null>(null);
  readonly model = signal({ email: '', password: '' });
  readonly mfaModel = signal({ code: '' });
  readonly loginForm = form(this.model, (schema) => {
    required(schema.email);
    email(schema.email);
    required(schema.password);
  });
  readonly mfaForm = form(this.mfaModel, (schema) => required(schema.code));

  private readonly router = inject(Router);
  private readonly returnUrl = safeLocalReturnUrl(
    inject(ActivatedRoute).snapshot.queryParamMap.get('returnUrl'),
  );

  async submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.problem.set(null);
    if (this.loginForm().invalid()) {
      this.loginForm.email().markAsTouched();
      this.loginForm.password().markAsTouched();
      return;
    }
    try {
      const value = this.model();
      const challenge = await this.auth.login(value.email.trim(), value.password);
      if (challenge) {
        this.mfaChallenge.set(challenge.challengeToken);
        return;
      }
      await this.router.navigateByUrl(this.returnUrl);
    } catch (error) {
      const apiError = error instanceof ApiError ? error : null;
      this.problem.set({
        code: apiError?.problem?.code ?? 'network',
        requestId: apiError?.problem?.requestId ?? '',
      });
    }
  }

  async verifyMFA(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.problem.set(null);
    const challengeToken = this.mfaChallenge();
    if (!challengeToken || this.mfaForm().invalid()) {
      this.mfaForm.code().markAsTouched();
      return;
    }
    try {
      await this.auth.verifyMFA(challengeToken, this.mfaModel().code.trim());
      await this.router.navigateByUrl(this.returnUrl);
    } catch (error) {
      const apiError = error instanceof ApiError ? error : null;
      this.problem.set({
        code: apiError?.problem?.code ?? 'network',
        requestId: apiError?.problem?.requestId ?? '',
      });
    }
  }

  backToPassword(): void {
    this.mfaChallenge.set(null);
    this.mfaModel.set({ code: '' });
    this.problem.set(null);
  }
}
