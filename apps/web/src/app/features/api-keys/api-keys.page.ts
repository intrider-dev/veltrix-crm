import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type { ApiKeyScope } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ApiKeysStore } from './api-keys.store';

@Component({
  selector: 'app-api-keys-page',
  imports: [ErrorPanelComponent, FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  providers: [ApiKeysStore],
  template: `<div class="page settings-feature">
    <header class="page-header">
      <div>
        <h1>{{ i18n.t('integrations.apiKeys.title') }}</h1>
        <p>{{ i18n.t('integrations.apiKeys.subtitle') }}</p>
      </div>
    </header>
    @if (!permissions.allows('settings.write')) {
      <div class="error-panel" role="alert">{{ i18n.t('integrations.permission') }}</div>
    } @else {
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      <section class="panel editor">
        <form (submit)="create($event)" novalidate>
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('common.field.name') }}</mat-label
            ><input matInput [formField]="keyForm.name"
          /></mat-form-field>
          <fieldset>
            <legend>{{ i18n.t('integrations.apiKeys.scopes') }}</legend>
            <div class="scope-grid">
              @for (scope of scopes; track scope) {
                <label
                  ><input
                    type="checkbox"
                    [checked]="selectedScopes().includes(scope)"
                    (change)="toggleScope(scope)"
                  />{{ i18n.t(scopeKey(scope)) }}</label
                >
              }
            </div>
          </fieldset>
          <button
            mat-flat-button
            type="submit"
            [disabled]="store.saving() || selectedScopes().length === 0"
          >
            {{ i18n.t('integrations.apiKeys.create') }}
          </button>
        </form>
      </section>
      @if (store.revealedToken(); as token) {
        <section class="secret-panel" role="status">
          <strong>{{ i18n.t('integrations.apiKeys.created') }}</strong>
          <p>{{ i18n.t('integrations.secretOnce') }}</p>
          <code>{{ token }}</code
          ><button mat-button type="button" (click)="store.revealedToken.set(null)">
            {{ i18n.t('common.action.close') }}
          </button>
        </section>
      }
      <section class="panel key-list" [attr.aria-busy]="store.loading()">
        @if (store.loading()) {
          <div class="list-skeleton">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else {
          @for (key of store.keys(); track key.id) {
            <article>
              <div>
                <h2>{{ key.name }}</h2>
                <p>
                  <code>{{ key.prefix }}…</code> · {{ key.scopes.join(', ') }}
                </p>
                <small
                  >{{ i18n.t('integrations.apiKeys.createdAt') }} {{ i18n.date(key.createdAt) }}
                  @if (key.lastUsedAt) {
                    · {{ i18n.t('integrations.apiKeys.lastUsed') }} {{ i18n.date(key.lastUsedAt) }}
                  }
                </small>
              </div>
              <span class="status-pill">{{
                i18n.t(key.revokedAt ? 'integrations.apiKeys.revoked' : 'common.status.active')
              }}</span>
              @if (!key.revokedAt) {
                <button mat-button type="button" (click)="store.revoke(key)">
                  {{ i18n.t('integrations.apiKeys.revoke') }}
                </button>
              }
            </article>
          } @empty {
            <div class="empty-state">{{ i18n.t('integrations.apiKeys.empty') }}</div>
          }
        }
      </section>
    }
  </div>`,
  styles: `
    .settings-feature {
      max-width: 70rem;
    }
    .editor {
      padding: 1rem;
    }
    .editor form {
      display: grid;
      grid-template-columns: minmax(12rem, 1fr) 2fr auto;
      align-items: start;
      gap: 1rem;
    }
    fieldset {
      border: 0;
      margin: 0;
      padding: 0;
    }
    .scope-grid {
      display: grid;
      grid-template-columns: repeat(2, 1fr);
      gap: 0.45rem;
    }
    .scope-grid label {
      display: flex;
      gap: 0.45rem;
      font-size: 0.78rem;
    }
    .secret-panel {
      padding: 1rem;
      border: 1px solid color-mix(in srgb, var(--brand) 30%, transparent);
      border-radius: 0.65rem;
      background: var(--brand-soft);
    }
    .secret-panel p {
      color: var(--text-muted);
    }
    .secret-panel code {
      display: block;
      margin: 0.6rem 0;
      overflow-wrap: anywhere;
      user-select: all;
    }
    .key-list article {
      display: grid;
      grid-template-columns: 1fr auto auto;
      align-items: center;
      gap: 0.75rem;
      padding: 0.9rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .key-list article:last-child {
      border: 0;
    }
    article h2 {
      margin: 0;
      font-size: 0.9rem;
    }
    article p,
    article small {
      display: block;
      margin: 0.2rem 0 0;
      color: var(--text-muted);
      font-size: 0.72rem;
    }
    @media (max-width: 750px) {
      .editor form {
        grid-template-columns: 1fr;
      }
      .key-list article {
        grid-template-columns: 1fr auto;
      }
      .key-list article button {
        grid-column: 1/-1;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ApiKeysPage implements OnInit {
  readonly store = inject(ApiKeysStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly scopes: readonly ApiKeyScope[] = [
    'contacts.read',
    'contacts.write',
    'companies.read',
    'companies.write',
    'deals.read',
    'deals.write',
    'activities.read',
    'activities.write',
    'reports.read',
    'webhooks.write',
  ];
  readonly selectedScopes = signal<readonly ApiKeyScope[]>(['contacts.read']);
  readonly model = signal({ name: '' });
  readonly keyForm = form(this.model, (schema) => required(schema.name));
  ngOnInit(): void {
    if (this.permissions.allows('settings.write')) void this.store.load();
  }
  scopeKey(scope: ApiKeyScope): `integrations.scope.${ApiKeyScope}` {
    return `integrations.scope.${scope}`;
  }
  toggleScope(scope: ApiKeyScope): void {
    this.selectedScopes.update((items) =>
      items.includes(scope) ? items.filter((item) => item !== scope) : [...items, scope],
    );
  }
  async create(event: Event): Promise<void> {
    event.preventDefault();
    if (this.keyForm().invalid() || !this.selectedScopes().length) return;
    await this.store.create(this.model().name.trim(), this.selectedScopes());
    this.model.set({ name: '' });
  }
}
