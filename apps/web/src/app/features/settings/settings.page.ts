import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import { AuthStore } from '../../core/auth/auth.store';
import { Permissions } from '../../core/auth/permissions';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import {
  AppearanceStore,
  type DensityPreference,
  type ThemePreference,
} from '../../core/preferences/appearance.store';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { IconComponent } from '../../shared/icon/icon.component';

@Component({
  selector: 'app-settings-page',
  imports: [IconComponent, MatButtonModule, RouterLink],
  template: `
    <div class="page settings-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('settings.settings.title') }}</h1>
        </div>
      </header>
      @if (message()) {
        <div
          class="settings-message"
          [class.error]="messageError()"
          role="status"
          aria-live="polite"
        >
          {{ message() }}
        </div>
      }
      <section class="panel setting-section">
        <header>
          <div>
            <h2>{{ i18n.t('settings.settings.language') }}</h2>
            <p>{{ i18n.t('settings.language.description') }}</p>
          </div>
        </header>
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('settings.language.label') }}</strong
            ><small
              >{{ i18n.t('settings.workspace.defaultLanguage') }}: {{ workspaceLocale() }}</small
            >
          </div>
          <div
            class="segmented segmented-control"
            role="group"
            [attr.aria-label]="i18n.t('settings.language.label')"
          >
            @for (locale of i18n.supportedLocales; track locale) {
              <button
                mat-button
                type="button"
                [class.active]="i18n.locale() === locale"
                [disabled]="saving()"
                (click)="setLocale(locale)"
              >
                {{ i18n.languageName(locale) }}
              </button>
            }
          </div>
        </div>
        @if (workspace.active(); as workspace) {
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.workspace.timezone') }}</strong>
            </div>
            <span>{{ workspace.timezone }}</span>
          </div>
        }
      </section>

      <section class="panel setting-section">
        <header>
          <div>
            <h2>{{ i18n.t('settings.settings.appearance') }}</h2>
          </div>
        </header>
        <div class="setting-row">
          <strong>{{ i18n.t('settings.settings.appearance') }}</strong>
          <div class="choice-grid">
            @for (theme of themes; track theme.value) {
              <button
                type="button"
                [class.active]="appearance.theme() === theme.value"
                (click)="appearance.setTheme(theme.value)"
              >
                <app-icon [name]="theme.value === 'dark' ? 'moon' : 'sun'" /><span>{{
                  i18n.t(theme.key)
                }}</span>
              </button>
            }
          </div>
        </div>
        <div class="setting-row">
          <strong>{{ i18n.t('settings.appearance.comfortable') }}</strong>
          <div
            class="segmented segmented-control"
            role="group"
            [attr.aria-label]="i18n.t('settings.appearance.comfortable')"
          >
            @for (density of densities; track density.value) {
              <button
                mat-button
                type="button"
                [class.active]="appearance.density() === density.value"
                (click)="appearance.setDensity(density.value)"
              >
                {{ i18n.t(density.key) }}
              </button>
            }
          </div>
        </div>
      </section>

      <section class="panel setting-section">
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('settings.settings.security') }}</strong>
          </div>
          <a mat-stroked-button routerLink="/settings/security">{{
            i18n.t('common.action.viewAll')
          }}</a>
        </div>
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('settings.settings.newWorkspace') }}</strong>
          </div>
          <a mat-stroked-button routerLink="/workspace/new">{{
            i18n.t('common.action.viewAll')
          }}</a>
        </div>
        @if (permissions.allows('members.read')) {
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.settings.members') }}</strong>
            </div>
            <a mat-stroked-button routerLink="/settings/members">{{
              i18n.t('common.action.viewAll')
            }}</a>
          </div>
        }
        @if (permissions.allows('roles.write')) {
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.settings.roles') }}</strong>
            </div>
            <a mat-stroked-button routerLink="/settings/roles">{{
              i18n.t('common.action.viewAll')
            }}</a>
          </div>
        }
        @if (permissions.allows('lead_stages.manage')) {
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.settings.leadStages') }}</strong>
            </div>
            <a mat-stroked-button routerLink="/settings/lead-stages">{{
              i18n.t('common.action.viewAll')
            }}</a>
          </div>
        }
        @if (permissions.allows('deal_stages.manage')) {
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.settings.pipelines') }}</strong>
            </div>
            <a mat-stroked-button routerLink="/settings/pipelines">{{
              i18n.t('common.action.viewAll')
            }}</a>
          </div>
        }
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('settings.settings.customFields') }}</strong>
          </div>
          <a mat-stroked-button routerLink="/settings/custom-fields">{{
            i18n.t('common.action.viewAll')
          }}</a>
        </div>
        @if (permissions.allows('settings.write')) {
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.settings.apiKeys') }}</strong>
            </div>
            <a mat-stroked-button routerLink="/settings/api">{{
              i18n.t('common.action.viewAll')
            }}</a>
          </div>
          <div class="setting-row">
            <div>
              <strong>{{ i18n.t('settings.settings.webhooks') }}</strong>
            </div>
            <a mat-stroked-button routerLink="/settings/webhooks">{{
              i18n.t('common.action.viewAll')
            }}</a>
          </div>
        }
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('web.audit.title') }}</strong>
          </div>
          <a mat-stroked-button routerLink="/settings/audit">{{
            i18n.t('common.action.viewAll')
          }}</a>
        </div>
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('settings.settings.localization') }}</strong>
          </div>
          <a mat-stroked-button routerLink="/settings/localization">{{
            i18n.t('common.action.viewAll')
          }}</a>
        </div>
        <div class="setting-row">
          <div>
            <strong>{{ i18n.t('settings.settings.translations') }}</strong>
          </div>
          <a mat-stroked-button routerLink="/settings/translations">{{
            i18n.t('common.action.viewAll')
          }}</a>
        </div>
      </section>
    </div>
  `,
  styles: `
    .settings-page {
      max-width: 58rem;
    }
    .setting-section {
      overflow: hidden;
    }
    .setting-section > header {
      padding: 1rem 1.15rem;
      border-bottom: 1px solid var(--border);
    }
    .setting-section h2 {
      margin: 0;
      font-size: 1rem;
    }
    .setting-section header p {
      margin: 0.35rem 0 0;
      color: var(--text-muted);
      font-size: 0.82rem;
    }
    .setting-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 2rem;
      min-height: 4.5rem;
      padding: 0.8rem 1.15rem;
      border-bottom: 1px solid var(--border);
    }
    .setting-row:last-child {
      border: 0;
    }
    .setting-row strong,
    .setting-row small {
      display: block;
    }
    .setting-row small {
      margin-top: 0.25rem;
      color: var(--text-muted);
    }
    .segmented {
      display: flex;
      gap: 0.2rem;
      padding: 0.2rem;
      border-radius: 0.55rem;
      background: var(--surface-subtle);
    }
    .segmented .active {
      color: var(--brand);
      background: var(--surface-raised);
    }
    .choice-grid {
      display: grid;
      grid-template-columns: repeat(3, 6rem);
      gap: 0.5rem;
    }
    .choice-grid button {
      display: grid;
      place-items: center;
      gap: 0.4rem;
      min-height: 4.5rem;
      border: 1px solid var(--border);
      border-radius: 0.55rem;
      color: var(--text-muted);
      background: var(--surface-raised);
      cursor: pointer;
      transition:
        transform 120ms var(--ease-out),
        border-color 140ms var(--ease-out);
    }
    .choice-grid button:active {
      transform: scale(0.97);
    }
    .choice-grid button.active {
      border-color: var(--brand);
      color: var(--brand);
      background: var(--brand-soft);
    }
    .settings-message {
      padding: 0.75rem 1rem;
      border-radius: 0.55rem;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .settings-message.error {
      color: var(--danger);
      background: var(--danger-surface);
    }
    @media (max-width: 650px) {
      .setting-row {
        align-items: stretch;
        flex-direction: column;
        gap: 0.75rem;
      }
      .choice-grid {
        width: 100%;
        grid-template-columns: repeat(3, 1fr);
      }
      .segmented {
        align-self: stretch;
      }
      .segmented button {
        flex: 1;
      }
    }

    :host {
      --workspace-surface: var(--color-surface, var(--surface-raised));
      --workspace-subtle: var(--color-surface-subtle, var(--surface-subtle));
      --workspace-hover: var(--color-surface-hover, var(--surface-selected));
      --workspace-border: var(--color-border, var(--border));
      --workspace-anchor: var(--color-anchor, var(--brand));
    }
    .settings-page {
      max-width: 68rem;
    }
    .page-header {
      margin-bottom: 0.25rem;
    }
    .setting-section {
      padding: 0;
      border-radius: var(--radius-panel, 0.875rem);
      background: var(--workspace-surface);
    }
    .setting-section > header {
      min-height: 4rem;
      padding: 1rem 1.25rem;
      border-color: var(--workspace-border);
      background: var(--workspace-subtle);
    }
    .setting-section h2 {
      font-size: 1rem;
      font-weight: 650;
    }
    .setting-section header p {
      max-width: 48rem;
      line-height: 1.45;
    }
    .setting-row {
      min-height: 4.75rem;
      padding: 0.875rem 1.25rem;
      border-color: color-mix(in srgb, var(--workspace-border) 72%, transparent);
    }
    .setting-row:hover {
      background: var(--workspace-hover);
    }
    .setting-row > div:first-child {
      min-width: 0;
    }
    .setting-row strong {
      line-height: 1.4;
    }
    .segmented {
      flex-wrap: wrap;
      border-radius: var(--radius-control, 0.625rem);
      background: var(--workspace-subtle);
    }
    .segmented .active {
      color: var(--workspace-anchor);
      background: var(--workspace-surface);
      box-shadow: 0 1px 2px rgb(18 36 29 / 10%);
    }
    .choice-grid {
      grid-template-columns: repeat(3, minmax(5.5rem, 7rem));
    }
    .choice-grid button {
      min-height: 5rem;
      padding: 0.75rem;
      border-color: var(--workspace-border);
      border-radius: var(--radius-control, 0.625rem);
      background: var(--workspace-surface);
      transition:
        transform 120ms cubic-bezier(0.23, 1, 0.32, 1),
        border-color 140ms ease;
    }
    .choice-grid button.active {
      border-color: var(--workspace-anchor);
      color: var(--workspace-anchor);
      background: var(--color-signal-soft, var(--brand-soft));
    }
    .settings-message {
      padding: 0.875rem 1rem;
      border: 1px solid color-mix(in srgb, var(--workspace-anchor) 30%, var(--workspace-border));
      border-radius: var(--radius-control, 0.625rem);
    }
    @media (max-width: 650px) {
      .setting-section > header,
      .setting-row {
        padding-inline: 1rem;
      }
      .setting-row {
        gap: 1rem;
      }
      .setting-row > a,
      .setting-row > .segmented,
      .setting-row > .choice-grid {
        width: 100%;
      }
      .setting-row > a {
        justify-content: center;
      }
      .choice-grid {
        grid-template-columns: repeat(3, minmax(0, 1fr));
      }
    }
    @media (max-width: 420px) {
      .choice-grid {
        grid-template-columns: 1fr;
      }
    }
    @media (prefers-reduced-motion: reduce) {
      .choice-grid button {
        transition: none;
      }
      .choice-grid button:active {
        transform: none;
      }
    }
    @media (forced-colors: active) {
      .segmented .active,
      .choice-grid button.active {
        border: 1px solid CanvasText;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SettingsPage {
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  readonly appearance = inject(AppearanceStore);
  readonly permissions = inject(Permissions);
  readonly workspace = inject(WorkspaceStore);
  readonly saving = signal(false);
  readonly message = signal<string | null>(null);
  readonly messageError = signal(false);
  readonly themes: readonly { value: ThemePreference; key: AppMessageKey }[] = [
    { value: 'light', key: 'settings.appearance.light' },
    { value: 'dark', key: 'settings.appearance.dark' },
    { value: 'system', key: 'settings.appearance.system' },
  ];
  readonly densities: readonly { value: DensityPreference; key: AppMessageKey }[] = [
    { value: 'comfortable', key: 'settings.appearance.comfortable' },
    { value: 'compact', key: 'settings.appearance.compact' },
  ];
  readonly workspaceLocale = () => {
    const locale = this.workspace.active()?.defaultLocale;
    return this.i18n.languageName(locale ?? 'en');
  };
  async setLocale(locale: SupportedLocale): Promise<void> {
    this.saving.set(true);
    this.message.set(null);
    try {
      await this.auth.updateLocale(locale);
      this.messageError.set(false);
      this.message.set(this.i18n.t('settings.language.saved'));
    } catch (error) {
      this.messageError.set(true);
      this.message.set(this.i18n.problem(error instanceof Error ? error.message : 'network'));
    } finally {
      this.saving.set(false);
    }
  }
}
