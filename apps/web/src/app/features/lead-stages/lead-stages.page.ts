import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type { LeadStage, LeadStageCategory, LeadStageInput } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { LeadStagesStore } from './lead-stages.store';

interface StageModel {
  readonly name: string;
  readonly category: LeadStageCategory;
  readonly color: string;
}

@Component({
  selector: 'app-lead-stages-page',
  imports: [ErrorPanelComponent, FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  providers: [LeadStagesStore],
  template: `
    <div class="page stages-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('leadStages.title') }}</h1>
          <p>{{ i18n.t('leadStages.subtitle') }}</p>
        </div>
        @if (permissions.allows('lead_stages.manage')) {
          <button mat-flat-button type="button" (click)="startCreate()">
            {{ i18n.t('leadStages.add') }}
          </button>
        }
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      @if (editorOpen() && permissions.allows('lead_stages.manage')) {
        <section class="panel editor">
          <form (submit)="save($event)" novalidate>
            <header>
              <h2>{{ i18n.t(editing() ? 'leadStages.edit' : 'leadStages.create') }}</h2>
              <button mat-button type="button" (click)="closeEditor()">
                {{ i18n.t('common.action.cancel') }}
              </button>
            </header>
            <div class="editor-fields">
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
                @if (editing()?.systemKey) {
                  <input matInput [value]="systemStageName()" readonly aria-readonly="true" />
                } @else {
                  <input matInput [formField]="stageForm.name" />
                }
              </mat-form-field>
              <label class="native-field">
                <span>{{ i18n.t('leadStages.category') }}</span>
                <select
                  [value]="model().category"
                  [disabled]="editing() !== null"
                  (change)="setCategory($event)"
                >
                  @for (category of creatableCategories; track category) {
                    <option [value]="category">{{ i18n.t(categoryKey(category)) }}</option>
                  }
                  @if (model().category === 'converted') {
                    <option value="converted">{{ i18n.t(categoryKey('converted')) }}</option>
                  }
                </select>
              </label>
              <label class="color-field">
                <span>{{ i18n.t('leadStages.color') }}</span>
                <span class="color-control">
                  <input type="color" [value]="model().color" (input)="setColor($event)" />
                  <code>{{ model().color }}</code>
                </span>
              </label>
            </div>
            <div class="form-actions">
              <button
                mat-flat-button
                type="submit"
                [disabled]="store.saving() || stageForm().invalid()"
              >
                {{ i18n.t('common.action.save') }}
              </button>
            </div>
          </form>
        </section>
      }

      <section class="panel stage-list" [attr.aria-busy]="store.loading()">
        @for (stage of store.stages(); track stage.id) {
          <article>
            <span class="stage-swatch" [style.background]="stage.color" aria-hidden="true"></span>
            <div class="stage-copy">
              <strong>{{ stageLabel(stage) }}</strong>
              <small>{{ i18n.t(categoryKey(stage.category)) }}</small>
            </div>
            @if (stage.isDefault) {
              <span class="status-pill">{{ i18n.t('leadStages.default') }}</span>
            }
            @if (permissions.allows('lead_stages.manage')) {
              <div class="stage-actions">
                <button mat-stroked-button type="button" (click)="startEdit(stage)">
                  {{ i18n.t('common.action.edit') }}
                </button>
                @if (!stage.systemKey) {
                  <button mat-button type="button" (click)="store.remove(stage)">
                    {{ i18n.t('common.action.delete') }}
                  </button>
                }
              </div>
            }
          </article>
        } @empty {
          <div class="empty-state">{{ i18n.t('leadStages.empty') }}</div>
        }
      </section>
    </div>
  `,
  styles: `
    .stages-page {
      max-width: 58rem;
    }
    .editor {
      padding: 1rem;
    }
    .editor header {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .editor h2 {
      margin: 0;
      font-size: 1rem;
    }
    .editor-fields {
      display: grid;
      grid-template-columns: 2fr 1fr 1fr;
      gap: 0.75rem;
      margin-top: 1rem;
    }
    .color-field {
      display: grid;
      align-content: start;
      gap: 0.4rem;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .color-control {
      display: flex;
      align-items: center;
      gap: 0.65rem;
      min-height: var(--control-height);
      padding: 0 0.65rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      background: var(--surface-raised);
    }
    .color-control input {
      width: 2rem;
      height: 2rem;
      padding: 0;
      border: 0;
      background: transparent;
    }
    .color-control code {
      color: var(--text);
    }
    .stage-list {
      overflow: hidden;
    }
    .stage-list article {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto auto;
      align-items: center;
      gap: 0.75rem;
      min-height: 4.25rem;
      padding: 0.75rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .stage-list article:last-child {
      border-bottom: 0;
    }
    .stage-swatch {
      width: 0.7rem;
      height: 2.25rem;
      border-radius: 999px;
    }
    .stage-copy {
      display: grid;
      gap: 0.2rem;
    }
    .stage-copy small {
      color: var(--text-muted);
    }
    .stage-actions {
      display: flex;
      align-items: center;
      gap: 0.4rem;
    }
    @media (max-width: 720px) {
      .editor-fields {
        grid-template-columns: 1fr;
      }
      .stage-list article {
        grid-template-columns: auto minmax(0, 1fr);
      }
      .stage-list .status-pill,
      .stage-actions {
        grid-column: 2;
        justify-self: start;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LeadStagesPage implements OnInit {
  readonly store = inject(LeadStagesStore);
  readonly permissions = inject(Permissions);
  readonly i18n = inject(I18nService);
  readonly creatableCategories: readonly Exclude<LeadStageCategory, 'converted'>[] = [
    'new',
    'qualified',
    'disqualified',
  ];
  readonly editorOpen = signal(false);
  readonly editing = signal<LeadStage | null>(null);
  readonly model = signal<StageModel>({ name: '', category: 'new', color: '#64748b' });
  readonly stageForm = form(this.model, (schema) => required(schema.name));

  ngOnInit(): void {
    void this.store.load();
  }

  startCreate(): void {
    this.editing.set(null);
    this.model.set({ name: '', category: 'new', color: '#64748b' });
    this.editorOpen.set(true);
  }

  startEdit(stage: LeadStage): void {
    this.editing.set(stage);
    this.model.set({ name: stage.name, category: stage.category, color: stage.color });
    this.editorOpen.set(true);
  }

  closeEditor(): void {
    this.editorOpen.set(false);
    this.editing.set(null);
  }

  setCategory(event: Event): void {
    const category = (event.target as HTMLSelectElement).value as LeadStageCategory;
    this.model.update((value) => ({ ...value, category }));
  }

  setColor(event: Event): void {
    const color = (event.target as HTMLInputElement).value;
    this.model.update((value) => ({ ...value, color }));
  }

  async save(event: Event): Promise<void> {
    event.preventDefault();
    if (this.stageForm().invalid()) return;
    const input: LeadStageInput = { ...this.model(), name: this.model().name.trim() };
    const stage = this.editing();
    if (stage) await this.store.update(stage, input);
    else await this.store.create(input);
    this.closeEditor();
  }

  stageLabel(stage: LeadStage): string {
    return stage.systemKey ? this.i18n.t(this.categoryKey(stage.systemKey)) : stage.displayName;
  }

  systemStageName(): string {
    const stage = this.editing();
    return stage ? this.stageLabel(stage) : '';
  }

  categoryKey(category: LeadStageCategory): `leadStages.category.${LeadStageCategory}` {
    return `leadStages.category.${category}`;
  }
}
