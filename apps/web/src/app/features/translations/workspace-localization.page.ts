import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, disabled, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatSelectModule } from '@angular/material/select';
import { RouterLink } from '@angular/router';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import { AuthStore } from '../../core/auth/auth.store';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { LocalizationSettingsStore } from './localization-settings.store';

@Component({
  selector: 'app-workspace-localization-page',
  imports: [ErrorPanelComponent, FormField, MatButtonModule, MatSelectModule, RouterLink],
  providers: [LocalizationSettingsStore],
  template: `
    <div class="page locale-settings-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('translations.settingsTitle') }}</h1>
          <p>{{ i18n.t('translations.settingsSubtitle') }}</p>
        </div>
        <a mat-stroked-button routerLink="/settings/translations">
          {{ i18n.t('translations.openCenter') }}
        </a>
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="load()" />
      }
      @if (store.conflict()) {
        <section class="conflict-panel" role="alert">
          <span>{{ i18n.t('translations.policyConflict') }}</span>
          <button mat-button type="button" (click)="load()">
            {{ i18n.t('common.action.retry') }}
          </button>
        </section>
      }
      @if (validationError()) {
        <section class="validation-panel" role="alert">{{ validationError() }}</section>
      }
      @if (store.saved()) {
        <section class="saved-panel" role="status" aria-live="polite">
          {{ i18n.t('translations.policySaved') }}
        </section>
      }

      <section class="panel settings-card" [attr.aria-busy]="store.loading()">
        @if (store.loading() && !store.settings()) {
          <div class="skeleton policy-skeleton"></div>
        } @else if (store.settings()) {
          <form (submit)="save($event)" novalidate>
            <label class="native-field">
              <span>{{ i18n.t('translations.defaultLocale') }}</span>
              <mat-select
                panelClass="crm-select-panel"
                class="crm-select"
                [aria-label]="i18n.t('translations.defaultLocale')"
                [formField]="policyForm.defaultLocale"
              >
                @for (locale of supported(); track locale) {
                  <mat-option [value]="locale">{{ i18n.languageName(locale) }}</mat-option>
                }
              </mat-select>
            </label>

            <fieldset>
              <legend>{{ i18n.t('translations.supportedLocales') }}</legend>
              <p>{{ i18n.t('translations.supportedLocalesHint') }}</p>
              <div class="locale-options">
                @for (locale of i18n.supportedLocales; track locale) {
                  <label>
                    <input
                      type="checkbox"
                      [checked]="supported().includes(locale)"
                      [disabled]="!permissions.allows('settings.write')"
                      (change)="toggleLocale(locale, $event)"
                    />
                    <span>{{ i18n.languageName(locale) }}</span>
                    <code>{{ locale }}</code>
                  </label>
                }
              </div>
            </fieldset>

            @if (!permissions.allows('settings.write')) {
              <p class="permission-note">{{ i18n.t('translations.policyPermission') }}</p>
            } @else {
              <div class="form-actions">
                <button mat-flat-button type="submit" [disabled]="store.saving()">
                  {{ i18n.t(store.saving() ? 'web.form.saving' : 'translations.savePolicy') }}
                </button>
              </div>
            }
          </form>
        }
      </section>
    </div>
  `,
  styles: `
    :host {
      --locale-surface: var(--color-surface, var(--surface-raised));
      --locale-subtle: var(--color-surface-container, var(--surface-subtle));
      --locale-hover: var(--color-surface-container-high, var(--surface-selected));
      --locale-border: var(--color-outline-variant, var(--border));
      display: block;
    }
    .locale-settings-page {
      width: min(100%, 62rem);
      margin-inline: auto;
    }
    .settings-card {
      padding: clamp(1rem, 3vw, 1.5rem);
      border: 1px solid var(--locale-border);
      border-radius: 0.9rem;
      background: var(--locale-surface);
    }
    form {
      display: grid;
      gap: 1.25rem;
    }
    .native-field {
      display: grid;
      gap: 0.4rem;
      max-width: 22rem;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    .native-field .mat-mdc-select {
      min-height: 2.75rem;
      padding-inline: 0.8rem 2.4rem;
      border: 1px solid var(--locale-border);
      border-radius: 0.65rem;
      color: var(--text);
      background-color: var(--locale-surface);
    }
    fieldset {
      margin: 0;
      padding: 0;
      border: 0;
    }
    legend {
      font-weight: 700;
    }
    fieldset > p,
    .permission-note {
      margin: 0.35rem 0 0.8rem;
      color: var(--text-muted);
      font-size: 0.82rem;
    }
    .locale-options {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
      gap: 0.6rem;
    }
    .locale-options label {
      display: grid;
      grid-template-columns: auto 1fr auto;
      align-items: center;
      gap: 0.65rem;
      min-height: 3.5rem;
      padding: 0.75rem 0.9rem;
      border: 1px solid var(--locale-border);
      border-radius: 0.7rem;
      background: var(--locale-subtle);
      transition:
        border-color 120ms ease,
        background-color 120ms ease;
    }
    .locale-options input {
      width: 1.05rem;
      height: 1.05rem;
      accent-color: var(--brand);
    }
    code {
      color: var(--text-muted);
      font-size: 0.72rem;
    }
    .form-actions {
      display: flex;
      justify-content: flex-end;
    }
    .conflict-panel,
    .validation-panel,
    .saved-panel {
      padding: 0.75rem 0.9rem;
      border: 1px solid transparent;
      border-radius: 0.65rem;
    }
    .conflict-panel {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      color: var(--danger);
      border-color: color-mix(in srgb, var(--danger) 24%, transparent);
      background: var(--danger-surface);
    }
    .validation-panel {
      color: var(--danger);
      border-color: color-mix(in srgb, var(--danger) 24%, transparent);
      background: var(--danger-surface);
    }
    .saved-panel {
      color: var(--brand);
      border-color: color-mix(in srgb, var(--brand) 24%, transparent);
      background: var(--brand-soft);
    }
    .policy-skeleton {
      min-height: 14rem;
    }
    @media (hover: hover) and (pointer: fine) {
      .locale-options label:hover {
        border-color: color-mix(in srgb, var(--brand) 32%, var(--locale-border));
        background: var(--locale-hover);
      }
    }
    @media (max-width: 520px) {
      .page-header {
        align-items: stretch;
      }
      .page-header a,
      .form-actions button {
        width: 100%;
      }
      .conflict-panel {
        align-items: stretch;
        flex-direction: column;
      }
    }
    @media (prefers-reduced-motion: reduce) {
      .locale-options label {
        transition-duration: 0ms;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkspaceLocalizationPage implements OnInit {
  readonly store = inject(LocalizationSettingsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly supported = signal<readonly SupportedLocale[]>([]);
  readonly validationError = signal<string | null>(null);
  readonly policyModel = signal<{ defaultLocale: SupportedLocale }>({
    defaultLocale: this.i18n.supportedLocales[0],
  });
  readonly policyForm = form(this.policyModel, (schema) => {
    required(schema.defaultLocale);
    disabled(schema.defaultLocale, { when: () => !this.permissions.allows('settings.write') });
  });

  private readonly auth = inject(AuthStore);
  private readonly workspace = inject(WorkspaceStore);

  ngOnInit(): void {
    void this.load();
  }

  async load(): Promise<void> {
    const settings = await this.store.load();
    if (!settings) return;
    const supported = settings.supportedLocales
      .map((locale) => this.i18n.supportedLocale(locale))
      .filter((locale): locale is SupportedLocale => locale !== null);
    const defaultLocale = this.i18n.supportedLocale(settings.defaultLocale);
    this.supported.set(supported);
    this.policyModel.set({ defaultLocale: defaultLocale ?? supported[0] ?? 'en' });
    this.validationError.set(null);
  }

  toggleLocale(locale: SupportedLocale, event: Event): void {
    const checked = (event.target as HTMLInputElement).checked;
    this.supported.update((current) =>
      checked
        ? [...new Set([...current, locale])]
        : current.filter((candidate) => candidate !== locale),
    );
    if (!this.supported().includes(this.policyModel().defaultLocale) && this.supported()[0]) {
      this.policyModel.set({ defaultLocale: this.supported()[0] });
    }
  }

  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.validationError.set(null);
    const supported = this.supported();
    const defaultLocale = this.policyModel().defaultLocale;
    if (supported.length === 0) {
      this.validationError.set(this.i18n.t('translations.atLeastOneLocale'));
      return;
    }
    if (!supported.includes(defaultLocale)) {
      this.validationError.set(this.i18n.t('translations.defaultMustBeSupported'));
      return;
    }
    const updated = await this.store.save(defaultLocale, supported);
    if (!updated) return;
    const workspaceId = this.workspace.id();
    await this.auth.refreshSession();
    if (workspaceId) await this.workspace.select(workspaceId);
  }
}
