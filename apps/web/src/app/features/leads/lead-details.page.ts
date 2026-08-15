import { ChangeDetectionStrategy, Component, effect, inject, input, signal } from '@angular/core';
import { FormField, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Router, RouterLink } from '@angular/router';

import type { LeadInput, LeadStage } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { CustomFieldEditorComponent } from '../../shared/custom-fields/custom-field-editor.component';
import { PhoneInputComponent } from '../../shared/forms/phone-input.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { EntityChatComponent } from '../chat/entity-chat.component';
import { LeadDetailsStore } from './lead-details.store';

@Component({
  selector: 'app-lead-details-page',
  imports: [
    CustomFieldEditorComponent,
    EntityChatComponent,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    PhoneInputComponent,
    RecordAssignmentsComponent,
    RouterLink,
  ],
  providers: [LeadDetailsStore],
  template: `
    <div class="page lead-details">
      <a mat-button class="back-control" routerLink="/leads"
        ><app-icon name="back" />{{ i18n.t('leads.back') }}</a
      >
      @if (store.loading()) {
        <div class="panel skeleton loading"></div>
      } @else if (store.loadError() && !store.lead()) {
        <app-error-panel [error]="store.loadError()" (retry)="load()" />
      } @else if (store.lead(); as lead) {
        <header class="panel hero">
          <span class="mark"><app-icon name="lead" /></span>
          <div>
            <p>{{ i18n.t('leads.details') }}</p>
            <h1>{{ lead.name }}</h1>
            <span>{{ store.stageName(lead.stageId) }}</span>
          </div>
          @if (lead.status !== 'converted' && permissions.allows('leads.update')) {
            <div class="outcomes">
              <button mat-button type="button" (click)="lose()">
                {{ i18n.t('leads.markLost') }}
              </button>
              <button mat-flat-button type="button" (click)="convert()">
                {{ i18n.t('leads.markWon') }}
              </button>
            </div>
          } @else if (lead.convertedDealId) {
            <a mat-flat-button [routerLink]="['/deals', lead.convertedDealId]">{{
              i18n.t('leads.openDeal')
            }}</a>
          }
        </header>
        @if (store.mutationError()) {
          <app-error-panel [error]="store.mutationError()" [retryable]="false" />
        }
        <div class="detail-grid">
          <section class="panel editor">
            <h2>{{ i18n.t('leads.overview') }}</h2>
            <form (submit)="save($event)">
              <div class="record-fields">
                <div class="fields">
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('common.field.name') }}</mat-label
                    ><input matInput [formField]="leadForm.name"
                  /></mat-form-field>
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('common.field.email') }}</mat-label
                    ><input matInput type="email" [formField]="leadForm.email"
                  /></mat-form-field>
                  <app-phone-input
                    [formField]="leadForm.phone"
                    [label]="i18n.t('common.field.phone')"
                    (validityChange)="phoneValid.set($event)"
                  />
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('leads.company') }}</mat-label
                    ><input matInput [formField]="leadForm.companyName"
                  /></mat-form-field>
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('leads.jobTitle') }}</mat-label
                    ><input matInput [formField]="leadForm.jobTitle"
                  /></mat-form-field>
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('leads.source') }}</mat-label
                    ><input matInput [formField]="leadForm.source"
                  /></mat-form-field>
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('leads.plannedStart') }}</mat-label
                    ><input matInput type="date" [formField]="leadForm.plannedStartDate"
                  /></mat-form-field>
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('leads.expectedClose') }}</mat-label
                    ><input matInput type="date" [formField]="leadForm.expectedCloseDate"
                  /></mat-form-field>
                  <label class="native-field"
                    ><span>{{ i18n.t('leads.stage') }}</span
                    ><select [formField]="leadForm.stageId">
                      @for (stage of openStages(); track stage.id) {
                        <option [value]="stage.id">{{ stage.displayName }}</option>
                      }
                    </select></label
                  >
                </div>
                <app-custom-field-editor
                  entityType="lead"
                  [values]="customFields()"
                  (valuesChange)="customFields.set($event)"
                />
              </div>
              @if (permissions.allows('leads.update') && lead.status !== 'converted') {
                <div class="actions">
                  <button mat-flat-button type="submit" [disabled]="store.saving()">
                    {{ i18n.t('common.action.save') }}
                  </button>
                </div>
              }
            </form>
          </section>
          <section class="panel assignments">
            <app-record-assignments
              resourceType="lead"
              [resourceId]="lead.id"
              [version]="lead.version"
              (versionChange)="store.setVersion($event)"
            />
          </section>
        </div>
        <app-entity-chat entityType="lead" [entityId]="lead.id" />
      }
    </div>
  `,
  styles: `
    .lead-details {
      max-width: 86rem;
    }
    .loading {
      min-height: 18rem;
    }
    .back-control {
      width: fit-content;
      min-height: 2.5rem;
      color: var(--text-muted);
      text-decoration: none;
    }
    .back-control app-icon {
      margin-inline-end: 0.4rem;
    }
    .hero {
      display: grid;
      grid-template-columns: auto 1fr auto;
      align-items: center;
      gap: 1rem;
      padding: 1.1rem;
    }
    .hero h1,
    .hero p {
      margin: 0;
    }
    .hero p {
      color: var(--text-muted);
      font-size: 0.75rem;
      text-transform: uppercase;
    }
    .mark {
      display: grid;
      width: 3rem;
      height: 3rem;
      place-items: center;
      border-radius: 0.75rem;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .outcomes {
      display: flex;
      gap: 0.5rem;
    }
    .detail-grid {
      display: grid;
      grid-template-columns: minmax(0, 2fr) minmax(18rem, 1fr);
      gap: 1rem;
    }
    .editor,
    .assignments {
      padding: 1rem;
    }
    .editor h2 {
      margin: 0 0 1rem;
    }
    .fields {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.75rem;
    }
    .record-fields {
      display: grid;
      gap: 1rem;
    }
    .actions {
      display: flex;
      justify-content: flex-end;
      margin-top: 1rem;
    }
    @media (max-width: 820px) {
      .detail-grid,
      .fields {
        grid-template-columns: 1fr;
      }
      .hero {
        grid-template-columns: auto 1fr;
      }
      .outcomes {
        grid-column: 1 / -1;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LeadDetailsPage {
  readonly id = input.required<string>();
  readonly store = inject(LeadDetailsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  private readonly router = inject(Router);
  readonly customFields = signal<Record<string, unknown>>({});
  readonly phoneValid = signal(true);
  readonly model = signal({
    name: '',
    email: '',
    phone: '',
    companyName: '',
    jobTitle: '',
    source: '',
    plannedStartDate: '',
    expectedCloseDate: '',
    stageId: '',
  });
  readonly leadForm = form(this.model, (schema) => {
    required(schema.name);
    validate(schema.phone, ({ value }) =>
      !value() || this.phoneValid()
        ? undefined
        : { kind: 'phone', message: this.i18n.t('common.validation.phone') },
    );
  });
  private readonly routeLoad = effect(() => void this.load(this.id()));
  async load(id = this.id()): Promise<void> {
    const lead = await this.store.load(id);
    if (!lead) return;
    this.model.set({
      name: lead.name,
      email: lead.email ?? '',
      phone: lead.phone ?? '',
      companyName: lead.companyName ?? '',
      jobTitle: lead.jobTitle ?? '',
      source: lead.source ?? '',
      plannedStartDate: lead.plannedStartDate ?? '',
      expectedCloseDate: lead.expectedCloseDate ?? '',
      stageId: lead.stageId,
    });
    this.customFields.set({ ...lead.customFields });
  }
  openStages(): readonly LeadStage[] {
    return this.store
      .stages()
      .filter((stage) => stage.category !== 'converted' && stage.systemKey !== 'converted');
  }
  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    const lead = this.store.lead();
    this.leadForm().markAsTouched();
    if (!lead || this.leadForm().invalid() || (!!this.model().phone && !this.phoneValid())) return;
    const value = this.model();
    const body: LeadInput = {
      name: value.name.trim(),
      email: value.email.trim() || null,
      phone: value.phone.trim() || null,
      companyName: value.companyName.trim() || null,
      jobTitle: value.jobTitle.trim() || null,
      source: value.source.trim() || null,
      status: lead.status === 'converted' ? 'qualified' : lead.status,
      stageId: lead.stageId,
      ownerId: lead.ownerId ?? null,
      teamId: lead.teamId ?? null,
      plannedStartDate: value.plannedStartDate || null,
      expectedCloseDate: value.expectedCloseDate || null,
      customFields: this.customFields(),
    };
    if (!(await this.store.save(body))) return;
    const stage = this.store.stages().find((item) => item.id === value.stageId);
    if (stage && stage.id !== lead.stageId) await this.store.move(stage);
  }
  async lose(): Promise<void> {
    const stage =
      this.store.stages().find((item) => item.category === 'disqualified' && item.isDefault) ??
      this.store.stages().find((item) => item.category === 'disqualified');
    if (stage) await this.store.move(stage);
  }
  async convert(): Promise<void> {
    const result = await this.store.win();
    if (result?.dealId) await this.router.navigate(['/deals', result.dealId]);
  }
}
