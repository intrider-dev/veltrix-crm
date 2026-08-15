import { A11yModule } from '@angular/cdk/a11y';
import type { OnDestroy, OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormField, form, maxLength, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { Router, RouterLink } from '@angular/router';

import { apiErrorMessage } from '../../core/api/api-error-message';
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
    MatSelectModule,
    RouterLink,
  ],
  providers: [CompaniesStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div class="page-heading">
          <div class="title-line">
            <h1>{{ i18n.t('common.nav.companies') }}</h1>
            <span class="count-badge">{{ visibleCount() }}</span>
          </div>
          <p>{{ i18n.t('companies.subtitle') }}</p>
        </div>
        <label class="header-search">
          <span class="visually-hidden">{{ i18n.t('companies.filters.search') }}</span>
          <app-icon name="search" />
          <input
            type="search"
            [placeholder]="i18n.t('companies.filters.search')"
            [value]="store.query()"
            (input)="searchChanged($event)"
          />
          <kbd>Ctrl K</kbd>
        </label>
        @if (permissions.allows('records.create')) {
          <button class="add-company" mat-flat-button type="button" (click)="openCreate()">
            <app-icon name="add" />{{ i18n.t('companies.actions.add') }}
          </button>
        }
      </header>

      <section class="summary-grid" [attr.aria-label]="i18n.t('companies.summary.title')">
        <article class="summary-card summary-card--violet">
          <span class="summary-icon"><app-icon name="building" /></span>
          <div>
            <small>{{ i18n.t('companies.summary.loaded') }}</small
            ><strong>{{ loadedCount() }}</strong>
          </div>
          <span class="sparkline" aria-hidden="true"></span>
        </article>
        <article class="summary-card summary-card--blue">
          <span class="summary-icon"><app-icon name="check" /></span>
          <div>
            <small>{{ i18n.t('companies.summary.active') }}</small
            ><strong>{{ activeCount() }}</strong>
          </div>
          <span class="sparkline" aria-hidden="true"></span>
        </article>
        <article class="summary-card summary-card--green">
          <span class="summary-icon"><app-icon name="file" /></span>
          <div>
            <small>{{ i18n.t('companies.summary.withDomain') }}</small
            ><strong>{{ withDomainCount() }}</strong>
          </div>
          <span class="sparkline" aria-hidden="true"></span>
        </article>
        <article class="summary-card summary-card--orange">
          <span class="summary-icon"><app-icon name="deal" /></span>
          <div>
            <small>{{ i18n.t('companies.summary.withIndustry') }}</small
            ><strong>{{ withIndustryCount() }}</strong>
          </div>
          <span class="sparkline" aria-hidden="true"></span>
        </article>
      </section>

      <div class="company-workspace">
        <section
          class="panel filter-toolbar company-toolbar"
          [attr.aria-label]="i18n.t('companies.filters.search')"
        >
          <div
            class="mode-switch segmented-control"
            role="group"
            [attr.aria-label]="i18n.t('common.nav.companies')"
          >
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
            <label class="filter-field">
              <span>{{ i18n.t('common.field.status') }}</span>
              <mat-select
                panelClass="crm-select-panel"
                class="crm-select"
                [aria-label]="i18n.t('common.field.status')"
                [value]="store.status()"
                (selectionChange)="statusChanged($event.value)"
              >
                <mat-option value="all">{{ i18n.t('companies.filters.allStatuses') }}</mat-option>
                <mat-option value="active">{{ i18n.t('common.status.active') }}</mat-option>
                <mat-option value="inactive">{{ i18n.t('common.status.inactive') }}</mat-option>
              </mat-select>
            </label>
            <label class="filter-field saved-view-select">
              <span>{{ i18n.t('companies.savedViews.title') }}</span>
              <mat-select
                panelClass="crm-select-panel"
                class="crm-select"
                [aria-label]="i18n.t('companies.savedViews.title')"
                [value]="selectedViewId()"
                (selectionChange)="savedViewChanged($event.value)"
              >
                <mat-option value="">{{ i18n.t('companies.savedViews.current') }}</mat-option>
                @for (view of store.savedViews(); track view.id) {
                  <mat-option [value]="view.id">{{ view.name }}</mat-option>
                }
              </mat-select>
            </label>
            @if (permissions.allows('records.update')) {
              <button mat-stroked-button type="button" (click)="saveViewOpen.set(true)">
                {{ i18n.t('companies.savedViews.save') }}
              </button>
              @if (selectedView()) {
                <button
                  mat-button
                  type="button"
                  class="danger-action"
                  (click)="deleteSelectedView()"
                >
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
          @if (store.mode() === 'companies' && store.companies().length > 0) {
            <div class="company-columns" aria-hidden="true">
              <span></span>
              <span>{{ i18n.t('companies.columns.company') }}</span>
              <span>{{ i18n.t('companies.columns.industry') }}</span>
              <span>{{ i18n.t('companies.columns.domain') }}</span>
              <span>{{ i18n.t('common.field.status') }}</span>
              <span>{{ i18n.t('companies.columns.created') }}</span>
              <span></span>
            </div>
          }
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
                  <span class="company-cell">{{ company.industry || '—' }}</span>
                  <span class="company-cell company-domain">{{ company.domain || '—' }}</span>
                  <span class="company-status">{{ i18n.t(statusKey(company.status)) }}</span>
                  <time class="company-cell" [attr.datetime]="company.createdAt">{{
                    i18n.date(company.createdAt)
                  }}</time>
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
        @if (store.mode() === 'companies' && previewCompany(); as company) {
          <aside class="company-preview" [attr.aria-label]="i18n.t('companies.preview.title')">
            <header>
              <span class="company-mark" aria-hidden="true">{{ company.name.charAt(0) }}</span>
              <div>
                <h2>{{ company.name }}</h2>
                <span class="company-status">{{ i18n.t(statusKey(company.status)) }}</span>
              </div>
            </header>
            <dl>
              <div>
                <dt>{{ i18n.t('companies.columns.domain') }}</dt>
                <dd>{{ company.domain || '—' }}</dd>
              </div>
              <div>
                <dt>{{ i18n.t('companies.columns.industry') }}</dt>
                <dd>{{ company.industry || '—' }}</dd>
              </div>
              <div>
                <dt>{{ i18n.t('companies.columns.created') }}</dt>
                <dd>{{ i18n.date(company.createdAt) }}</dd>
              </div>
              <div>
                <dt>{{ i18n.t('companies.preview.updated') }}</dt>
                <dd>{{ i18n.date(company.updatedAt) }}</dd>
              </div>
            </dl>
            <a mat-stroked-button [routerLink]="['/companies', company.id]"
              >{{ i18n.t('companies.preview.open') }}<app-icon name="chevron"
            /></a>
          </aside>
        }
      </div>
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
        (click)="closeCreate()"
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
            (click)="closeCreate()"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>
        <form (submit)="create($event)" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
            <input
              matInput
              [formField]="companyForm.name"
              [attr.aria-invalid]="createAttempted() && companyForm.name().invalid()"
              [attr.aria-describedby]="
                createAttempted() && companyForm.name().invalid() ? 'company-name-error' : null
              "
            />
          </mat-form-field>
          @if (createAttempted() && companyForm.name().invalid()) {
            <p class="field-error" id="company-name-error" role="alert">
              {{ i18n.t('common.validation.name') }}
            </p>
          }
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('web.company.domain') }}</mat-label>
            <input
              matInput
              inputmode="url"
              [formField]="companyForm.domain"
              [attr.aria-invalid]="createAttempted() && companyForm.domain().invalid()"
              [attr.aria-describedby]="
                createAttempted() && companyForm.domain().invalid() ? 'company-domain-error' : null
              "
            />
          </mat-form-field>
          @if (createAttempted() && companyForm.domain().invalid()) {
            <p class="field-error" id="company-domain-error" role="alert">
              {{ i18n.t('companies.validation.domainLength') }}
            </p>
          }
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('web.company.industry') }}</mat-label>
            <input
              matInput
              [formField]="companyForm.industry"
              [attr.aria-invalid]="createAttempted() && companyForm.industry().invalid()"
              [attr.aria-describedby]="
                createAttempted() && companyForm.industry().invalid()
                  ? 'company-industry-error'
                  : null
              "
            />
          </mat-form-field>
          @if (createAttempted() && companyForm.industry().invalid()) {
            <p class="field-error" id="company-industry-error" role="alert">
              {{ i18n.t('companies.validation.industryLength') }}
            </p>
          }
          @if (createError()) {
            <div class="form-error" role="alert">{{ createError() }}</div>
          }
          <div class="drawer-actions">
            <button mat-button type="button" (click)="closeCreate()">
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
  readonly createAttempted = signal(false);
  readonly saveViewOpen = signal(false);
  readonly selectedViewId = signal('');
  readonly loadedCount = computed(() => this.store.companies().length);
  readonly activeCount = computed(
    () => this.store.companies().filter((company) => company.status === 'active').length,
  );
  readonly withDomainCount = computed(
    () => this.store.companies().filter((company) => Boolean(company.domain)).length,
  );
  readonly withIndustryCount = computed(
    () => this.store.companies().filter((company) => Boolean(company.industry)).length,
  );
  readonly previewCompany = computed(() =>
    this.store.mode() === 'companies' ? (this.store.companies()[0] ?? null) : null,
  );
  readonly model = signal({ name: '', domain: '', industry: '' });
  readonly companyForm = form(this.model, (schema) => {
    required(schema.name);
    maxLength(schema.name, 200);
    maxLength(schema.domain, 253);
    maxLength(schema.industry, 120);
  });
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

  statusChanged(value: string): void {
    if (value === 'all' || value === 'active' || value === 'inactive') {
      void this.store.setStatus(value);
    }
  }

  savedViewChanged(id: string): void {
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

  openCreate(): void {
    this.createError.set(null);
    this.createAttempted.set(false);
    this.createOpen.set(true);
  }

  closeCreate(): void {
    this.createOpen.set(false);
    this.createError.set(null);
    this.createAttempted.set(false);
  }

  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.createError.set(null);
    this.createAttempted.set(true);
    this.model.update((value) => ({
      name: value.name.trim(),
      domain: value.domain.trim(),
      industry: value.industry.trim(),
    }));
    if (this.companyForm().invalid()) {
      this.companyForm.name().markAsTouched();
      this.companyForm.domain().markAsTouched();
      this.companyForm.industry().markAsTouched();
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
      this.closeCreate();
      await this.router.navigate(['/companies', company.id]);
    } catch (error) {
      this.createError.set(apiErrorMessage(this.i18n, error, 'validation'));
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
