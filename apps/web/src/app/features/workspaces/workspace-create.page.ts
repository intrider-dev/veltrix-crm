import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, maxLength, minLength, pattern, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router } from '@angular/router';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import { ApiClient } from '../../core/api/api-client.service';
import type { CreateWorkspaceRequest } from '../../core/api/api.types';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { identityErrorMessage } from '../auth/identity-error';

interface WorkspaceCreateModel {
  readonly name: string;
  readonly slug: string;
  readonly defaultLocale: SupportedLocale;
  readonly timezone: string;
  readonly defaultCurrency: string;
}

const CYRILLIC_SLUG_TRANSLITERATION: Readonly<Record<string, string>> = {
  а: 'a',
  б: 'b',
  в: 'v',
  г: 'g',
  д: 'd',
  е: 'e',
  ё: 'e',
  ж: 'zh',
  з: 'z',
  и: 'i',
  й: 'y',
  к: 'k',
  л: 'l',
  м: 'm',
  н: 'n',
  о: 'o',
  п: 'p',
  р: 'r',
  с: 's',
  т: 't',
  у: 'u',
  ф: 'f',
  х: 'h',
  ц: 'ts',
  ч: 'ch',
  ш: 'sh',
  щ: 'sch',
  ъ: '',
  ы: 'y',
  ь: '',
  э: 'e',
  ю: 'yu',
  я: 'ya',
};

