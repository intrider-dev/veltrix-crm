import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type {
  PipelineInput,
  PipelineRecord,
  PipelineStageInput,
  PipelineStageRecord,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ToastService } from '../../shared/feedback/toast.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { PipelineSettingsStore } from './pipeline-settings.store';

type ForecastCategory = PipelineStageInput['forecastCategory'];

@Component({
  selector: 'app-pipeline-settings-page',
  imports: [ErrorPanelComponent, MatButtonModule, MatFormFieldModule, MatInputModule],
  providers: [PipelineSettingsStore],
  template: `
    <div class="page pipeline-settings">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('pipelines.title') }}</h1>
          <p>{{ i18n.t('pipelines.subtitle') }}</p>
        </div>
        @if (canManage()) {
          <button mat-flat-button type="button" (click)="startPipelineCreate()">
            {{ i18n.t('pipelines.addPipeline') }}
          </button>
        }
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      @if (pipelineEditorOpen() && canManage()) {
        <section class="panel editor" aria-labelledby="pipeline-editor-title">
          <header>
            <h2 id="pipeline-editor-title">
              {{
                i18n.t(editingPipeline() ? 'pipelines.editPipeline' : 'pipelines.createPipeline')
              }}
            </h2>
            <button mat-button type="button" (click)="pipelineEditorOpen.set(false)">
              {{ i18n.t('common.action.cancel') }}
            </button>
          </header>
          <div class="editor-fields">
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
              <input matInput [value]="pipelineName()" (input)="setPipelineName($event)" />
            </mat-form-field>
            <label class="toggle-field">
              <input
                type="checkbox"
                [checked]="pipelineDefault()"
                (change)="setPipelineDefault($event)"
              />
              <span>{{ i18n.t('pipelines.default') }}</span>
            </label>
            <button
              mat-flat-button
              type="button"
              [disabled]="store.saving() || !pipelineName().trim()"
              (click)="savePipeline()"
            >
              {{ i18n.t('common.action.save') }}
            </button>
          </div>
        </section>
      }

      @if (stagePipeline(); as pipeline) {
        <section class="panel editor" aria-labelledby="stage-editor-title">
          <header>
            <h2 id="stage-editor-title">
              {{ i18n.t(editingStage() ? 'pipelines.editStage' : 'pipelines.createStage') }}
            </h2>
            <button mat-button type="button" (click)="closeStageEditor()">
              {{ i18n.t('common.action.cancel') }}
            </button>
          </header>
          <div class="stage-fields">
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
              <input matInput [value]="stageName()" (input)="setStageName($event)" />
            </mat-form-field>
            <label class="native-field">
              <span>{{ i18n.t('pipelines.probability') }}</span>
              <input
                type="number"
                min="0"
                max="100"
                [value]="stageProbability()"
                (input)="setStageProbability($event)"
              />
            </label>
            <label class="native-field">
              <span>{{ i18n.t('pipelines.forecast') }}</span>
              <select [value]="stageForecast()" (change)="setStageForecast($event)">
                @for (category of forecastCategories; track category) {
                  <option [value]="category">{{ i18n.t(forecastKey(category)) }}</option>
                }
              </select>
            </label>
            <button
              mat-flat-button
              type="button"
              [disabled]="store.saving() || !stageName().trim()"
              (click)="saveStage(pipeline)"
            >
              {{ i18n.t('common.action.save') }}
            </button>
          </div>
        </section>
      }

      <section class="pipeline-list" [attr.aria-busy]="store.loading()">
        @for (pipeline of store.pipelines(); track pipeline.id) {
          <article class="panel pipeline-card">
            <header>
              <div>
                <h2>{{ pipeline.displayName }}</h2>
                @if (pipeline.isDefault) {
                  <span class="status-pill">{{ i18n.t('pipelines.default') }}</span>
                }
              </div>
              @if (canManage()) {
                <div class="actions">
                  <button mat-stroked-button type="button" (click)="startStageCreate(pipeline)">
                    {{ i18n.t('pipelines.addStage') }}
                  </button>
                  <button mat-button type="button" (click)="startPipelineEdit(pipeline)">
                    {{ i18n.t('common.action.edit') }}
                  </button>
                  @if (!pipeline.isDefault && pipeline.stages.length === 0) {
                    <button mat-button type="button" (click)="deletePipeline(pipeline)">
                      {{ i18n.t('common.action.delete') }}
                    </button>
                  }
                </div>
              }
            </header>
            <ol class="stage-list">
              @for (
                stage of pipeline.stages;
                track stage.id;
                let first = $first;
                let last = $last
              ) {
                <li>
                  <span class="position">{{ $index + 1 }}</span>
                  <div>
                    <strong>{{ stage.displayName }}</strong
                    ><small
                      >{{ stage.probability }}% ·
                      {{ i18n.t(forecastKey(stage.forecastCategory)) }}</small
                    >
                  </div>
                  @if (canManage()) {
                    <div class="actions">
                      <button
                        mat-button
                        type="button"
                        [disabled]="first || store.saving()"
                        [attr.aria-label]="
                          i18n.t('pipelines.moveUpLabel', { name: stage.displayName })
                        "
                        (click)="store.moveStage(pipeline, stage, -1)"
                      >
                        ↑
                      </button>
                      <button
                        mat-button
                        type="button"
                        [disabled]="last || store.saving()"
                        [attr.aria-label]="
                          i18n.t('pipelines.moveDownLabel', { name: stage.displayName })
                        "
                        (click)="store.moveStage(pipeline, stage, 1)"
                      >
                        ↓
                      </button>
                      <button mat-button type="button" (click)="startStageEdit(pipeline, stage)">
                        {{ i18n.t('common.action.edit') }}
                      </button>
                      <button mat-button type="button" (click)="deleteStage(stage)">
                        {{ i18n.t('common.action.delete') }}
                      </button>
                    </div>
                  }
                </li>
              } @empty {
                <li class="empty-state">{{ i18n.t('pipelines.emptyStages') }}</li>
              }
            </ol>
          </article>
        } @empty {
          <div class="panel empty-state">{{ i18n.t('pipelines.empty') }}</div>
        }
      </section>
    </div>
  `,
  styles: `
    .pipeline-settings {
      max-width: 70rem;
    }
    .pipeline-list {
      display: grid;
      gap: 0.8rem;
    }
    .pipeline-card {
      overflow: hidden;
    }
    .pipeline-card > header,
    .editor > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 0.9rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    h2 {
      margin: 0;
      font-size: 1rem;
    }
    .pipeline-card > header > div:first-child {
      display: flex;
      align-items: center;
      gap: 0.55rem;
    }
    .actions {
      display: flex;
      align-items: center;
      gap: 0.25rem;
      flex-wrap: wrap;
    }
    .stage-list {
      margin: 0;
      padding: 0;
      list-style: none;
    }
    .stage-list li {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.75rem;
      min-height: 3.7rem;
      padding: 0.6rem 0.9rem;
      border-bottom: 1px solid var(--border);
    }
    .stage-list li:last-child {
      border-bottom: 0;
    }
    .stage-list li > div:nth-child(2) {
      display: grid;
      gap: 0.15rem;
    }
    .stage-list small {
      color: var(--text-muted);
    }
    .position {
      display: grid;
      width: 1.75rem;
      height: 1.75rem;
      place-items: center;
      border-radius: 50%;
      color: var(--brand);
      background: var(--brand-soft);
      font-size: 0.72rem;
      font-weight: 700;
    }
    .editor {
      padding-bottom: 1rem;
    }
    .editor-fields,
    .stage-fields {
      display: grid;
      grid-template-columns: minmax(14rem, 1fr) auto auto;
      align-items: end;
      gap: 0.75rem;
      padding: 1rem;
    }
    .stage-fields {
      grid-template-columns: minmax(14rem, 1.5fr) minmax(8rem, 0.6fr) minmax(10rem, 0.8fr) auto;
    }
    .toggle-field {
      display: flex;
      align-items: center;
      gap: 0.45rem;
      min-height: var(--control-height);
    }
    @media (max-width: 760px) {
      .editor-fields,
      .stage-fields,
      .stage-list li {
        grid-template-columns: 1fr;
      }
      .position {
        display: none;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PipelineSettingsPage implements OnInit {
  readonly store = inject(PipelineSettingsStore);
  readonly i18n = inject(I18nService);
  private readonly permissions = inject(Permissions);
  private readonly toasts = inject(ToastService);
  readonly forecastCategories: readonly ForecastCategory[] = [
    'pipeline',
    'best_case',
    'commit',
    'omitted',
  ];
  readonly pipelineEditorOpen = signal(false);
  readonly editingPipeline = signal<PipelineRecord | null>(null);
  readonly pipelineName = signal('');
  readonly pipelineDefault = signal(false);
  readonly stagePipeline = signal<PipelineRecord | null>(null);
  readonly editingStage = signal<PipelineStageRecord | null>(null);
  readonly stageName = signal('');
  readonly stageProbability = signal(20);
  readonly stageForecast = signal<ForecastCategory>('pipeline');

  ngOnInit(): void {
    void this.store.load();
  }
  canManage(): boolean {
    return this.permissions.allows('deal_stages.manage');
  }
  setPipelineName(event: Event): void {
    this.pipelineName.set((event.target as HTMLInputElement).value);
  }
  setPipelineDefault(event: Event): void {
    this.pipelineDefault.set((event.target as HTMLInputElement).checked);
  }
  setStageName(event: Event): void {
    this.stageName.set((event.target as HTMLInputElement).value);
  }
  setStageProbability(event: Event): void {
    this.stageProbability.set(Number((event.target as HTMLInputElement).value));
  }
  setStageForecast(event: Event): void {
    this.stageForecast.set((event.target as HTMLSelectElement).value as ForecastCategory);
  }
  startPipelineCreate(): void {
    this.editingPipeline.set(null);
    this.pipelineName.set('');
    this.pipelineDefault.set(false);
    this.pipelineEditorOpen.set(true);
  }
  startPipelineEdit(pipeline: PipelineRecord): void {
    this.editingPipeline.set(pipeline);
    this.pipelineName.set(pipeline.name);
    this.pipelineDefault.set(pipeline.isDefault);
    this.pipelineEditorOpen.set(true);
  }
  async savePipeline(): Promise<void> {
    const input: PipelineInput = {
      name: this.pipelineName().trim(),
      isDefault: this.pipelineDefault(),
    };
    const current = this.editingPipeline();
    const saved = current
      ? await this.store.updatePipeline(current, input)
      : await this.store.createPipeline(input);
    if (saved) {
      this.pipelineEditorOpen.set(false);
      this.toasts.show({ messageKey: 'pipelines.saved', messageParams: {} });
    }
  }
  async deletePipeline(pipeline: PipelineRecord): Promise<void> {
    if (await this.store.deletePipeline(pipeline))
      this.toasts.show({ messageKey: 'pipelines.deleted', messageParams: {} });
  }
  startStageCreate(pipeline: PipelineRecord): void {
    this.stagePipeline.set(pipeline);
    this.editingStage.set(null);
    this.stageName.set('');
    this.stageProbability.set(20);
    this.stageForecast.set('pipeline');
  }
  startStageEdit(pipeline: PipelineRecord, stage: PipelineStageRecord): void {
    this.stagePipeline.set(pipeline);
    this.editingStage.set(stage);
    this.stageName.set(stage.name);
    this.stageProbability.set(stage.probability);
    this.stageForecast.set(stage.forecastCategory as ForecastCategory);
  }
  closeStageEditor(): void {
    this.stagePipeline.set(null);
    this.editingStage.set(null);
  }
  async saveStage(pipeline: PipelineRecord): Promise<void> {
    const probability = Math.max(0, Math.min(100, Math.round(this.stageProbability())));
    const input: PipelineStageInput = {
      name: this.stageName().trim(),
      probability,
      forecastCategory: this.stageForecast(),
    };
    const current = this.editingStage();
    const saved = current
      ? await this.store.updateStage(current, input)
      : await this.store.createStage(pipeline, input);
    if (saved) {
      this.closeStageEditor();
      this.toasts.show({ messageKey: 'pipelines.saved', messageParams: {} });
    }
  }
  async deleteStage(stage: PipelineStageRecord): Promise<void> {
    if (await this.store.deleteStage(stage))
      this.toasts.show({ messageKey: 'pipelines.deleted', messageParams: {} });
  }
  forecastKey(
    category: string,
  ):
    | 'pipelines.forecast.pipeline'
    | 'pipelines.forecast.best_case'
    | 'pipelines.forecast.commit'
    | 'pipelines.forecast.omitted' {
    return `pipelines.forecast.${category}` as
      | 'pipelines.forecast.pipeline'
      | 'pipelines.forecast.best_case'
      | 'pipelines.forecast.commit'
      | 'pipelines.forecast.omitted';
  }
}
