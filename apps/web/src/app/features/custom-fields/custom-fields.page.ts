import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type { CustomFieldDefinitionInput, CustomFieldValueType } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { CustomFieldsStore } from './custom-fields.store';

@Component({
  selector: 'app-custom-fields-page',
  imports: [ErrorPanelComponent, FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  providers: [CustomFieldsStore],
  template: `
    <div class="page settings-feature">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('customFields.title') }}</h1>
          <p>{{ i18n.t('customFields.subtitle') }}</p>
        </div>
        @if (permissions.allows('settings.write')) {
          <button mat-flat-button type="button" (click)="createOpen.set(!createOpen())">
            {{ i18n.t('customFields.add') }}
          </button>
        }
      </header>
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      @if (createOpen() && permissions.allows('settings.write')) {
        <section class="panel editor">
          <form class="feature-form" (submit)="create($event)" novalidate>
            <label class="native-field"
              ><span>{{ i18n.t('customFields.entity') }}</span
              ><select [formField]="fieldForm.entityType">
                <option value="contact">{{ i18n.t('web.entity.contact') }}</option>
                <option value="company">{{ i18n.t('web.entity.company') }}</option>
                <option value="lead">{{ i18n.t('web.entity.lead') }}</option>
                <option value="deal">{{ i18n.t('web.entity.deal') }}</option>
              </select></label
            >
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('customFields.key') }}</mat-label
              ><input matInput [formField]="fieldForm.fieldKey"
            /></mat-form-field>
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('customFields.label') }}</mat-label
              ><input matInput [formField]="fieldForm.label"
            /></mat-form-field>
            <label class="native-field"
              ><span>{{ i18n.t('customFields.type') }}</span
              ><select [formField]="fieldForm.valueType">
                @for (type of valueTypes; track type) {
                  <option [value]="type">{{ i18n.t(typeKey(type)) }}</option>
                }
              </select></label
            >
            <label class="check-field"
              ><input type="checkbox" [formField]="fieldForm.required" />{{
                i18n.t('customFields.required')
              }}</label
            >
            @if (model().valueType === 'single_select' || model().valueType === 'multi_select') {
              <mat-form-field class="wide" appearance="outline"
                ><mat-label>{{ i18n.t('customFields.options') }}</mat-label
                ><textarea matInput rows="3" [formField]="fieldForm.options"></textarea
                ><mat-hint>{{ i18n.t('customFields.optionsHint') }}</mat-hint></mat-form-field
              >
            }
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
      <section class="panel field-list" [attr.aria-busy]="store.loading()">
        @if (store.loading()) {
          <div class="list-skeleton">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else {
          @for (field of store.definitions(); track field.id) {
            <article>
              <div>
                <h2>{{ field.label }}</h2>
                <p>
                  <code>{{ field.fieldKey }}</code> · {{ i18n.t(typeKey(field.valueType)) }} ·
                  {{ i18n.t(entityKey(field.entityType)) }}
                </p>
              </div>
              <span class="status-pill">v{{ field.schemaVersion }}</span>
              @if (permissions.allows('settings.write')) {
                <button mat-button type="button" (click)="store.remove(field)">
                  {{ i18n.t('common.action.delete') }}
                </button>
              }
            </article>
          } @empty {
            <div class="empty-state">{{ i18n.t('customFields.empty') }}</div>
          }
        }
      </section>
    </div>
  `,
  styles: `
    .settings-feature {
      max-width: 70rem;
    }
    .editor {
      padding: 1rem;
    }
    .feature-form {
      grid-template-columns: repeat(3, minmax(11rem, 1fr));
    }
    .wide {
      grid-column: span 2;
    }
    .check-field {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      min-height: 3.5rem;
    }
    .field-list article {
      display: grid;
      grid-template-columns: 1fr auto auto;
      align-items: center;
      gap: 0.75rem;
      padding: 0.85rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .field-list article:last-child {
      border: 0;
    }
    article h2 {
      margin: 0;
      font-size: 0.9rem;
    }
    article p {
      margin: 0.25rem 0 0;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    @media (max-width: 700px) {
      .feature-form {
        grid-template-columns: 1fr;
      }
      .wide {
        grid-column: auto;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CustomFieldsPage implements OnInit {
  readonly store = inject(CustomFieldsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly valueTypes: readonly CustomFieldValueType[] = [
    'text',
    'multiline_text',
    'number',
    'money',
    'date',
    'boolean',
    'single_select',
    'multi_select',
    'user_reference',
  ];
  readonly model = signal<{
    readonly entityType: CustomFieldDefinitionInput['entityType'];
    readonly fieldKey: string;
    readonly label: string;
    readonly valueType: CustomFieldValueType;
    readonly required: boolean;
    readonly options: string;
  }>({
    entityType: 'contact',
    fieldKey: '',
    label: '',
    valueType: 'text',
    required: false,
    options: '',
  });
  readonly fieldForm = form(this.model, (schema) => {
    required(schema.fieldKey);
    required(schema.label);
  });
  ngOnInit(): void {
    void this.store.load();
  }
  typeKey(type: CustomFieldValueType): `customFields.type.${CustomFieldValueType}` {
    return `customFields.type.${type}`;
  }
  entityKey(
    entityType: CustomFieldDefinitionInput['entityType'],
  ): 'web.entity.contact' | 'web.entity.company' | 'web.entity.lead' | 'web.entity.deal' {
    return `web.entity.${entityType}`;
  }
  async create(event: Event): Promise<void> {
    event.preventDefault();
    if (this.fieldForm().invalid()) return;
    const value = this.model();
    await this.store.create({
      entityType: value.entityType,
      fieldKey: value.fieldKey.trim(),
      label: value.label.trim(),
      valueType: value.valueType,
      validation: { required: value.required },
      options: parseOptions(value.options),
    });
    this.model.set({
      entityType: 'contact',
      fieldKey: '',
      label: '',
      valueType: 'text',
      required: false,
      options: '',
    });
    this.createOpen.set(false);
  }
}

function parseOptions(value: string): { value: string; label: string }[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [key, ...label] = line.split(':');
      return { value: key?.trim() ?? '', label: (label.join(':').trim() || key?.trim()) ?? '' };
    });
}
