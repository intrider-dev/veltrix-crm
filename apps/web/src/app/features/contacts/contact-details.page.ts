import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import {
  FormField,
  email,
  form,
  readonly as readonlyField,
  required,
} from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router, RouterLink } from '@angular/router';

import type { Activity, CreateActivity, UpdateContact } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { AttachmentPanelComponent } from '../../shared/attachments/attachment-panel.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ContactDetailsStore } from './contact-details.store';

@Component({
  selector: 'app-contact-details-page',
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
  providers: [ContactDetailsStore],
  template: `
    <div class="page details-page">
      <a mat-button routerLink="/contacts" class="back-control"
        ><app-icon name="back" />{{ i18n.t('web.contact.back') }}</a
      >
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="load()" />
      }
      @if (store.loading() && !store.contact()) {
        <div class="skeleton hero-skeleton"></div>
        <div class="skeleton body-skeleton"></div>
      } @else if (store.contact(); as contact) {
        <header class="contact-hero panel">
          <div class="contact-avatar" aria-hidden="true">
            {{ contact.firstName.charAt(0) }}{{ contact.lastName.charAt(0) }}
          </div>
          <div>
            <h1>{{ contact.displayName }}</h1>
            <p>
              {{ contact.jobTitle }}
              @if (contact.companyName) {
                · <a [routerLink]="['/companies', contact.companyId]">{{ contact.companyName }}</a>
              }
            </p>
          </div>
          <div class="hero-actions">
            <span class="status">{{
              i18n.t(
                contact.status === 'active' ? 'common.status.active' : 'common.status.inactive'
              )
            }}</span>
            @if (permissions.allows('records.delete')) {
              <button
                mat-button
                type="button"
                class="danger-action"
                (click)="deleteConfirm.set(true)"
              >
                {{ i18n.t('contacts.trash.moveAction') }}
              </button>
            }
          </div>
        </header>

        @if (deleteConfirm()) {
          <section class="destructive-confirm" role="alert">
            <span>{{ i18n.t('contacts.trash.moveConfirm', { name: contact.displayName }) }}</span>
            <div>
              <button mat-button type="button" (click)="deleteConfirm.set(false)">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button
                mat-flat-button
                type="button"
                class="danger-button"
                [disabled]="store.deleting()"
                (click)="deleteContact()"
              >
                {{ i18n.t('contacts.trash.confirmMove') }}
              </button>
            </div>
          </section>
        }
        @if (deleteError()) {
          <section class="conflict" role="alert">{{ deleteError() }}</section>
        }

        @if (store.conflict()) {
          <section class="conflict" role="alert">
            <strong>{{ i18n.t('contacts.problem.contact.conflict') }}</strong
            ><button mat-button type="button" (click)="load()">
              {{ i18n.t('common.action.retry') }}
            </button>
          </section>
        }

        <div class="details-grid">
          <section class="panel edit-panel">
            <header>
              <h2>{{ i18n.t('common.action.edit') }}</h2>
              <small>{{ i18n.t('web.contact.unsaved') }}</small>
            </header>
            <form (submit)="save($event)" novalidate>
              <div class="field-grid">
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('contacts.contacts.firstName') }}</mat-label
                  ><input matInput [formField]="contactForm.firstName"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('contacts.contacts.lastName') }}</mat-label
                  ><input matInput [formField]="contactForm.lastName"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('common.field.email') }}</mat-label
                  ><input matInput type="email" [formField]="contactForm.email"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('common.field.phone') }}</mat-label
                  ><input matInput inputmode="tel" [formField]="contactForm.phone"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('contacts.contacts.jobTitle') }}</mat-label
                  ><input matInput [formField]="contactForm.jobTitle"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('contacts.contacts.source') }}</mat-label
                  ><input matInput [formField]="contactForm.source"
                /></mat-form-field>
              </div>
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
                <label
                  >{{ i18n.t('activities.activity.task') }}
                  <select [formField]="activityForm.type">
                    <option value="task">{{ i18n.t('activities.activity.task') }}</option>
                    <option value="call">{{ i18n.t('activities.activity.call') }}</option>
                    <option value="meeting">{{ i18n.t('activities.activity.meeting') }}</option>
                    <option value="note">{{ i18n.t('activities.activity.note') }}</option>
                  </select>
                </label>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('activities.activity.title') }}</mat-label
                  ><input matInput [formField]="activityForm.title"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('activities.activity.body') }}</mat-label
                  ><textarea matInput rows="3" [formField]="activityForm.body"></textarea>
                </mat-form-field>
                <button mat-flat-button type="submit">{{ i18n.t('common.action.add') }}</button>
              </form>
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
                        <strong>{{ activity.title }}</strong
                        ><span>{{ i18n.t(activityTypeKey(activity.type)) }}</span>
                      </header>
                      @if (activity.body) {
                        <p>{{ activity.body }}</p>
                      }
                      <time [attr.datetime]="activity.occurredAt">{{
                        i18n.date(activity.occurredAt, { dateStyle: 'medium', timeStyle: 'short' })
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
              <h2>{{ i18n.t('contacts.duplicates.title') }}</h2>
              <p>{{ i18n.t('contacts.duplicates.description') }}</p>
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
                    ? 'contacts.duplicates.refresh'
                    : 'contacts.duplicates.find'
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
            <div class="empty-state compact-empty">{{ i18n.t('contacts.duplicates.empty') }}</div>
          } @else if (store.duplicates().length > 0) {
            <ul class="duplicate-list">
              @for (candidate of store.duplicates(); track candidate.id) {
                <li>
                  <div>
                    <a [routerLink]="['/contacts', candidate.id]">{{ candidate.displayName }}</a>
                    <span>{{ candidate.email ?? candidate.phone ?? '—' }}</span>
                  </div>
                  <div class="match-summary">
                    <strong>{{ duplicateScore(candidate.score) }}</strong>
                    <span>{{ i18n.t(duplicateReasonKey(candidate.reason)) }}</span>
                  </div>
                  @if (permissions.allows('records.delete')) {
                    @if (mergeConfirmId() === candidate.id) {
                      <div class="merge-confirm" role="alert">
                        <span>{{ i18n.t('contacts.duplicates.mergeWarning') }}</span>
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
                          {{ i18n.t('contacts.duplicates.confirmMerge') }}
                        </button>
                      </div>
                    } @else {
                      <button mat-button type="button" (click)="mergeConfirmId.set(candidate.id)">
                        {{ i18n.t('contacts.duplicates.merge') }}
                      </button>
                    }
                  }
                </li>
              }
            </ul>
          }
        </section>
        <app-attachment-panel entityType="contact" [entityId]="id()" />
      }
    </div>
  `,
  styleUrl: './contact-details.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ContactDetailsPage implements OnInit {
  readonly id = input.required<string>();
  readonly store = inject(ContactDetailsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly saveError = signal<string | null>(null);
  readonly activityOpen = signal(false);
  readonly deleteConfirm = signal(false);
  readonly deleteError = signal<string | null>(null);
  readonly mergeConfirmId = signal<string | null>(null);
  readonly contactModel = signal({
    firstName: '',
    lastName: '',
    email: '',
    phone: '',
    jobTitle: '',
    source: '',
  });
  readonly contactForm = form(this.contactModel, (schema) => {
    required(schema.firstName);
    required(schema.lastName);
    email(schema.email);
    readonlyField(schema, { when: () => !this.permissions.allows('records.update') });
  });
  readonly activityModel = signal<{
    type: 'task' | 'call' | 'meeting' | 'note';
    title: string;
    body: string;
  }>({ type: 'task', title: '', body: '' });
  readonly activityForm = form(this.activityModel, (schema) => required(schema.title));
  private readonly router = inject(Router);

  ngOnInit(): void {
    void this.load();
  }

  async load(): Promise<void> {
    await this.store.load(this.id());
    const contact = this.store.contact();
    if (contact)
      this.contactModel.set({
        firstName: contact.firstName,
        lastName: contact.lastName,
        email: contact.email ?? '',
        phone: contact.phone ?? '',
        jobTitle: contact.jobTitle ?? '',
        source: contact.source ?? '',
      });
  }

  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!this.permissions.allows('records.update')) return;
    this.saveError.set(null);
    if (this.contactForm().invalid()) {
      this.contactForm().markAsTouched();
      return;
    }
    const value = this.contactModel();
    const current = this.store.contact();
    if (!current) return;
    const body: UpdateContact = {
      firstName: value.firstName.trim(),
      lastName: value.lastName.trim(),
      email: value.email.trim() || null,
      phone: value.phone.trim() || null,
      jobTitle: value.jobTitle.trim() || null,
      source: value.source.trim() || null,
      companyId: current.companyId,
      ownerId: current.ownerId,
      status: current.status,
      customFields: current.customFields,
    };
    try {
      await this.store.save(this.id(), body);
    } catch (error) {
      this.saveError.set(
        this.i18n.problem(error instanceof Error ? error.message : 'versionConflict'),
      );
    }
  }

  async addActivity(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.activityForm().invalid()) {
      this.activityForm().markAsTouched();
      return;
    }
    const value = this.activityModel();
    const body: CreateActivity = {
      type: value.type,
      title: value.title.trim(),
      body: value.body.trim() || null,
      relatedType: 'contact',
      relatedId: this.id(),
      priority: 'normal',
    };
    await this.store.addActivity(this.id(), body);
    this.activityModel.set({ type: 'task', title: '', body: '' });
    this.activityOpen.set(false);
  }

  async deleteContact(): Promise<void> {
    this.deleteError.set(null);
    try {
      await this.store.deleteContact(this.id());
      await this.router.navigate(['/contacts']);
    } catch (error) {
      this.deleteError.set(
        this.i18n.problem(error instanceof Error ? error.message : 'contact.delete'),
      );
    }
  }

  async mergeDuplicate(candidateId: string): Promise<void> {
    try {
      await this.store.mergeDuplicate(this.id(), candidateId);
      this.mergeConfirmId.set(null);
    } catch {
      // The persistent duplicate panel renders the localized store error.
    }
  }

  duplicateScore(score: number): string {
    return new Intl.NumberFormat(this.i18n.locale(), {
      style: 'percent',
      maximumFractionDigits: 0,
    }).format(score);
  }

  duplicateReasonKey(reason: string): AppMessageKey {
    switch (reason) {
      case 'email_exact':
        return 'contacts.duplicates.reason.emailExact';
      case 'phone_exact':
        return 'contacts.duplicates.reason.phoneExact';
      default:
        return 'contacts.duplicates.reason.nameSimilar';
    }
  }

  duplicateErrorMessage(): string {
    const error = this.store.duplicatesError();
    return this.i18n.problem(error instanceof Error ? error.message : 'generic');
  }

  activityTypeKey(type: Activity['type']): AppMessageKey {
    return `activities.activity.${type}` as AppMessageKey;
  }
}
