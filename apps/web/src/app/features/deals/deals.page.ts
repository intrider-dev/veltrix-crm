import { CdkTrapFocus } from '@angular/cdk/a11y';
import {
  CdkDrag,
  CdkDragHandle,
  CdkDragPlaceholder,
  CdkDropList,
  CdkDropListGroup,
  type CdkDragDrop,
} from '@angular/cdk/drag-drop';
import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  Injector,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { FormField, form, min, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';

import type { CreateDeal, Deal } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { DealsStore } from './deals.store';

@Component({
  selector: 'app-deals-page',
  imports: [
    CdkTrapFocus,
    CdkDrag,
    CdkDragHandle,
    CdkDragPlaceholder,
    CdkDropList,
    CdkDropListGroup,
    ErrorPanelComponent,
    FormField,
    FormsModule,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    RouterLink,
  ],
  providers: [DealsStore],
  template: `
    <div class="page deals-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('sales.pipeline.title') }}</h1>
          <p>{{ i18n.plural('common.resultCount', visibleDealCount()) }}</p>
        </div>
        @if (permissions.allows('deals.create')) {
          <button #dealCreateTrigger mat-flat-button type="button" (click)="openCreate()">
            <app-icon name="add" />{{ i18n.t('sales.deal.add') }}
          </button>
        }
      </header>
      <section class="workspace-controls">
        @if (store.pipelines().length > 1) {
          <mat-select
            class="pipeline-select"
            [value]="store.activePipelineId()"
            (selectionChange)="store.selectPipeline($event.value)"
            [attr.aria-label]="i18n.t('sales.deal.pipeline')"
          >
            @for (pipeline of store.pipelines(); track pipeline.id) {
              <mat-option [value]="pipeline.id">{{ pipeline.displayName }}</mat-option>
            }
          </mat-select>
        }
        <nav
          class="view-switcher segmented-control"
          [attr.aria-label]="i18n.t('sales.deal.view.switcher')"
        >
          @for (mode of viewModes; track mode) {
            <button
              mat-button
              type="button"
              [class.active]="store.viewMode() === mode"
              [attr.aria-pressed]="store.viewMode() === mode"
              (click)="store.setViewMode(mode)"
            >
              {{
                i18n.t(
                  mode === 'list'
                    ? 'sales.deal.view.list'
                    : mode === 'gantt'
                      ? 'sales.deal.view.gantt'
                      : 'sales.deal.view.kanban'
                )
              }}
            </button>
          }
        </nav>
      </section>
      @if (store.conflict()) {
        <section class="conflict" role="alert">{{ i18n.t('web.deal.conflict') }}</section>
      }
      @if (store.error() && !store.conflict()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      @if (store.loading() && !store.activePipeline()) {
        <div class="board-skeleton">
          <div class="skeleton"></div>
          <div class="skeleton"></div>
          <div class="skeleton"></div>
        </div>
      } @else if (store.activePipeline(); as pipeline) {
        @if (store.viewMode() === 'kanban') {
          <section
            class="kanban"
            cdkDropListGroup
            tabindex="0"
            [attr.aria-label]="i18n.t('sales.pipeline.title')"
            [attr.aria-busy]="store.saving()"
          >
            @for (stage of pipeline.stages; track stage.id) {
              <article class="stage">
                <header>
                  <div>
                    <h2>{{ stage.displayName }}</h2>
                    <small>{{ stage.probability }}%</small>
                  </div>
                  <span>{{ store.dealsFor(stage.id).length }}</span>
                </header>
                <div
                  class="drop-zone"
                  cdkDropList
                  [id]="stage.id"
                  [cdkDropListData]="store.dealsFor(stage.id)"
                  (cdkDropListDropped)="drop($event, stage.id)"
                >
                  @for (deal of store.dealsFor(stage.id); track deal.id) {
                    <article
                      class="deal-card"
                      cdkDrag
                      [cdkDragData]="deal"
                      [cdkDragDisabled]="!permissions.allows('deals.update')"
                    >
                      <div class="drag-handle" cdkDragHandle aria-hidden="true">···</div>
                      <h3>
                        <a [routerLink]="['/deals', deal.id]">{{ deal.name }}</a>
                      </h3>
                      <strong>{{ i18n.money(deal.amountMinor, deal.currency) }}</strong>
                      @if (deal.expectedCloseDate) {
                        <time [attr.datetime]="deal.expectedCloseDate">{{
                          i18n.date(deal.expectedCloseDate)
                        }}</time>
                      }
                      <label
                        ><span class="visually-hidden">{{
                          i18n.t('web.deal.moveWithKeyboard')
                        }}</span
                        ><select
                          [ngModel]="deal.stageId"
                          [disabled]="!permissions.allows('deals.update')"
                          (ngModelChange)="moveFromSelect(deal, $event)"
                          [attr.aria-label]="i18n.t('web.deal.moveWithKeyboard')"
                        >
                          @for (target of pipeline.stages; track target.id) {
                            <option [value]="target.id">{{ target.displayName }}</option>
                          }
                        </select></label
                      >
                      <div class="deal-placeholder" *cdkDragPlaceholder></div>
                    </article>
                  } @empty {
                    <p class="stage-empty">{{ i18n.t('sales.pipeline.empty') }}</p>
                  }
                  @if (store.nextCursorByStage()[stage.id]) {
                    <button
                      mat-button
                      type="button"
                      class="load-more"
                      (click)="store.loadMore(stage.id)"
                    >
                      {{ i18n.t('sales.deal.loadMore') }}
                    </button>
                  }
                </div>
              </article>
            }
          </section>
        } @else if (store.viewMode() === 'list') {
          <section class="deal-list panel" [attr.aria-busy]="store.loading()">
            <div class="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>{{ i18n.t('common.field.name') }}</th>
                    <th>{{ i18n.t('sales.deal.stage') }}</th>
                    <th>{{ i18n.t('sales.deal.amount') }}</th>
                    <th>{{ i18n.t('sales.deal.plannedStart') }}</th>
                    <th>{{ i18n.t('sales.deal.expectedClose') }}</th>
                  </tr>
                </thead>
                <tbody>
                  @for (deal of store.listDeals(); track deal.id) {
                    <tr>
                      <td>
                        <a [routerLink]="['/deals', deal.id]">{{ deal.name }}</a>
                      </td>
                      <td>
                        <select
                          [ngModel]="deal.stageId"
                          [disabled]="!permissions.allows('deals.update')"
                          (ngModelChange)="moveFromSelect(deal, $event)"
                          [attr.aria-label]="i18n.t('web.deal.moveWithKeyboard')"
                        >
                          @for (stage of pipeline.stages; track stage.id) {
                            <option [value]="stage.id">{{ stage.displayName }}</option>
                          }
                        </select>
                      </td>
                      <td>{{ i18n.money(deal.amountMinor, deal.currency) }}</td>
                      <td>{{ deal.plannedStartDate ? i18n.date(deal.plannedStartDate) : '—' }}</td>
                      <td>
                        {{ deal.expectedCloseDate ? i18n.date(deal.expectedCloseDate) : '—' }}
                      </td>
                    </tr>
                  } @empty {
                    <tr>
                      <td colspan="5" class="table-empty">{{ i18n.t('sales.deal.list.empty') }}</td>
                    </tr>
                  }
                </tbody>
              </table>
            </div>
            @if (store.listNextCursor()) {
              <button mat-button type="button" class="load-more" (click)="store.loadMoreList()">
                {{ i18n.t('sales.deal.loadMore') }}
              </button>
            }
          </section>
        } @else {
          <section class="gantt panel" [attr.aria-busy]="store.loading()">
            <header class="gantt-range">
              <time [attr.datetime]="timelineBounds().start">{{
                i18n.date(timelineBounds().start)
              }}</time>
              <span>{{ i18n.t('sales.deal.list.loaded') }}: {{ store.listDeals().length }}</span>
              <time [attr.datetime]="timelineBounds().end">{{
                i18n.date(timelineBounds().end)
              }}</time>
            </header>
            <div class="gantt-grid">
              @for (deal of scheduledDeals(); track deal.id) {
                <a class="gantt-label" [routerLink]="['/deals', deal.id]">{{ deal.name }}</a>
                <div class="gantt-track">
                  <a
                    class="gantt-bar"
                    [routerLink]="['/deals', deal.id]"
                    [style.inset-inline-start.%]="ganttStart(deal)"
                    [style.width.%]="ganttWidth(deal)"
                    [attr.aria-label]="deal.name"
                  >
                    <span>{{ i18n.money(deal.amountMinor, deal.currency) }}</span>
                  </a>
                </div>
              } @empty {
                <p class="gantt-empty">{{ i18n.t('sales.deal.list.empty') }}</p>
              }
            </div>
            @if (unscheduledDeals().length) {
              <details class="unscheduled">
                <summary>
                  {{ i18n.t('sales.deal.unscheduled') }} · {{ unscheduledDeals().length }}
                </summary>
                <ul>
                  @for (deal of unscheduledDeals(); track deal.id) {
                    <li>
                      <a [routerLink]="['/deals', deal.id]">{{ deal.name }}</a>
                    </li>
                  }
                </ul>
              </details>
            }
            @if (store.listNextCursor()) {
              <button mat-button type="button" class="load-more" (click)="store.loadMoreList()">
                {{ i18n.t('sales.deal.loadMore') }}
              </button>
            }
          </section>
        }
      } @else if (!store.loading()) {
        <div class="empty-state panel">{{ i18n.t('web.deal.emptyBoard') }}</div>
      }
    </div>

    @if (createOpen() && store.activePipeline(); as pipeline) {
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
        [attr.aria-labelledby]="'new-deal-title'"
      >
        <header>
          <h2 id="new-deal-title">{{ i18n.t('sales.deal.createTitle') }}</h2>
          <button
            mat-icon-button
            type="button"
            (click)="closeCreate()"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>
        <form (submit)="create($event)">
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('common.field.name') }}</mat-label
            ><input #dealNameInput matInput [formField]="dealForm.name"
          /></mat-form-field>
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('sales.deal.amount') }}</mat-label
            ><input matInput type="number" [formField]="dealForm.amount"
          /></mat-form-field>
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('sales.deal.currency') }}</mat-label
            ><input matInput [formField]="dealForm.currency"
          /></mat-form-field>
          <label class="native-field"
            >{{ i18n.t('sales.deal.stage')
            }}<select [formField]="dealForm.stageId">
              @for (stage of pipeline.stages; track stage.id) {
                <option [value]="stage.id">{{ stage.displayName }}</option>
              }
            </select></label
          >
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('sales.deal.plannedStart') }}</mat-label
            ><input matInput type="date" [formField]="dealForm.plannedStartDate"
          /></mat-form-field>
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('sales.deal.expectedClose') }}</mat-label
            ><input matInput type="date" [formField]="dealForm.expectedCloseDate"
          /></mat-form-field>
          <div class="drawer-actions">
            <button mat-button type="button" (click)="closeCreate()">
              {{ i18n.t('common.action.cancel') }}</button
            ><button mat-flat-button type="submit" [disabled]="store.saving()">
              {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
            </button>
          </div>
        </form>
      </aside>
    }
  `,
  styleUrl: './deals.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DealsPage implements OnInit {
  readonly store = inject(DealsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly viewModes = ['list', 'kanban', 'gantt'] as const;
  readonly createOpen = signal(false);
  readonly visibleDealCount = computed(() =>
    this.store.viewMode() === 'kanban' ? this.store.deals().length : this.store.listDeals().length,
  );
  readonly scheduledDeals = computed(() =>
    this.store
      .listDeals()
      .filter((deal) => Boolean(deal.plannedStartDate && deal.expectedCloseDate)),
  );
  readonly unscheduledDeals = computed(() =>
    this.store.listDeals().filter((deal) => !deal.plannedStartDate || !deal.expectedCloseDate),
  );
  readonly timelineBounds = computed(() => {
    const starts = this.scheduledDeals().map((deal) => dateValue(deal.plannedStartDate!));
    const ends = this.scheduledDeals().map((deal) => dateValue(deal.expectedCloseDate!));
    const today = Date.now();
    return {
      start: isoDate(starts.length ? Math.min(...starts) : today),
      end: isoDate(ends.length ? Math.max(...ends) : today + 30 * dayMilliseconds),
    };
  });
  readonly dealModel = signal({
    name: '',
    amount: 0,
    currency: 'USD',
    stageId: '',
    plannedStartDate: '',
    expectedCloseDate: '',
  });
  readonly dealForm = form(this.dealModel, (schema) => {
    required(schema.name);
    min(schema.amount, 0);
    required(schema.currency);
    required(schema.stageId);
  });
  readonly dealCreateTrigger = viewChild<ElementRef<HTMLButtonElement>>('dealCreateTrigger');
  readonly dealNameInput = viewChild<ElementRef<HTMLInputElement>>('dealNameInput');
  private readonly injector = inject(Injector);

  ngOnInit(): void {
    void this.initialize();
  }
  async initialize(): Promise<void> {
    await this.store.load();
    const stage = this.store.activePipeline()?.stages[0];
    if (stage) this.dealModel.update((value) => ({ ...value, stageId: stage.id }));
  }
  openCreate(): void {
    this.createOpen.set(true);
    focusAfterNextRender(this.injector, () => this.dealNameInput()?.nativeElement);
  }
  closeCreate(): void {
    this.createOpen.set(false);
    focusAfterNextRender(this.injector, () => this.dealCreateTrigger()?.nativeElement);
  }
  @HostListener('document:keydown.escape')
  closeCreateFromKeyboard(): void {
    if (this.createOpen()) this.closeCreate();
  }
  drop(event: CdkDragDrop<readonly Deal[]>, stageId: string): void {
    if (!this.permissions.allows('deals.update')) return;
    const deal = event.item.data as Deal;
    void this.store.move(deal.id, stageId, event.currentIndex);
  }
  moveFromSelect(deal: Deal, stageId: string): void {
    if (!this.permissions.allows('deals.update')) return;
    void this.store.move(deal.id, stageId, 0);
  }
  ganttStart(deal: Deal): number {
    const start = dateValue(this.timelineBounds().start);
    const end = dateValue(this.timelineBounds().end);
    return (
      ((dateValue(deal.plannedStartDate!) - start) / Math.max(end - start, dayMilliseconds)) * 100
    );
  }
  ganttWidth(deal: Deal): number {
    const start = dateValue(this.timelineBounds().start);
    const end = dateValue(this.timelineBounds().end);
    const range = Math.max(end - start, dayMilliseconds);
    const duration = Math.max(
      dateValue(deal.expectedCloseDate!) - dateValue(deal.plannedStartDate!),
      dayMilliseconds,
    );
    return Math.max((duration / range) * 100, 2);
  }
  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.dealForm().invalid()) {
      this.dealForm().markAsTouched();
      return;
    }
    const value = this.dealModel();
    const pipeline = this.store.activePipeline();
    if (!pipeline) return;
    const body: CreateDeal = {
      name: value.name.trim(),
      pipelineId: pipeline.id,
      stageId: value.stageId,
      amountMinor: Math.round(value.amount * 100),
      currency: value.currency.trim().toUpperCase(),
      plannedStartDate: value.plannedStartDate || null,
      expectedCloseDate: value.expectedCloseDate || null,
    };
    await this.store.create(body);
    this.closeCreate();
    this.dealModel.set({
      name: '',
      amount: 0,
      currency: 'USD',
      stageId: pipeline.stages[0]?.id ?? '',
      plannedStartDate: '',
      expectedCloseDate: '',
    });
  }
}

const dayMilliseconds = 86_400_000;

function dateValue(value: string): number {
  return Date.parse(`${value}T00:00:00Z`);
}

function isoDate(value: number): string {
  return new Date(value).toISOString().slice(0, 10);
}
