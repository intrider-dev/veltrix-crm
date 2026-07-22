import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { FormField, form, readonly as readonlyField, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router, RouterLink } from '@angular/router';

import type { Activity, CreateActivity, UpdateCompany } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { AttachmentPanelComponent } from '../../shared/attachments/attachment-panel.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { CompanyDetailsStore } from './company-details.store';

@Component({
  selector: 'app-company-details-page',
  imports: [
    AttachmentPanelComponent,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  providers: [CompanyDetailsStore],
  template: `
    <div class="page details-page">
      <a routerLink="/companies" class="back-link"
        ><app-icon name="back" />{{ i18n.t('common.nav.companies') }}</a
      >
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="load()" />
      }
      @if (store.loading() && !store.company()) {
        <div class="skeleton hero-skeleton"></div>
        <div class="skeleton body-skeleton"></div>
      } @else if (store.company(); as company) {
        <header class="company-hero panel">
          <span class="company-mark" aria-hidden="true">{{ company.name.charAt(0) }}</span>
          <div class="company-title">
            <h1>{{ company.name }}</h1>
            <p>{{ company.domain || company.industry || '—' }}</p>
          </div>
          <div class="hero-actions">
            <span class="status">{{ i18n.t(statusKey(company.status)) }}</span>
            @if (permissions.allows('records.delete')) {
              <button mat-button type="button" class="danger-action" (click)="deleteOpen.set(true)">
                {{ i18n.t('companies.details.deleteConfirm') }}
              </button>
            }
          </div>
        </header>

        @if (deleteOpen()) {
          <section class="destructive-confirm" role="alert">
            <span>{{ i18n.t('companies.details.deletePrompt', { name: company.name }) }}</span>
            <div>
              <button mat-button type="button" (click)="deleteOpen.set(false)">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button
                mat-flat-button
                type="button"
                class="danger-button"
                [disabled]="store.deleting()"
                (click)="deleteCompany()"
              >
                {{ i18n.t('companies.details.deleteConfirm') }}
              </button>
            </div>
          </section>
        }
        @if (store.conflict()) {
          <section class="conflict" role="alert">
            <strong>{{ i18n.t('companies.details.conflict') }}</strong>
            <button mat-button type="button" (click)="load()">
              {{ i18n.t('common.action.retry') }}
            </button>
          </section>
        }

        <div class="details-grid">
          <section class="panel edit-panel">
            <header>
              <div>
                <h2>{{ i18n.t('companies.details.edit') }}</h2>
                <small>{{ i18n.t('companies.details.unsaved') }}</small>
              </div>
            </header>
            <form (submit)="save($event)" novalidate>
              <div class="field-grid">
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
                <label class="select-field">
                  <span>{{ i18n.t('common.field.status') }}</span>
                  <select [formField]="companyForm.status">
                    <option value="active">{{ i18n.t('common.status.active') }}</option>
                    <option value="inactive">{{ i18n.t('common.status.inactive') }}</option>
                  </select>
                </label>
              </div>
              <fieldset>
                <legend>{{ i18n.t('companies.details.address') }}</legend>
                <div class="field-grid address-grid">
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('companies.details.street') }}</mat-label>
                    <input matInput [formField]="companyForm.street" />
                  </mat-form-field>
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('companies.details.city') }}</mat-label>
                    <input matInput [formField]="companyForm.city" />
                  </mat-form-field>
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('companies.details.region') }}</mat-label>
                    <input matInput [formField]="companyForm.region" />
                  </mat-form-field>
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('companies.details.postalCode') }}</mat-label>
                    <input matInput [formField]="companyForm.postalCode" />
                  </mat-form-field>
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('companies.details.country') }}</mat-label>
                    <input matInput [formField]="companyForm.country" />
                  </mat-form-field>
                </div>
              </fieldset>
              @if (saveError()) {
                <div class="form-error" role="alert">{{ saveError() }}</div>
              }
              @if (permissions.allows('records.update')) {
                <div class="actions">
                  <button mat-flat-button type="submit" [disabled]="store.saving()">
                    {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.save') }}
                  </button>
                </div>
              }
            </form>
          </section>

          <section class="panel timeline-panel">
            <header>
              <h2>{{ i18n.t('activities.activity.timeline') }}</h2>
              @if (permissions.allows('records.create')) {
                <button mat-button type="button" (click)="activityOpen.set(!activityOpen())">
                  <app-icon name="add" />{{ i18n.t('activities.activity.add') }}
                </button>
              }
            </header>
            @if (activityOpen()) {
              <form class="activity-form" (submit)="addActivity($event)">
                <label class="select-field">
                  <span>{{ i18n.t('web.activity.type') }}</span>
                  <select [formField]="activityForm.type">
                    <option value="task">{{ i18n.t('activities.activity.task') }}</option>
                    <option value="call">{{ i18n.t('activities.activity.call') }}</option>
                    <option value="meeting">{{ i18n.t('activities.activity.meeting') }}</option>
                    <option value="note">{{ i18n.t('activities.activity.note') }}</option>
                  </select>
                </label>
                <mat-form-field appearance="outline">
                  <mat-label>{{ i18n.t('activities.activity.title') }}</mat-label>
                  <input matInput [formField]="activityForm.title" />
                </mat-form-field>
                <mat-form-field appearance="outline">
                  <mat-label>{{ i18n.t('activities.activity.body') }}</mat-label>
                  <textarea matInput rows="3" [formField]="activityForm.body"></textarea>
                </mat-form-field>
                <button mat-flat-button type="submit">{{ i18n.t('common.action.add') }}</button>
              </form>
            }
            @if (store.activityError()) {
              <div class="form-error" role="alert">{{ activityErrorMessage() }}</div>
            }
            @if (store.activities().length === 0) {
              <div class="empty-state">{{ i18n.t('dashboard.dashboard.emptyActivity') }}</div>
            } @else {
              <ol class="timeline">
                @for (activity of store.activities(); track activity.id) {
                  <li>
                    <span class="dot" aria-hidden="true"></span>
                    <article>
                      <header>
                        <strong>{{ activity.title }}</strong>
                        <span>{{ i18n.t(activityTypeKey(activity.type)) }}</span>
                      </header>
                      @if (activity.body) {
                        <p>{{ activity.body }}</p>
                      }
                      <time [attr.datetime]="activity.occurredAt">{{
                        i18n.date(activity.occurredAt, {
                          dateStyle: 'medium',
                          timeStyle: 'short',
                        })
                      }}</time>
                    </article>
                  </li>
                }
              </ol>
            }
          </section>
        </div>

        <section class="panel duplicates-panel">
          <header>
            <div>
              <h2>{{ i18n.t('companies.details.duplicatesTitle') }}</h2>
              <p>{{ i18n.t('companies.details.duplicatesDescription') }}</p>
            </div>
            <button
              mat-stroked-button
              type="button"
              [disabled]="store.duplicatesLoading()"
              (click)="store.loadDuplicates(id())"
            >
              {{
                i18n.t(
                  store.duplicatesLoaded()
                    ? 'companies.details.duplicatesRefresh'
                    : 'companies.details.duplicatesFind'
                )
              }}
            </button>
          </header>
          @if (store.duplicatesError()) {
            <div class="form-error" role="alert">{{ duplicateErrorMessage() }}</div>
          }
          @if (store.duplicatesLoading() && !store.duplicatesLoaded()) {
            <div class="duplicates-skeleton skeleton"></div>
          } @else if (store.duplicatesLoaded() && store.duplicates().length === 0) {
            <div class="empty-state">{{ i18n.t('companies.details.duplicatesEmpty') }}</div>
          } @else if (store.duplicates().length > 0) {
            <ul class="duplicate-list">
              @for (candidate of store.duplicates(); track candidate.id) {
                <li>
                  <div>
                    <a [routerLink]="['/companies', candidate.id]">{{ candidate.displayName }}</a>
                    <span>{{ candidate.domain || '—' }}</span>
                  </div>
                  <div class="match-summary">
                    <strong>{{ duplicateScore(candidate.score) }}</strong>
                    <span>{{ i18n.t(duplicateReasonKey(candidate.reason)) }}</span>
                  </div>
                  @if (permissions.allows('records.delete')) {
                    @if (mergeConfirmId() === candidate.id) {
                      <div class="merge-confirm" role="alert">
                        <span>{{ i18n.t('companies.details.duplicatesMergeWarning') }}</span>
                        <button mat-button type="button" (click)="mergeConfirmId.set(null)">
                          {{ i18n.t('common.action.cancel') }}
                        </button>
                        <button
                          mat-flat-button
                          type="button"
                          class="danger-button"
                          [disabled]="store.merging() === candidate.id"
                          (click)="mergeDuplicate(candidate.id)"
                        >
                          {{ i18n.t('companies.details.duplicatesMergeConfirm') }}
                        </button>
                      </div>
                    } @else {
                      <button mat-button type="button" (click)="mergeConfirmId.set(candidate.id)">
                        {{ i18n.t('companies.details.duplicatesMerge') }}
                      </button>
                    }
                  }
                </li>
              }
            </ul>
          }
        </section>
        <app-attachment-panel entityType="company" [entityId]="id()" />
      }
    </div>
  `,
  styleUrl: './company-details.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CompanyDetailsPage implements OnInit {
  readonly id = input.required<string>();
  readonly store = inject(CompanyDetailsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly saveError = signal<string | null>(null);
  readonly activityOpen = signal(false);
  readonly deleteOpen = signal(false);
  readonly mergeConfirmId = signal<string | null>(null);
  readonly companyModel = signal({
    name: '',
    domain: '',
    industry: '',
    status: 'active',
    street: '',
    city: '',
    region: '',
    postalCode: '',
    country: '',
  });
  readonly companyForm = form(this.companyModel, (schema) => {
    required(schema.name);
    readonlyField(schema, { when: () => !this.permissions.allows('records.update') });
  });
  readonly activityModel = signal<{
    type: 'task' | 'call' | 'meeting' | 'note';
    title: string;
    body: string;
  }>({ type: 'note', title: '', body: '' });
  readonly activityForm = form(this.activityModel, (schema) => required(schema.title));
  private readonly router = inject(Router);

  ngOnInit(): void {
    void this.load();
  }

  async load(): Promise<void> {
    await this.store.load(this.id());
    const company = this.store.company();
    if (!company) return;
    const address = company.address ?? {};
    this.companyModel.set({
      name: company.name,
      domain: company.domain ?? '',
      industry: company.industry ?? '',
      status: company.status,
      street: address['street'] ?? '',
      city: address['city'] ?? '',
      region: address['region'] ?? '',
      postalCode: address['postalCode'] ?? '',
      country: address['country'] ?? '',
    });
  }

  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!this.permissions.allows('records.update')) return;
    this.saveError.set(null);
    if (this.companyForm().invalid()) {
      this.companyForm().markAsTouched();
      return;
    }
    const current = this.store.company();
    if (!current) return;
    const value = this.companyModel();
    const body: UpdateCompany = {
      name: value.name.trim(),
      domain: value.domain.trim() || null,
      industry: value.industry.trim() || null,
      ownerId: current.ownerId,
      teamId: current.teamId,
      status: value.status,
      address: compactAddress(value),
      customFields: current.customFields ?? {},
    };
    try {
      await this.store.save(this.id(), body);
    } catch (error) {
      this.saveError.set(this.i18n.problem(error instanceof Error ? error.message : 'generic'));
    }
  }

  async addActivity(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!this.permissions.allows('records.create') || this.activityForm().invalid()) {
      this.activityForm().markAsTouched();
      return;
    }
    const value = this.activityModel();
    const body: CreateActivity = {
      type: value.type,
      title: value.title.trim(),
      body: value.body.trim() || null,
      relatedType: 'company',
      relatedId: this.id(),
      priority: 'normal',
    };
    try {
      await this.store.addActivity(body);
      this.activityModel.set({ type: 'note', title: '', body: '' });
      this.activityOpen.set(false);
    } catch {
      // The persistent activity error renders the localized problem.
    }
  }

  async deleteCompany(): Promise<void> {
    try {
      await this.store.deleteCompany(this.id());
      await this.router.navigate(['/companies']);
    } catch (error) {
      this.saveError.set(this.i18n.problem(error instanceof Error ? error.message : 'generic'));
      this.deleteOpen.set(false);
    }
  }

  async mergeDuplicate(candidateId: string): Promise<void> {
    try {
      await this.store.mergeDuplicate(this.id(), candidateId);
      this.mergeConfirmId.set(null);
    } catch {
      // The persistent duplicate error remains visible beside the operation.
    }
  }

  statusKey(status: string): AppMessageKey {
    return status === 'active' ? 'common.status.active' : 'common.status.inactive';
  }

  activityTypeKey(type: Activity['type']): AppMessageKey {
    return `activities.activity.${type}` as AppMessageKey;
  }

  duplicateReasonKey(reason: string): AppMessageKey {
    return reason === 'domain_exact'
      ? 'companies.details.duplicatesReasonDomain'
      : 'companies.details.duplicatesReasonName';
  }

  duplicateScore(score: number): string {
    return new Intl.NumberFormat(this.i18n.locale(), {
      style: 'percent',
      maximumFractionDigits: 0,
    }).format(score);
  }

  activityErrorMessage(): string {
    const error = this.store.activityError();
    return this.i18n.problem(error instanceof Error ? error.message : 'generic');
  }

  duplicateErrorMessage(): string {
    const error = this.store.duplicatesError();
    return this.i18n.problem(error instanceof Error ? error.message : 'generic');
  }
}

function compactAddress(value: {
  readonly street: string;
  readonly city: string;
  readonly region: string;
  readonly postalCode: string;
  readonly country: string;
}): Record<string, string> {
  return Object.fromEntries(
    Object.entries({
      street: value.street.trim(),
      city: value.city.trim(),
      region: value.region.trim(),
      postalCode: value.postalCode.trim(),
      country: value.country.trim(),
    }).filter((entry) => entry[1] !== ''),
  );
}
