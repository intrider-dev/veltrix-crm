import { ChangeDetectionStrategy, Component, effect, inject, input, signal } from '@angular/core';
import { FormField, form, min, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';

import type {
  CreateActivity,
  DealLineItemInput,
  DealParticipantInput,
  DealUpdateInput,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { AttachmentPanelComponent } from '../../shared/attachments/attachment-panel.component';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { CustomFieldEditorComponent } from '../../shared/custom-fields/custom-field-editor.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { EntityChatComponent } from '../chat/entity-chat.component';
import { DealDetailsStore } from './deal-details.store';

interface DealEditorModel {
  readonly name: string;
  readonly amount: number;
  readonly currency: string;
  readonly plannedStartDate: string;
  readonly expectedCloseDate: string;
  readonly forecastCategory: DealUpdateInput['forecastCategory'];
}

@Component({
  selector: 'app-deal-details-page',
  imports: [
    AttachmentPanelComponent,
    CustomFieldEditorComponent,
    EntityChatComponent,
    RecordAssignmentsComponent,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  providers: [DealDetailsStore],
  template: `
    <div class="page deal-details">
      <a mat-button class="back-control" routerLink="/deals">
        <app-icon name="back" />{{ i18n.t('sales.deal.back') }}
      </a>

      @if (store.loading()) {
        <div
          class="panel hero-skeleton skeleton"
          [attr.aria-label]="i18n.t('common.app.loading')"
        ></div>
        <div class="panel body-skeleton skeleton"></div>
      } @else if (store.error() && !store.deal()) {
        <app-error-panel [error]="store.error()" (retry)="load()" />
      } @else if (store.deal(); as deal) {
        <header class="panel deal-hero">
          <div class="deal-mark" aria-hidden="true"><app-icon name="deal" /></div>
          <div>
            <p class="eyebrow">{{ i18n.t('sales.deal.details') }}</p>
            <h1>{{ deal.name }}</h1>
            <p>
              {{ i18n.money(deal.amountMinor, deal.currency) }} ·
              {{ store.stageName(deal.stageId) }}
            </p>
          </div>
          <span
            class="status"
            [class.won]="deal.status === 'won'"
            [class.lost]="deal.status === 'lost'"
          >
            {{ i18n.t(statusKey(deal.status)) }}
          </span>
        </header>

        @if (store.conflict()) {
          <section class="conflict" role="alert">
            <span>{{ i18n.t('sales.deal.versionConflict') }}</span>
            <button mat-button type="button" (click)="load()">
              {{ i18n.t('common.action.retry') }}
            </button>
          </section>
        }
        @if (actionError()) {
          <section class="form-error" role="alert">{{ actionError() }}</section>
        }

        <div class="details-grid">
          <section class="panel edit-panel" aria-labelledby="deal-overview-title">
            <header>
              <h2 id="deal-overview-title">{{ i18n.t('sales.deal.overview') }}</h2>
            </header>
            @if (permissions.allows('deals.update')) {
              <form (submit)="save($event)">
                <div class="record-fields">
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
                    <input matInput [formField]="dealForm.name" />
                  </mat-form-field>
                  <div class="field-grid">
                    <mat-form-field appearance="outline">
                      <mat-label>{{ i18n.t('sales.deal.amount') }}</mat-label>
                      <input matInput type="number" [formField]="dealForm.amount" />
                    </mat-form-field>
                    <mat-form-field appearance="outline">
                      <mat-label>{{ i18n.t('sales.deal.currency') }}</mat-label>
                      <input matInput [formField]="dealForm.currency" />
                    </mat-form-field>
                  </div>
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('sales.deal.plannedStart') }}</mat-label>
                    <input matInput type="date" [formField]="dealForm.plannedStartDate" />
                  </mat-form-field>
                  <mat-form-field appearance="outline">
                    <mat-label>{{ i18n.t('sales.deal.expectedClose') }}</mat-label>
                    <input matInput type="date" [formField]="dealForm.expectedCloseDate" />
                  </mat-form-field>
                  <label class="native-field">
                    {{ i18n.t('sales.deal.forecastCategory') }}
                    <select [formField]="dealForm.forecastCategory">
                      <option value="pipeline">{{ i18n.t('sales.forecast.pipeline') }}</option>
                      <option value="best_case">{{ i18n.t('sales.forecast.bestCase') }}</option>
                      <option value="commit">{{ i18n.t('sales.forecast.commit') }}</option>
                      <option value="omitted">{{ i18n.t('sales.forecast.omitted') }}</option>
                    </select>
                  </label>
                  <app-custom-field-editor
                    entityType="deal"
                    [values]="customFields()"
                    (valuesChange)="customFields.set($event)"
                  />
                </div>
                <div class="actions">
                  <button mat-flat-button type="submit" [disabled]="store.saving()">
                    {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.save') }}
                  </button>
                </div>
              </form>

              <div class="outcome-panel">
                <h3>{{ i18n.t('sales.deal.outcome') }}</h3>
                <mat-form-field appearance="outline">
                  <mat-label>{{ i18n.t('sales.deal.lostReason') }}</mat-label>
                  <input matInput [formField]="outcomeForm.lostReason" />
                </mat-form-field>
                <div class="outcome-actions">
                  <button mat-button type="button" (click)="setOutcome('open')">
                    {{ i18n.t('sales.status.open') }}
                  </button>
                  <button mat-flat-button type="button" (click)="setOutcome('won')">
                    {{ i18n.t('sales.status.won') }}
                  </button>
                  <button mat-button type="button" (click)="setOutcome('lost')">
                    {{ i18n.t('sales.status.lost') }}
                  </button>
                </div>
              </div>
            }
          </section>

          <div class="side-stack">
            <section class="panel collection-panel assignment-card">
              <app-record-assignments
                resourceType="deal"
                [resourceId]="deal.id"
                [version]="deal.version"
                (versionChange)="store.setVersion($event)"
              />
            </section>
            <section class="panel collection-panel" aria-labelledby="line-items-title">
              <header>
                <h2 id="line-items-title">{{ i18n.t('sales.lineItems.title') }}</h2>
              </header>
              <ul>
                @for (item of store.lineItems(); track item.id) {
                  <li>
                    <span>{{ item.name }} × {{ item.quantity }}</span
                    ><strong>{{ i18n.money(item.unitPriceMinor, item.currency) }}</strong>
                  </li>
                } @empty {
                  <li class="empty-state">{{ i18n.t('sales.lineItems.empty') }}</li>
                }
              </ul>
              @if (permissions.allows('deals.update')) {
                <form class="inline-form" (submit)="addLineItem($event)">
                  <input
                    [formField]="lineItemForm.name"
                    [placeholder]="i18n.t('sales.lineItems.name')"
                  />
                  <input
                    type="number"
                    step="0.01"
                    [formField]="lineItemForm.quantity"
                    [attr.aria-label]="i18n.t('sales.lineItems.quantity')"
                  />
                  <input
                    type="number"
                    [formField]="lineItemForm.unitPrice"
                    [attr.aria-label]="i18n.t('sales.lineItems.unitPrice')"
                  />
                  <button mat-button type="submit">{{ i18n.t('common.action.add') }}</button>
                </form>
              }
            </section>

            <section class="panel collection-panel" aria-labelledby="participants-title">
              <header>
                <h2 id="participants-title">{{ i18n.t('sales.participants.title') }}</h2>
              </header>
              <ul>
                @for (participant of store.participants(); track participant.contactId) {
                  <li>
                    <a [routerLink]="['/contacts', participant.contactId]">{{
                      participant.displayName
                    }}</a
                    ><span>{{ participant.role || participant.email || '—' }}</span>
                  </li>
                } @empty {
                  <li class="empty-state">{{ i18n.t('sales.participants.empty') }}</li>
                }
              </ul>
              @if (permissions.allows('deals.update')) {
                <form class="inline-form participant-form" (submit)="addParticipant($event)">
                  <input
                    [formField]="participantForm.contactId"
                    [placeholder]="i18n.t('sales.participants.contactId')"
                  />
                  <input
                    [formField]="participantForm.role"
                    [placeholder]="i18n.t('sales.participants.role')"
                  />
                  <button mat-button type="submit">{{ i18n.t('common.action.add') }}</button>
                </form>
              }
            </section>
          </div>

          <section class="panel timeline-panel" aria-labelledby="timeline-title">
            <header>
              <h2 id="timeline-title">{{ i18n.t('activities.activity.timeline') }}</h2>
            </header>
            @if (permissions.allows('records.create')) {
              <form class="task-form" (submit)="addTask($event)">
                <mat-form-field appearance="outline">
                  <mat-label>{{ i18n.t('activities.activity.title') }}</mat-label>
                  <input matInput [formField]="taskForm.title" />
                </mat-form-field>
                <mat-form-field appearance="outline">
                  <mat-label>{{ i18n.t('activities.activity.due') }}</mat-label>
                  <input matInput type="datetime-local" [formField]="taskForm.dueAt" />
                </mat-form-field>
                <button mat-flat-button type="submit">
                  {{ i18n.t('activities.activity.add') }}
                </button>
              </form>
            }
            <ol class="timeline">
              @for (activity of store.activities(); track activity.id) {
                <li>
                  <span class="dot" aria-hidden="true"></span>
                  <article>
                    <header>
                      <strong>{{ activity.title }}</strong
                      ><span>{{ activity.type }}</span>
                    </header>
                    @if (activity.body) {
                      <p>{{ activity.body }}</p>
                    }
                    <footer>
                      <time [attr.datetime]="activity.occurredAt">{{
                        i18n.date(activity.occurredAt, { dateStyle: 'medium', timeStyle: 'short' })
                      }}</time>
                      @if (
                        activity.type === 'task' &&
                        activity.status === 'open' &&
                        permissions.allows('records.update')
                      ) {
                        <button mat-button type="button" (click)="completeTask(activity)">
                          {{ i18n.t('activities.activity.complete') }}
                        </button>
                      }
                    </footer>
                  </article>
                </li>
              }
              @for (entry of store.history(); track entry.id) {
                <li>
                  <span class="dot history-dot" aria-hidden="true"></span>
                  <article>
                    <strong>{{ i18n.t('sales.history.stageChanged') }}</strong>
                    <p>
                      {{ store.stageName(entry.fromStageId) }} →
                      {{ store.stageName(entry.toStageId) }}
                    </p>
                    <time [attr.datetime]="entry.changedAt">{{
                      i18n.date(entry.changedAt, { dateStyle: 'medium', timeStyle: 'short' })
                    }}</time>
                  </article>
                </li>
              }
            </ol>
          </section>
        </div>
        <app-attachment-panel entityType="deal" [entityId]="id()" />
        <app-entity-chat entityType="deal" [entityId]="deal.id" />
      }
    </div>
  `,
  styleUrl: './deal-details.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DealDetailsPage {
  readonly id = input.required<string>();
  readonly store = inject(DealDetailsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly actionError = signal<string | null>(null);
  readonly customFields = signal<Record<string, unknown>>({});
  readonly dealModel = signal<DealEditorModel>({
    name: '',
    amount: 0,
    currency: 'USD',
    plannedStartDate: '',
    expectedCloseDate: '',
    forecastCategory: 'pipeline',
  });
  readonly dealForm = form(this.dealModel, (schema) => {
    required(schema.name);
    min(schema.amount, 0);
    required(schema.currency);
  });
  readonly outcomeModel = signal({ lostReason: '' });
  readonly outcomeForm = form(this.outcomeModel);
  readonly lineItemModel = signal({ name: '', quantity: 1, unitPrice: 0 });
  readonly lineItemForm = form(this.lineItemModel, (schema) => {
    required(schema.name);
    min(schema.quantity, 0.0001);
    min(schema.unitPrice, 0);
  });
  readonly participantModel = signal({ contactId: '', role: '' });
  readonly participantForm = form(this.participantModel, (schema) => required(schema.contactId));
  readonly taskModel = signal({ title: '', dueAt: '' });
  readonly taskForm = form(this.taskModel, (schema) => required(schema.title));

  private readonly routeLoad = effect(() => void this.load(this.id()));

  async load(id = this.id()): Promise<void> {
    const deal = await this.store.load(id);
    if (!deal) return;
    this.dealModel.set({
      name: deal.name,
      amount: deal.amountMinor / 100,
      currency: deal.currency,
      plannedStartDate: deal.plannedStartDate ?? '',
      expectedCloseDate: deal.expectedCloseDate ?? '',
      forecastCategory: normalizeForecastCategory(deal.forecastCategory),
    });
    this.outcomeModel.set({ lostReason: deal.lostReason ?? '' });
    this.customFields.set({ ...deal.customFields });
  }

  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.dealForm().invalid()) {
      this.dealForm().markAsTouched();
      return;
    }
    const current = this.store.deal();
    if (!current) return;
    const value = this.dealModel();
    const body: DealUpdateInput = {
      name: value.name.trim(),
      pipelineId: current.pipelineId,
      stageId: current.stageId,
      contactId: current.contactId ?? null,
      companyId: current.companyId ?? null,
      ownerId: current.ownerId ?? null,
      amountMinor: Math.round(value.amount * 100),
      currency: value.currency.trim().toUpperCase(),
      plannedStartDate: value.plannedStartDate || null,
      expectedCloseDate: value.expectedCloseDate || null,
      forecastCategory: value.forecastCategory,
      customFields: this.customFields(),
    };
    await this.run(() => this.store.save(body));
  }

  async setOutcome(status: 'open' | 'won' | 'lost'): Promise<void> {
    const lostReason = status === 'lost' ? this.outcomeModel().lostReason.trim() || null : null;
    if (status === 'lost' && !lostReason) {
      this.actionError.set(this.i18n.t('sales.deal.lostReasonRequired'));
      return;
    }
    await this.run(() => this.store.setOutcome(status, lostReason));
  }

  async addLineItem(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.lineItemForm().invalid()) {
      this.lineItemForm().markAsTouched();
      return;
    }
    const deal = this.store.deal();
    if (!deal) return;
    const value = this.lineItemModel();
    const body: DealLineItemInput = {
      name: value.name.trim(),
      quantity: String(value.quantity),
      unitPriceMinor: Math.round(value.unitPrice * 100),
      currency: deal.currency,
      position: this.store.lineItems().length,
    };
    await this.run(() => this.store.addLineItem(body));
    if (!this.actionError()) this.lineItemModel.set({ name: '', quantity: 1, unitPrice: 0 });
  }

  async addParticipant(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.participantForm().invalid()) {
      this.participantForm().markAsTouched();
      return;
    }
    const value = this.participantModel();
    const body: DealParticipantInput = {
      contactId: value.contactId.trim(),
      role: value.role.trim() || null,
    };
    await this.run(() => this.store.addParticipant(body));
    if (!this.actionError()) this.participantModel.set({ contactId: '', role: '' });
  }

  async addTask(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.taskForm().invalid()) {
      this.taskForm().markAsTouched();
      return;
    }
    const value = this.taskModel();
    const body: CreateActivity = {
      type: 'task',
      title: value.title.trim(),
      relatedType: 'deal',
      relatedId: this.id(),
      dueAt: value.dueAt ? new Date(value.dueAt).toISOString() : null,
      priority: 'normal',
    };
    await this.run(() => this.store.addActivity(body));
    if (!this.actionError()) this.taskModel.set({ title: '', dueAt: '' });
  }

  async completeTask(activity: Parameters<DealDetailsStore['completeActivity']>[0]): Promise<void> {
    await this.run(() => this.store.completeActivity(activity));
  }

  statusKey(
    status: 'open' | 'won' | 'lost',
  ): 'sales.status.open' | 'sales.status.won' | 'sales.status.lost' {
    return `sales.status.${status}`;
  }

  private async run(operation: () => Promise<void>): Promise<void> {
    this.actionError.set(null);
    try {
      await operation();
    } catch {
      this.actionError.set(this.i18n.t('web.status.error'));
    }
  }
}

function normalizeForecastCategory(value: string): DealUpdateInput['forecastCategory'] {
  switch (value) {
    case 'best_case':
    case 'commit':
    case 'omitted':
    case 'pipeline':
      return value;
    default:
      return 'pipeline';
  }
}