@Component({
  selector: 'app-workspace-create-page',
  imports: [FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  template: `
    <div class="page workspace-create-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('identity.workspace.title') }}</h1>
          <p>{{ i18n.t('identity.workspace.subtitle') }}</p>
        </div>
      </header>

      <section class="panel form-panel" [attr.aria-label]="i18n.t('identity.workspace.title')">
        <form (submit)="submit($event)" novalidate>
          <mat-form-field appearance="outline" class="wide">
            <mat-label>{{ i18n.t('identity.workspace.name') }}</mat-label>
            <input
              matInput
              autocomplete="organization"
              [formField]="workspaceForm.name"
              (input)="suggestSlug($event)"
            />
            @if (workspaceForm.name().touched() && workspaceForm.name().invalid()) {
              <mat-error>{{ i18n.t('identity.validation.workspaceName') }}</mat-error>
            }
          </mat-form-field>

          <mat-form-field appearance="outline" class="wide">
            <mat-label>{{ i18n.t('identity.workspace.slug') }}</mat-label>
            <input
              matInput
              autocomplete="off"
              spellcheck="false"
              [formField]="workspaceForm.slug"
              (input)="slugEdited.set(true)"
            />
            <mat-hint>{{ i18n.t('identity.workspace.slugHint') }}</mat-hint>
            @if (workspaceForm.slug().touched() && workspaceForm.slug().invalid()) {
              <mat-error>{{ i18n.t('identity.validation.slug') }}</mat-error>
            }
          </mat-form-field>

          <label class="native-field">
            <span>{{ i18n.t('identity.workspace.defaultLocale') }}</span>
            <select [formField]="workspaceForm.defaultLocale">
              @for (locale of i18n.supportedLocales; track locale) {
                <option [value]="locale">{{ i18n.languageName(locale) }}</option>
              }
            </select>
          </label>

          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.workspace.timezone') }}</mat-label>
            <input
              matInput
              autocomplete="off"
              spellcheck="false"
              [formField]="workspaceForm.timezone"
            />
            @if (workspaceForm.timezone().touched() && workspaceForm.timezone().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>

          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('identity.workspace.currency') }}</mat-label>
            <input
              matInput
              autocomplete="off"
              autocapitalize="characters"
              spellcheck="false"
              [formField]="workspaceForm.defaultCurrency"
              (input)="normalizeCurrency($event)"
            />
            <mat-hint>{{ i18n.t('identity.workspace.currencyHint') }}</mat-hint>
            @if (
              workspaceForm.defaultCurrency().touched() && workspaceForm.defaultCurrency().invalid()
            ) {
              <mat-error>{{ i18n.t('identity.validation.currency') }}</mat-error>
            }
          </mat-form-field>

          @if (error()) {
            <div class="form-error wide" role="alert">{{ error() }}</div>
          }

          <div class="form-actions wide">
            <button mat-button type="button" [disabled]="pending()" (click)="cancel()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="submit" [disabled]="pending()">
              {{ i18n.t(pending() ? 'identity.workspace.creating' : 'identity.workspace.submit') }}
            </button>
          </div>
        </form>
      </section>
    </div>
  `,
  styles: `
    .workspace-create-page {
      max-width: 48rem;
    }
    .form-panel {
      padding: clamp(1rem, 3vw, 1.5rem);
    }
    form {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.8rem 1rem;
    }
    .wide {
      grid-column: 1 / -1;
    }
    .native-field select {
      min-height: 3.5rem;
      padding-inline: 0.9rem;
      border-radius: 0.25rem;
    }
    .form-error {
      padding: 0.75rem 0.9rem;
      border-radius: 0.5rem;
      color: var(--danger);
      background: var(--danger-surface);
    }
    @media (max-width: 640px) {
      form {
        grid-template-columns: 1fr;
      }
      .wide {
        grid-column: auto;
      }
      .form-actions {
        justify-content: stretch;
      }
      .form-actions > * {
        flex: 1;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkspaceCreatePage {
  readonly i18n = inject(I18nService);
  readonly pending = signal(false);
  readonly error = signal<string | null>(null);
  readonly slugEdited = signal(false);
  readonly model = signal<WorkspaceCreateModel>({
    name: '',
    slug: '',
    defaultLocale: this.i18n.locale(),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    defaultCurrency: 'USD',
  });
  readonly workspaceForm = form(this.model, (schema) => {
    required(schema.name);
    minLength(schema.name, 2);
    maxLength(schema.name, 120);
    required(schema.slug);
    minLength(schema.slug, 2);
    pattern(schema.slug, /^[a-z0-9]+(?:-[a-z0-9]+)*$/);
    maxLength(schema.slug, 63);
    required(schema.defaultLocale);
    required(schema.timezone);
    maxLength(schema.timezone, 100);
    required(schema.defaultCurrency);
    pattern(schema.defaultCurrency, /^[A-Z]{3}$/);
  });

  private readonly api = inject(ApiClient);
  private readonly auth = inject(AuthStore);
  private readonly workspace = inject(WorkspaceStore);
  private readonly router = inject(Router);

  suggestSlug(event: Event): void {
    if (this.slugEdited()) return;
    const name = (event.target as HTMLInputElement).value;
    this.model.update((current) => ({ ...current, slug: this.slugify(name) }));
  }

  normalizeCurrency(event: Event): void {
    const input = event.target as HTMLInputElement;
    const currency = input.value
      .toUpperCase()
      .replace(/[^A-Z]/g, '')
      .slice(0, 3);
    if (currency === input.value) return;
    input.value = currency;
    this.model.update((current) => ({ ...current, defaultCurrency: currency }));
  }

  async submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.error.set(null);
    this.workspaceForm().markAsTouched();
    if (this.workspaceForm().invalid()) return;
    this.pending.set(true);
    try {
      const value = this.model();
      const body: CreateWorkspaceRequest = {
        name: value.name.trim(),
        slug: value.slug.trim(),
        defaultLocale: value.defaultLocale,
        timezone: value.timezone.trim(),
        defaultCurrency: value.defaultCurrency,
      };
      const created = await this.api.createWorkspace(body);
      if (!(await this.auth.refreshSession())) {
        await this.router.navigateByUrl('/login');
        return;
      }
      await this.workspace.select(created.id);
      await this.router.navigateByUrl('/dashboard');
    } catch (error) {
      this.error.set(identityErrorMessage(this.i18n, error));
    } finally {
      this.pending.set(false);
    }
  }

  async cancel(): Promise<void> {
    await this.router.navigateByUrl(this.workspace.id() ? '/settings' : '/dashboard');
  }

  private slugify(value: string): string {
    return value
      .normalize('NFKD')
      .replace(/[\u0300-\u036f]/g, '')
      .toLowerCase()
      .replace(/[а-яё]/g, (character) => CYRILLIC_SLUG_TRANSLITERATION[character] ?? character)
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 63);
  }
}
