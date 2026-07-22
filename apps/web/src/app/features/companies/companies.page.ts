import { A11yModule } from '@angular/cdk/a11y';
import type { OnDestroy, OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router, RouterLink } from '@angular/router';

import type { CreateCompany, SavedView } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { CompaniesStore } from './companies.store';

@Component({
  selector: 'app-companies-page',
  imports: [
    A11yModule,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  providers: [CompaniesStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.companies') }}</h1>
          <p>{{ i18n.plural('common.resultCount', visibleCount()) }}</p>
        </div>
        @if (permissions.allows('records.create')) {
          <button mat-flat-button type="button" (click)="createOpen.set(true)">
            <app-icon name="add" />{{ i18n.t('common.action.add') }}
          </button>
        }
      </header>

      <section class="panel toolbar" [attr.aria-label]="i18n.t('companies.filters.search')">
        <div class="mode-switch" role="group" [attr.aria-label]="i18n.t('common.nav.companies')">
          <button
            mat-button
            type="button"
            [class.selected]="store.mode() === 'companies'"
            (click)="store.setMode('companies')"
          >
            {{ i18n.t('companies.actions.activeList') }}
          </button>
          <button
            mat-button
            type="button"
            [class.selected]="store.mode() === 'trash'"
            (click)="store.setMode('trash')"
          >
            {{ i18n.t('companies.actions.trashList') }}
          </button>
        </div>

        @if (store.mode() === 'companies') {
          <mat-form-field appearance="outline" class="search-field" subscriptSizing="dynamic">
            <mat-label>{{ i18n.t('companies.filters.search') }}</mat-label>
            <app-icon matPrefix name="search" />
            <input matInput type="search" [value]="store.query()" (input)="searchChanged($event)" />
          </mat-form-field>
          <label class="compact-select">
            <span>{{ i18n.t('common.field.status') }}</span>
            <select [value]="store.status()" (change)="statusChanged($event)">
              <option value="all">{{ i18n.t('companies.filters.allStatuses') }}</option>
              <option value="active">{{ i18n.t('common.status.active') }}</option>
              <option value="inactive">{{ i18n.t('common.status.inactive') }}</option>
            </select>
          </label>
          <label class="compact-select saved-view-select">
            <span>{{ i18n.t('companies.savedViews.title') }}</span>
            <select [value]="selectedViewId()" (change)="savedViewChanged($event)">
              <option value="">{{ i18n.t('companies.savedViews.current') }}</option>
              @for (view of store.savedViews(); track view.id) {
                <option [value]="view.id">{{ view.name }}</option>
              }
            </select>
          </label>
          @if (permissions.allows('records.update')) {
            <button mat-stroked-button type="button" (click)="saveViewOpen.set(true)">
              {{ i18n.t('companies.savedViews.save') }}
            </button>
            @if (selectedView()) {
              <button mat-button type="button" class="danger-action" (click)="deleteSelectedView()">
                {{ i18n.t('companies.savedViews.delete') }}
              </button>
            }
          }
        }
      </section>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load(true)" />
      }
      @if (store.operationError()) {
        <div class="form-error" role="alert">{{ operationErrorMessage() }}</div>
      }

      <section class="panel company-list" [attr.aria-busy]="store.loading()">
        @if (store.loading() && visibleCount() === 0) {
          <div class="list-skeleton">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else if (store.mode() === 'companies') {
          @if (store.companies().length === 0) {
            <div class="empty-state">{{ i18n.t('companies.empty.active') }}</div>
          } @else {
            @for (company of store.companies(); track company.id) {
              <a [routerLink]="['/companies', company.id]">
                <span class="company-mark" aria-hidden="true">{{ company.name.charAt(0) }}</span>
                <span class="company-summary">
                  <strong>{{ company.name }}</strong>
                  <small>{{ company.domain || company.industry || '—' }}</small>
                </span>
                <span class="company-status">{{ i18n.t(statusKey(company.status)) }}</span>
                <app-icon name="chevron" />
              </a>
            }
            @if (store.nextCursor()) {
              <div class="load-more">
                <button
                  mat-stroked-button
                  type="button"
                  [disabled]="store.loading()"
                  (click)="store.load(false)"
                >
                  {{ i18n.t('companies.list.loadMore') }}
                </button>
              </div>
            }
          }
        } @else {
          @if (store.trash().length === 0) {
            <div class="empty-state">{{ i18n.t('companies.empty.trash') }}</div>
          } @else {
            @for (company of store.trash(); track company.id) {
              <article class="trash-row">
                <span class="company-mark" aria-hidden="true">{{
                  company.displayName.charAt(0)
                }}</span>
                <span class="company-summary">
                  <strong>{{ company.displayName }}</strong>
                  <small>{{ company.domain || '—' }}</small>
                </span>
                <time [attr.datetime]="company.deletedAt">
                  {{ i18n.t('companies.trash.deletedAt') }} · {{ i18n.date(company.deletedAt) }}
                </time>
                @if (permissions.allows('records.update')) {
                  <button
                    mat-stroked-button
                    type="button"
                    [disabled]="store.operationPending()"
                    (click)="store.restore(company)"
                  >
                    {{ i18n.t('companies.trash.restore') }}
                  </button>
                }
              </article>
            }
            @if (store.trashNextCursor()) {
              <div class="load-more">
                <button
                  mat-stroked-button
                  type="button"
                  [disabled]="store.loading()"
                  (click)="store.load(false)"
                >
                  {{ i18n.t('companies.trash.loadMore') }}
                </button>
              </div>
            }
          }
        }
      </section>
    </div>

    @if (saveViewOpen()) {
      <button
        class="drawer-scrim"
        type="button"
        (click)="closeSaveView()"
        [attr.aria-label]="i18n.t('common.action.close')"
      ></button>
      <aside
        class="small-dialog"
        role="dialog"
        aria-modal="true"
        cdkTrapFocus
        [cdkTrapFocusAutoCapture]="true"
        [attr.aria-labelledby]="'save-company-view-title'"
      >
        <h2 id="save-company-view-title">{{ i18n.t('companies.savedViews.save') }}</h2>
        <form (submit)="saveCurrentView($event)">
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('companies.savedViews.name') }}</mat-label>
            <input matInput [formField]="viewForm.name" />
          </mat-form-field>
          <div class="drawer-actions">
            <button mat-button type="button" (click)="closeSaveView()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="submit" [disabled]="store.operationPending()">
              {{ i18n.t('common.action.save') }}
            </button>
          </div>
        </form>
      </aside>
    }

    @if (createOpen()) {
      <button
        class="drawer-scrim"
        type="button"
        (click)="createOpen.set(false)"
        [attr.aria-label]="i18n.t('common.action.close')"
      ></button>
      <aside
        class="create-drawer"
        role="dialog"
        aria-modal="true"
        cdkTrapFocus
        [cdkTrapFocusAutoCapture]="true"
        [attr.aria-labelledby]="'new-company-title'"
      >
        <header>
          <h2 id="new-company-title">{{ i18n.t('web.company.createTitle') }}</h2>
          <button
            mat-icon-button
            type="button"
            (click)="createOpen.set(false)"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>
        <form (submit)="create($event)">
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
            <input matInput [formField]="companyForm.name" />
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('web.company.domain') }}</mat-label>
            <input matInput inputmode="url" [formField]="companyForm.domain" />
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('web.company.industry') }}</mat-label>
            <input matInput [formField]="companyForm.industry" />
          </mat-form-field>
          @if (createError()) {
            <div class="form-error" role="alert">{{ createError() }}</div>
          }
          <div class="drawer-actions">
            <button mat-button type="button" (click)="createOpen.set(false)">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="submit" [disabled]="store.creating()">
              {{ i18n.t(store.creating() ? 'web.form.saving' : 'common.action.create') }}
            </button>
          </div>
        </form>
      </aside>
    }
  `,
  styleUrl: './companies.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CompaniesPage implements OnInit, OnDestroy {
  readonly store = inject(CompaniesStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly createError = signal<string | null>(null);
  readonly saveViewOpen = signal(false);
  readonly selectedViewId = signal('');
  readonly model = signal({ name: '', domain: '', industry: '' });
  readonly companyForm = form(this.model, (schema) => required(schema.name));
  readonly viewModel = signal({ name: '' });
  readonly viewForm = form(this.viewModel, (schema) => required(schema.name));
  private readonly router = inject(Router);
  private searchTimer: ReturnType<typeof setTimeout> | null = null;

  ngOnInit(): void {
    void this.store.initialize();
  }

  ngOnDestroy(): void {
    if (this.searchTimer !== null) clearTimeout(this.searchTimer);
  }

  visibleCount(): number {
    return this.store.mode() === 'trash'
      ? this.store.trash().length
      : this.store.companies().length;
  }

  selectedView(): SavedView | null {
    return this.store.savedViews().find((view) => view.id === this.selectedViewId()) ?? null;
  }

  searchChanged(event: Event): void {
    this.store.query.set((event.target as HTMLInputElement).value);
    if (this.searchTimer !== null) clearTimeout(this.searchTimer);
    this.searchTimer = setTimeout(() => {
      this.searchTimer = null;
      void this.store.load(true);
    }, 150);
  }

  statusChanged(event: Event): void {
    const value = (event.target as HTMLSelectElement).value;
    if (value === 'all' || value === 'active' || value === 'inactive') {
      void this.store.setStatus(value);
    }
  }

  savedViewChanged(event: Event): void {
    const id = (event.target as HTMLSelectElement).value;
    this.selectedViewId.set(id);
    const view = this.selectedView();
    if (view) void this.store.applySavedView(view);
  }

  async saveCurrentView(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.viewForm().invalid()) {
      this.viewForm().markAsTouched();
      return;
    }
    try {
      const view = await this.store.saveCurrentView(this.viewModel().name);
      this.selectedViewId.set(view.id);
      this.closeSaveView();
    } catch {
      // The persistent operation panel shows the localized API problem.
    }
  }

  async deleteSelectedView(): Promise<void> {
    const view = this.selectedView();
    if (!view) return;
    try {
      await this.store.deleteSavedView(view);
      this.selectedViewId.set('');
    } catch {
      // The persistent operation panel shows the localized API problem.
    }
  }

  closeSaveView(): void {
    this.saveViewOpen.set(false);
    this.viewModel.set({ name: '' });
  }

  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.createError.set(null);
    if (this.companyForm().invalid()) {
      this.companyForm().markAsTouched();
      return;
    }
    const value = this.model();
    const body: CreateCompany = {
      name: value.name.trim(),
      domain: value.domain.trim() || null,
      industry: value.industry.trim() || null,
    };
    try {
      const company = await this.store.create(body);
      this.createOpen.set(false);
      await this.router.navigate(['/companies', company.id]);
    } catch (error) {
      this.createError.set(
        this.i18n.problem(error instanceof Error ? error.message : 'validation'),
      );
    }
  }

  statusKey(status: string): AppMessageKey {
    return status === 'active' ? 'common.status.active' : 'common.status.inactive';
  }

  operationErrorMessage(): string {
    const error = this.store.operationError();
    return this.i18n.problem(error instanceof Error ? error.message : 'generic');
  }
}
