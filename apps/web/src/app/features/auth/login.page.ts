import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormField, email, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { productConfig, type SupportedLocale } from '@veltrix-crm/product-config';

import { ApiError } from '../../core/api/api-error';
import { safeLocalReturnUrl } from '../../core/auth/auth.guard';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { BrandLogoComponent } from '../../shared/brand/brand-logo.component';

@Component({
  selector: 'app-login-page',
  imports: [
    BrandLogoComponent,
    FormField,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  template: `
    <main class="login-layout">
      <section class="story" aria-labelledby="product-name">
        <app-brand-logo [showName]="false" size="large" />
        <p class="eyebrow">{{ product.shortName }}</p>
        <h1 id="product-name">
          {{ i18n.t('common.product.welcome', { productName: product.productName }) }}
        </h1>
        <p>{{ i18n.t('auth.login.subtitle') }}</p>
      </section>

      <mat-card appearance="outlined" class="login-card">
        <div
          class="locale-switch"
          role="group"
          [attr.aria-label]="i18n.t('settings.language.label')"
        >
          @for (locale of i18n.supportedLocales; track locale) {
            <button
              mat-button
              type="button"
              [class.active]="i18n.locale() === locale"
              (click)="setLocale(locale)"
            >
              {{ i18n.languageName(locale) }}
            </button>
          }
        </div>
        <mat-card-header>
          <mat-card-title>{{ i18n.t('auth.login.title') }}</mat-card-title>
          <mat-card-subtitle>{{ i18n.t('auth.login.subtitle') }}</mat-card-subtitle>
        </mat-card-header>
        <mat-card-content>
          @if (mfaChallenge()) {
            <form (submit)="verifyMFA($event)" novalidate>
              <p class="mfa-copy">{{ i18n.t('auth.mfa.subtitle') }}</p>
              <mat-form-field appearance="outline">
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
              <button mat-button class="submit" type="button" (click)="backToPassword()">
                {{ i18n.t('auth.login.back') }}
              </button>
            </form>
          } @else {
            <form (submit)="submit($event)" novalidate>
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('auth.login.email') }}</mat-label>
                <input
                  matInput
                  autocomplete="username"
                  inputmode="email"
                  [formField]="loginForm.email"
                />
                @if (loginForm.email().touched() && loginForm.email().invalid()) {
                  <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
                }
              </mat-form-field>

              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('auth.login.password') }}</mat-label>
                <input
                  matInput
                  autocomplete="current-password"
                  [type]="passwordVisible() ? 'text' : 'password'"
                  [formField]="loginForm.password"
                />
                <button
                  mat-button
                  matSuffix
                  type="button"
                  (click)="passwordVisible.set(!passwordVisible())"
                  [attr.aria-pressed]="passwordVisible()"
                >
                  {{ i18n.t('auth.login.showPassword') }}
                </button>
                @if (loginForm.password().touched() && loginForm.password().invalid()) {
                  <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
                }
              </mat-form-field>

              <a mat-button class="forgot" routerLink="/password-reset">
                {{ i18n.t('auth.login.forgot') }}
              </a>

              @if (error()) {
                <div class="form-error" role="alert">{{ error() }}</div>
              }

              <button mat-flat-button class="submit" type="submit" [disabled]="auth.pending()">
                {{ i18n.t(auth.pending() ? 'web.login.progress' : 'auth.login.submit') }}
              </button>
              <a mat-button class="registration" routerLink="/register">
                {{ i18n.t('auth.login.developmentRegister') }}
              </a>
            </form>
          }
        </mat-card-content>
      </mat-card>
    </main>
  `,
  styles: `
    :host {
      min-height: 100dvh;
      display: block;
      background: var(--surface-canvas);
    }
    .login-layout {
      min-height: 100dvh;
      display: grid;
      grid-template-columns: minmax(0, 1fr) minmax(22rem, 31rem);
      align-items: center;
      gap: clamp(3rem, 8vw, 8rem);
      max-width: 76rem;
      margin: auto;
      padding: clamp(1.25rem, 5vw, 4rem);
    }
    .story {
      max-width: 34rem;
    }
    .brand-mark {
      display: grid;
      place-items: center;
      width: 2.75rem;
      height: 2.75rem;
      border-radius: 0.8rem;
      color: white;
      background: var(--brand);
      font-weight: 750;
      box-shadow: 0 0.5rem 1.5rem color-mix(in srgb, var(--brand) 24%, transparent);
    }
    .eyebrow {
      margin: 2rem 0 0.5rem;
      color: var(--brand);
      font-weight: 700;
      letter-spacing: 0.03em;
    }
    h1 {
      max-width: 12ch;
      margin: 0;
      font-size: clamp(2.5rem, 6vw, 4.75rem);
      line-height: 0.98;
      letter-spacing: -0.055em;
      overflow-wrap: normal;
      word-break: normal;
    }
    .story > p:last-child {
      max-width: 48ch;
      color: var(--text-muted);
      font-size: 1.05rem;
      line-height: 1.65;
    }
    .login-card {
      padding: 0.5rem;
      border-color: var(--border);
      background: var(--surface-raised);
      box-shadow: var(--shadow-lg);
    }
    mat-card-header {
      padding: 1rem 1rem 1.5rem;
    }
    mat-card-title {
      font-size: 1.5rem;
      font-weight: 720;
      letter-spacing: -0.02em;
    }
    mat-card-subtitle {
      margin-top: 0.35rem;
    }
    form {
      display: grid;
      gap: 0.25rem;
    }
    mat-form-field,
    .submit {
      width: 100%;
    }
    .submit {
      min-height: 2.9rem;
      margin-top: 0.5rem;
    }
    .forgot {
      justify-self: end;
      margin: -0.65rem 0 0.25rem;
    }
    .registration {
      justify-self: center;
    }
    .mfa-copy {
      margin: 0 0 1rem;
      color: var(--text-muted);
      line-height: 1.5;
    }
    .locale-switch {
      display: flex;
      justify-content: flex-end;
      padding: 0.35rem;
      gap: 0.15rem;
    }
    .locale-switch button {
      min-width: auto;
      padding-inline: 0.75rem;
    }
    .locale-switch .active {
      background: var(--surface-selected);
      color: var(--brand);
    }
    .form-error {
      margin: 0 0 0.5rem;
      border-left: 3px solid var(--danger);
      padding: 0.7rem 0.8rem;
      color: var(--danger);
      background: var(--danger-surface);
      border-radius: 0.25rem;
    }
    @media (max-width: 760px) {
      .login-layout {
        grid-template-columns: 1fr;
        gap: 2.5rem;
      }
      .story > p:last-child {
        display: none;
      }
      h1 {
        font-size: 2.25rem;
      }
      .eyebrow {
        margin-top: 1.25rem;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoginPage {
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  readonly product = productConfig;
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

  async setLocale(locale: SupportedLocale): Promise<void> {
    await this.i18n.setLocale(locale);
  }

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
