import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, min, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type {
  AutomationActionType,
  AutomationComparator,
  AutomationEntity,
  AutomationTrigger,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { parseJsonObject } from '../../shared/forms/feature-validation';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { AutomationsStore } from './automations.store';

interface AutomationFormModel {
  readonly name: string;
  readonly trigger: AutomationTrigger;
  readonly entityType: AutomationEntity;
  readonly conditionField: string;
  readonly comparator: AutomationComparator;
  readonly conditionValue: string;
  readonly actionType: AutomationActionType;
  readonly actionParams: string;
  readonly rateLimit: number;
}

@Component({
  selector: 'app-automations-page',
  imports: [
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
  ],
  providers: [AutomationsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.automations') }}</h1>
          <p>{{ i18n.t('automations.subtitle') }}</p>
        </div>
        @if (permissions.allows('settings.write')) {
          <button mat-flat-button type="button" (click)="createOpen.set(!createOpen())">
            <app-icon name="add" />{{ i18n.t('automations.add') }}
          </button>
        }
      </header>
      @if (!permissions.allows('settings.write')) {
        <div class="error-panel" role="alert">{{ i18n.t('automations.permission') }}</div>
      } @else {
        @if (store.error()) {
          <app-error-panel [error]="store.error()" (retry)="store.load()" />
        }
        @if (formError()) {
          <div class="error-panel" role="alert">{{ i18n.t('automations.jsonError') }}</div>
        }
        @if (createOpen()) {
          <section class="panel editor">
            <form class="feature-form" (submit)="create($event)" novalidate>
              <mat-form-field appearance="outline"
                ><mat-label>{{ i18n.t('common.field.name') }}</mat-label
                ><input matInput [formField]="ruleForm.name"
              /></mat-form-field>
              <label class="native-field"
                ><span>{{ i18n.t('automations.trigger') }}</span
                ><select [formField]="ruleForm.trigger">
                  @for (item of triggers; track item) {
                    <option [value]="item">{{ i18n.t(triggerKey(item)) }}</option>
                  }
                </select></label
              >
              <label class="native-field"
                ><span>{{ i18n.t('automations.entity') }}</span
                ><select [formField]="ruleForm.entityType">
                  @for (item of entities; track item) {
                    <option [value]="item">{{ i18n.t(entityKey(item)) }}</option>
                  }
                </select></label
              >
              <mat-form-field appearance="outline"
                ><mat-label>{{ i18n.t('automations.conditionField') }}</mat-label
                ><input matInput [formField]="ruleForm.conditionField"
              /></mat-form-field>
              <label class="native-field"
                ><span>{{ i18n.t('automations.comparator') }}</span
                ><select [formField]="ruleForm.comparator">
                  @for (item of comparators; track item) {
                    <option [value]="item">{{ i18n.t(comparatorKey(item)) }}</option>
                  }
                </select></label
              >
              <mat-form-field appearance="outline"
                ><mat-label>{{ i18n.t('automations.conditionValue') }}</mat-label
                ><input matInput [formField]="ruleForm.conditionValue"
              /></mat-form-field>
              <label class="native-field"
                ><span>{{ i18n.t('automations.action') }}</span
                ><select [formField]="ruleForm.actionType">
                  @for (item of actions; track item) {
                    <option [value]="item">{{ i18n.t(actionKey(item)) }}</option>
                  }
                </select></label
              >
              <mat-form-field class="wide" appearance="outline"
                ><mat-label>{{ i18n.t('automations.actionParams') }}</mat-label
                ><textarea matInput rows="3" [formField]="ruleForm.actionParams"></textarea>
              </mat-form-field>
              <mat-form-field appearance="outline"
                ><mat-label>{{ i18n.t('automations.rateLimit') }}</mat-label
                ><input matInput type="number" [formField]="ruleForm.rateLimit"
              /></mat-form-field>
              <div class="form-actions">
                <button mat-button type="button" (click)="createOpen.set(false)">
                  {{ i18n.t('common.action.cancel') }}</button
                ><button mat-flat-button type="submit" [disabled]="store.saving()">
                  {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
                </button>
              </div>
            </form>
          </section>
        }
        <section class="panel record-list" [attr.aria-busy]="store.loading()">
          @if (store.loading()) {
            <div class="list-skeleton">
              <div class="skeleton"></div>
              <div class="skeleton"></div>
            </div>
          } @else if (store.rules().length === 0) {
            <div class="empty-state">{{ i18n.t('automations.empty') }}</div>
          } @else {
            @for (rule of store.rules(); track rule.id) {
              <article>
                <div>
                  <h2>{{ rule.name }}</h2>
                  <p>
                    {{ i18n.t(triggerKey(rule.trigger)) }} ·
                    {{ i18n.t(entityKey(rule.entityType)) }} · {{ rule.actions.length }}
                    {{ i18n.t('automations.actions') }}
                  </p>
                </div>
                <button
                  mat-stroked-button
                  type="button"
                  [attr.aria-pressed]="rule.enabled"
                  (click)="store.toggle(rule)"
                >
                  {{ i18n.t(rule.enabled ? 'automations.enabled' : 'automations.disabled') }}
                </button>
              </article>
            }
          }
        </section>
      }
    </div>
  `,
  styles: `
    .editor {
      padding: 1rem 1rem 0.85rem;
    }
    .feature-form {
      grid-template-columns: repeat(2, minmax(14rem, 1fr));
    }
    .wide {
      grid-column: span 2;
    }
    .feature-form .form-actions {
      grid-column: 1 / -1;
      padding-top: 0.25rem;
      border-top: 1px solid var(--border);
    }
    .record-list article {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 1rem;
      border-bottom: 1px solid var(--border);
    }
    .record-list article:last-child {
      border: 0;
    }
    article h2 {
      margin: 0;
      font-size: 0.95rem;
    }
    article p {
      margin: 0.3rem 0 0;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    @media (max-width: 760px) {
      .feature-form {
        grid-template-columns: 1fr;
      }
      .wide {
        grid-column: auto;
      }
      .feature-form .form-actions {
        grid-column: auto;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AutomationsPage implements OnInit {
  readonly store = inject(AutomationsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly formError = signal(false);
  readonly triggers: readonly AutomationTrigger[] = [
    'record_created',
    'record_updated',
    'deal_stage_changed',
    'deal_won',
    'deal_lost',
    'task_overdue',
    'scheduled',
  ];
  readonly entities: readonly AutomationEntity[] = [
    'contact',
    'company',
    'lead',
    'deal',
    'activity',
    'workspace',
  ];
  readonly comparators: readonly AutomationComparator[] = [
    'equals',
    'not_equals',
    'contains',
    'greater_than',
    'greater_or_equal',
    'less_than',
    'less_or_equal',
    'date_before',
    'date_after',
    'tag_present',
    'owner_equals',
    'team_equals',
  ];
  readonly actions: readonly AutomationActionType[] = [
    'create_task',
    'assign_owner',
    'add_tag',
    'remove_tag',
    'create_notification',
    'send_email',
    'invoke_webhook',
    'update_field',
  ];
  readonly model = signal<AutomationFormModel>({
    name: '',
    trigger: 'record_created',
    entityType: 'lead',
    conditionField: 'status',
    comparator: 'equals',
    conditionValue: 'new',
    actionType: 'create_task',
    actionParams: '{"titleKey":"activities.activity.task","dueInHours":24,"priority":"normal"}',
    rateLimit: 100,
  });
  readonly ruleForm = form(this.model, (schema) => {
    required(schema.name);
    required(schema.conditionField);
    required(schema.conditionValue);
    required(schema.actionParams);
    min(schema.rateLimit, 1);
  });

  ngOnInit(): void {
    if (this.permissions.allows('settings.write')) void this.store.load();
  }
  triggerKey(value: AutomationTrigger): `automations.trigger.${AutomationTrigger}` {
    return `automations.trigger.${value}`;
  }
  entityKey(value: AutomationEntity): `automations.entity.${AutomationEntity}` {
    return `automations.entity.${value}`;
  }
  comparatorKey(value: AutomationComparator): `automations.comparator.${AutomationComparator}` {
    return `automations.comparator.${value}`;
  }
  actionKey(value: AutomationActionType): `automations.action.${AutomationActionType}` {
    return `automations.action.${value}`;
  }

  async create(event: Event): Promise<void> {
    event.preventDefault();
    if (this.ruleForm().invalid()) return;
    const value = this.model();
    const params = parseJsonObject(value.actionParams);
    if (!params) {
      this.formError.set(true);
      return;
    }
    this.formError.set(false);
    await this.store.create({
      name: value.name.trim(),
      trigger: value.trigger,
      entityType: value.entityType,
      conditions: {
        field: value.conditionField.trim(),
        operator: value.comparator,
        value: scalarValue(value.conditionValue, value.comparator),
      },
      actions: [{ type: value.actionType, params }],
      enabled: false,
      rateLimitPerHour: value.rateLimit,
    });
    this.createOpen.set(false);
  }
}

function scalarValue(value: string, comparator: AutomationComparator): string | number {
  if (['greater_than', 'greater_or_equal', 'less_than', 'less_or_equal'].includes(comparator)) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : value;
  }
  return value;
}
