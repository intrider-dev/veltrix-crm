import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { CustomFieldDefinition, ReferenceUser } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Component({
  selector: 'app-custom-field-editor',
  template: `
    @if (definitions().length > 0) {
      <section class="custom-fields" aria-labelledby="custom-fields-title">
        <h3 id="custom-fields-title">{{ i18n.t('customFields.title') }}</h3>
        <div class="fields-grid">
          @for (field of definitions(); track field.id) {
            <div
              class="native-field"
              role="group"
              [attr.aria-label]="field.label"
              [class.wide]="field.valueType === 'multiline_text'"
            >
              <span
                >{{ field.label }}
                @if (field.validation.required) {
                  *
                }
              </span>
              @switch (field.valueType) {
                @case ('multiline_text') {
                  <textarea
                    rows="4"
                    [value]="textValue(field)"
                    (input)="setText(field, $event)"
                  ></textarea>
                }
                @case ('number') {
                  <input
                    type="number"
                    [value]="numberValue(field)"
                    (input)="setNumber(field, $event)"
                  />
                }
                @case ('money') {
                  <span class="money-field">
                    <input
                      type="number"
                      step="0.01"
                      [value]="moneyAmount(field)"
                      (input)="setMoney(field, $event)"
                    />
                    <input
                      class="currency"
                      maxlength="3"
                      [value]="moneyCurrency(field)"
                      (input)="setMoneyCurrency(field, $event)"
                    />
                  </span>
                }
                @case ('date') {
                  <input type="date" [value]="textValue(field)" (input)="setText(field, $event)" />
                }
                @case ('boolean') {
                  <input
                    type="checkbox"
                    [checked]="booleanValue(field)"
                    (change)="setBoolean(field, $event)"
                  />
                }
                @case ('single_select') {
                  <select [value]="textValue(field)" (change)="setText(field, $event)">
                    <option value=""></option>
                    @for (option of field.options; track option.value) {
                      <option [value]="option.value">{{ option.label }}</option>
                    }
                  </select>
                }
                @case ('multi_select') {
                  <select multiple (change)="setMultiple(field, $event)">
                    @for (option of field.options; track option.value) {
                      <option
                        [value]="option.value"
                        [selected]="arrayValue(field).includes(option.value)"
                      >
                        {{ option.label }}
                      </option>
                    }
                  </select>
                }
                @case ('user_reference') {
                  <select [value]="textValue(field)" (change)="setText(field, $event)">
                    <option value=""></option>
                    @for (member of members(); track member.userId) {
                      <option [value]="member.userId">{{ member.displayName }}</option>
                    }
                  </select>
                }
                @default {
                  <input type="text" [value]="textValue(field)" (input)="setText(field, $event)" />
                }
              }
            </div>
          }
        </div>
      </section>
    }
    @if (loadFailed()) {
      <p class="load-error" role="alert">{{ i18n.t('customFields.loadFailed') }}</p>
    }
  `,
  styles: `
    .custom-fields {
      border-top: 1px solid var(--border);
      padding-top: 1rem;
    }
    h3 {
      margin: 0 0 0.85rem;
      font-size: 0.9rem;
    }
    .fields-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.8rem;
    }
    .wide {
      grid-column: 1 / -1;
    }
    input,
    select,
    textarea {
      width: 100%;
      min-height: 2.6rem;
      padding: 0.55rem 0.7rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      background: var(--surface-raised);
      color: var(--text);
    }
    input[type='checkbox'] {
      width: 1.25rem;
      min-height: 1.25rem;
    }
    .money-field {
      display: grid;
      grid-template-columns: 1fr 5rem;
      gap: 0.5rem;
    }
    .load-error {
      margin: 0.75rem 0 0;
      color: var(--danger);
      font-size: 0.82rem;
    }
    @media (max-width: 680px) {
      .fields-grid {
        grid-template-columns: 1fr;
      }
      .wide {
        grid-column: auto;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CustomFieldEditorComponent {
  readonly entityType = input.required<'lead' | 'deal'>();
  readonly values = input.required<Record<string, unknown>>();
  readonly valuesChange = output<Record<string, unknown>>();
  readonly definitions = signal<readonly CustomFieldDefinition[]>([]);
  readonly members = signal<readonly ReferenceUser[]>([]);
  readonly loadFailed = signal(false);
  readonly i18n = inject(I18nService);
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private loadSequence = 0;

  constructor() {
    effect(() => {
      const workspaceId = this.workspace.id();
      const entityType = this.entityType();
      void this.load(workspaceId, entityType);
    });
  }

  private async load(workspaceId: string | null, entityType: 'lead' | 'deal'): Promise<void> {
    const sequence = ++this.loadSequence;
    this.loadFailed.set(false);
    this.definitions.set([]);
    this.members.set([]);
    if (!workspaceId) return;
    try {
      const definitions = await this.api.listCustomFields(workspaceId, entityType);
      if (sequence !== this.loadSequence || this.workspace.id() !== workspaceId) return;
      this.definitions.set(definitions);
      if (!definitions.some((field) => field.valueType === 'user_reference')) return;
      const members = await this.api.listReferenceUsers(workspaceId);
      if (sequence === this.loadSequence && this.workspace.id() === workspaceId) {
        this.members.set(members);
      }
    } catch {
      if (sequence === this.loadSequence && this.workspace.id() === workspaceId) {
        this.loadFailed.set(true);
      }
    }
  }

  textValue(field: CustomFieldDefinition): string {
    const value = this.values()[field.fieldKey];
    return typeof value === 'string' ? value : '';
  }
  numberValue(field: CustomFieldDefinition): number | '' {
    const value = this.values()[field.fieldKey];
    return typeof value === 'number' ? value : '';
  }
  booleanValue(field: CustomFieldDefinition): boolean {
    return this.values()[field.fieldKey] === true;
  }
  arrayValue(field: CustomFieldDefinition): readonly string[] {
    const value = this.values()[field.fieldKey];
    return Array.isArray(value)
      ? value.filter((item): item is string => typeof item === 'string')
      : [];
  }
  moneyAmount(field: CustomFieldDefinition): number | '' {
    const value = this.moneyValue(field);
    return value ? value.minor / 100 : '';
  }
  moneyCurrency(field: CustomFieldDefinition): string {
    return this.moneyValue(field)?.currency ?? 'USD';
  }
  setText(field: CustomFieldDefinition, event: Event): void {
    const value = (event.target as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement)
      .value;
    this.update(field.fieldKey, value || null);
  }
  setNumber(field: CustomFieldDefinition, event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.update(field.fieldKey, value === '' ? null : Number(value));
  }
  setBoolean(field: CustomFieldDefinition, event: Event): void {
    this.update(field.fieldKey, (event.target as HTMLInputElement).checked);
  }
  setMultiple(field: CustomFieldDefinition, event: Event): void {
    const options = [...(event.target as HTMLSelectElement).selectedOptions].map(
      (item) => item.value,
    );
    this.update(field.fieldKey, options);
  }
  setMoney(field: CustomFieldDefinition, event: Event): void {
    const amount = (event.target as HTMLInputElement).value;
    this.update(
      field.fieldKey,
      amount === ''
        ? null
        : { minor: Math.round(Number(amount) * 100), currency: this.moneyCurrency(field) },
    );
  }
  setMoneyCurrency(field: CustomFieldDefinition, event: Event): void {
    const currency = (event.target as HTMLInputElement).value.toUpperCase().slice(0, 3);
    const current = this.moneyValue(field) ?? { minor: 0, currency: 'USD' };
    this.update(field.fieldKey, { ...current, currency });
  }

  private moneyValue(field: CustomFieldDefinition): { minor: number; currency: string } | null {
    const value = this.values()[field.fieldKey];
    if (!value || typeof value !== 'object') return null;
    const candidate = value as { minor?: unknown; currency?: unknown };
    return typeof candidate.minor === 'number' && typeof candidate.currency === 'string'
      ? { minor: candidate.minor, currency: candidate.currency }
      : null;
  }
  private update(key: string, value: unknown): void {
    const next = { ...this.values() };
    if (value === null || value === '') delete next[key];
    else next[key] = value;
    this.valuesChange.emit(next);
  }
}
