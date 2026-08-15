import { CdkTrapFocus } from '@angular/cdk/a11y';
import {
  CdkDrag,
  CdkDragHandle,
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
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { FormField, email, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';

import type { Lead, LeadStage, LeadStatus } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { trimmedOrNull } from '../../shared/forms/feature-validation';
import { PhoneInputComponent } from '../../shared/forms/phone-input.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { LeadsStore } from './leads.store';

@Component({
  selector: 'app-leads-page',
  imports: [
    CdkTrapFocus,
    CdkDrag,
    CdkDragHandle,
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
    PhoneInputComponent,
    RecordAssignmentsComponent,
    RouterLink,
  ],
  providers: [LeadsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <div class="title-line">
            <h1>{{ i18n.t('common.nav.leads') }}</h1>
            <span class="loaded-count">{{ store.leads().length }}</span>
          </div>
          <p>{{ i18n.t('leads.subtitle') }}</p>
        </div>
        <form class="header-search" (submit)="filter($event)" role="search">
          <app-icon name="search" />
          <label class="visually-hidden" for="lead-search">{{ i18n.t('leads.search') }}</label>
          <input
            id="lead-search"
            type="search"
            [placeholder]="i18n.t('leads.search')"
            [value]="store.query()"
            (input)="store.query.set(inputValue($event))"
          />
          <kbd>/</kbd>
        </form>
        <div class="header-actions">
          @if (permissions.allows('leads.create')) {
            <button #leadCreateTrigger mat-flat-button type="button" (click)="openCreate()">
              <app-icon name="add" />{{ i18n.t('leads.add') }}
            </button>
          }
        </div>
      </header>

      <section class="stage-overview" [attr.aria-label]="i18n.t('leads.overviewLoaded')">
        @for (stage of store.stages(); track stage.id; let index = $index) {
          <button
            type="button"
            class="stage-summary"
            [class.active]="store.stageId() === stage.id"
            [style.--stage-index]="index"
            (click)="selectStage(stage.id)"
          >
            <span>{{ stageLabel(stage) }}</span>
            <strong>{{ store.leadsFor(stage.id).length }}</strong>
            <small>{{ stageShare(stage.id) }}%</small>
            <i aria-hidden="true"><b [style.width.%]="stageShare(stage.id)"></b></i>
          </button>
        }
      </section>

      @if (store.loadError()) {
        <app-error-panel [error]="store.loadError()" (retry)="store.load()" />
      }
      @if (store.stageError()) {
        <app-error-panel [error]="store.stageError()" (retry)="retryStages()" />
      }

      @if (createOpen()) {
        <button
          class="entity-drawer-scrim"
          type="button"
          (click)="closeCreate()"
          [attr.aria-label]="i18n.t('common.action.close')"
        ></button>
        <aside
          class="entity-create-drawer"
          role="dialog"
          aria-modal="true"
          cdkTrapFocus
          [cdkTrapFocusAutoCapture]="true"
          aria-labelledby="new-lead-title"
        >
          <header>
            <h2 id="new-lead-title">{{ i18n.t('leads.createTitle') }}</h2>
            <button
              mat-icon-button
              type="button"
              (click)="closeCreate()"
              [attr.aria-label]="i18n.t('common.action.close')"
            >
              <app-icon name="close" />
            </button>
          </header>
          <form class="entity-create-form" (submit)="create($event)" novalidate>
            <div class="entity-create-body">
              @if (store.formError()) {
                <app-error-panel
                  class="form-error"
                  [error]="store.formError()"
                  [retryable]="false"
                />
              }
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
                <input #nameInput matInput [formField]="leadForm.name" autocomplete="off" />
                @if (leadForm.name().touched() && leadForm.name().invalid()) {
                  <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
                }
              </mat-form-field>
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('common.field.email') }}</mat-label>
                <input matInput type="email" [formField]="leadForm.email" inputmode="email" />
                @if (leadForm.email().touched() && leadForm.email().invalid()) {
                  <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
                }
              </mat-form-field>
              <app-phone-input
                [formField]="leadForm.phone"
                [label]="i18n.t('common.field.phone')"
                (validityChange)="phoneValid.set($event)"
              />
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('leads.company') }}</mat-label>
                <input matInput [formField]="leadForm.companyName" />
              </mat-form-field>
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('leads.jobTitle') }}</mat-label>
                <input matInput [formField]="leadForm.jobTitle" />
              </mat-form-field>
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('leads.source') }}</mat-label>
                <input matInput [formField]="leadForm.source" />
              </mat-form-field>
              <label class="native-field">
                <span>{{ i18n.t('leads.stage') }}</span>
                <mat-select
                  panelClass="crm-select-panel"
                  [aria-label]="i18n.t('leads.stage')"
                  [formField]="leadForm.stageId"
                >
                  @for (stage of editableStages(); track stage.id) {
                    <mat-option [value]="stage.id">{{ stageLabel(stage) }}</mat-option>
                  }
                </mat-select>
              </label>
            </div>
            <footer class="entity-create-actions">
              <button mat-button type="button" (click)="closeCreate()">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button mat-flat-button type="submit" [disabled]="store.saving()">
                {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
              </button>
            </footer>
          </form>
        </aside>
      }

      @if (store.viewMode() === 'list') {
        <div class="leads-workspace">
          <section class="panel record-list" [attr.aria-busy]="store.loading()">
            <header class="list-toolbar">
              <nav
                class="view-switcher segmented-control"
                [attr.aria-label]="i18n.t('leads.view.switcher')"
              >
                @for (mode of viewModes; track mode) {
                  <button
                    mat-button
                    type="button"
                    [class.active]="store.viewMode() === mode"
                    [attr.aria-pressed]="store.viewMode() === mode"
                    (click)="store.setViewMode(mode)"
                  >
                    {{ viewModeLabel(mode) }}
                  </button>
                }
              </nav>
              <span>{{ i18n.t('leads.loadedCount', { count: store.leads().length }) }}</span>
            </header>
            @if (store.loading() && store.leads().length === 0) {
              <div class="list-skeleton" [attr.aria-label]="i18n.t('common.result.loading')">
                <div class="skeleton"></div>
                <div class="skeleton"></div>
                <div class="skeleton"></div>
              </div>
            } @else if (store.leads().length === 0) {
              <div class="empty-state">{{ i18n.t('leads.empty') }}</div>
            } @else {
              <div class="lead-table-wrap">
                <table class="lead-table">
                  <thead>
                    <tr>
                      <th>{{ i18n.t('leads.table.lead') }}</th>
                      <th>{{ i18n.t('leads.stage') }}</th>
                      <th>{{ i18n.t('leads.source') }}</th>
                      <th>{{ i18n.t('leads.table.contact') }}</th>
                      <th>{{ i18n.t('leads.table.created') }}</th>
                      <th>{{ i18n.t('leads.table.nextDate') }}</th>
                      <th>
                        <span class="visually-hidden">{{ i18n.t('leads.table.actions') }}</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    @for (lead of store.leads(); track lead.id) {
                      <tr>
                        <td>
                          <a
                            class="lead-identity"
                            [routerLink]="['/leads', lead.id]"
                            [attr.aria-label]="lead.name"
                          >
                            <span class="record-avatar" aria-hidden="true">{{
                              lead.name.charAt(0)
                            }}</span>
                            <span
                              ><strong>{{ lead.name }}</strong
                              ><small>{{
                                lead.companyName || lead.jobTitle || i18n.t('leads.noDetails')
                              }}</small></span
                            >
                          </a>
                        </td>
                        <td>
                          @if (permissions.allows('leads.update') && lead.status !== 'converted') {
                            <label class="visually-hidden" [for]="'lead-status-' + lead.id">{{
                              i18n.t('leads.stage')
                            }}</label>
                            <mat-select
                              panelClass="crm-select-panel"
                              class="stage-select"
                              [aria-label]="i18n.t('leads.stage')"
                              [id]="'lead-status-' + lead.id"
                              [ngModel]="lead.stageId"
                              [disabled]="store.isMoving(lead.id)"
                              (ngModelChange)="moveFromSelect(lead, $event)"
                            >
                              @for (stage of editableStages(); track stage.id) {
                                <mat-option [value]="stage.id">{{ stageLabel(stage) }}</mat-option>
                              }
                            </mat-select>
                          } @else {
                            <span class="status-pill">{{ leadStageLabel(lead) }}</span>
                          }
                        </td>
                        <td>
                          <span class="source-cell">{{ lead.source || '—' }}</span>
                        </td>
                        <td>
                          <span class="contact-cell">{{ lead.email || lead.phone || '—' }}</span>
                        </td>
                        <td>
                          <time [attr.datetime]="lead.createdAt">{{
                            i18n.date(lead.createdAt)
                          }}</time>
                        </td>
                        <td>
                          @if (lead.expectedCloseDate || lead.plannedStartDate) {
                            <time
                              [attr.datetime]="lead.expectedCloseDate || lead.plannedStartDate"
                              >{{
                                i18n.date(lead.expectedCloseDate ?? lead.plannedStartDate ?? '')
                              }}</time
                            >
                          } @else {
                            —
                          }
                        </td>
                        <td class="row-actions">
                          <a mat-button [routerLink]="['/leads', lead.id]">{{
                            i18n.t('leads.table.open')
                          }}</a>
                          <button mat-button type="button" (click)="selectedLead.set(lead)">
                            {{ i18n.t('leads.table.assign') }}
                          </button>
                        </td>
                      </tr>
                    }
                  </tbody>
                </table>
              </div>
            }
          </section>

          <aside class="filter-panel" [attr.aria-label]="i18n.t('leads.filters.title')">
            <header>
              <h2>{{ i18n.t('leads.filters.title') }}</h2>
              <button mat-button type="button" (click)="clearFilters()">
                {{ i18n.t('leads.filters.reset') }}
              </button>
            </header>
            <div class="quick-filters">
              <span>{{ i18n.t('leads.filters.quick') }}</span>
              @for (status of filterStatuses; track status) {
                <button
                  type="button"
                  [class.active]="store.status() === status"
                  (click)="toggleStatus(status)"
                >
                  <span class="status-dot" [attr.data-status]="status"></span>
                  {{ i18n.t(statusKey(status)) }}
                  <b>{{ statusCount(status) }}</b>
                </button>
              }
            </div>
            <label class="native-field">
              <span>{{ i18n.t('common.field.status') }}</span>
              <mat-select
                panelClass="crm-select-panel"
                [aria-label]="i18n.t('common.field.status')"
                [value]="store.status()"
                (selectionChange)="setFilterStatus($event.value)"
              >
                <mat-option value="">{{ i18n.t('leads.status.all') }}</mat-option>
                @for (status of filterStatuses; track status) {
                  <mat-option [value]="status">{{ i18n.t(statusKey(status)) }}</mat-option>
                }
              </mat-select>
            </label>
            <label class="native-field">
              <span>{{ i18n.t('leads.stage') }}</span>
              <mat-select
                panelClass="crm-select-panel"
                [aria-label]="i18n.t('leads.stage')"
                [value]="store.stageId()"
                (selectionChange)="setFilterStage($event.value)"
              >
                <mat-option value="">{{ i18n.t('leads.filters.allStages') }}</mat-option>
                @for (stage of store.stages(); track stage.id) {
                  <mat-option [value]="stage.id">{{ stageLabel(stage) }}</mat-option>
                }
              </mat-select>
            </label>
            <p>{{ i18n.t('leads.filters.serverHint') }}</p>
            <button class="apply-filters" mat-flat-button type="button" (click)="applyFilters()">
              {{ i18n.t('leads.filters.apply') }}
            </button>
          </aside>
        </div>
      } @else if (store.viewMode() === 'kanban') {
        <nav
          class="view-switcher floating-view-switcher segmented-control"
          [attr.aria-label]="i18n.t('leads.view.switcher')"
        >
          @for (mode of viewModes; track mode) {
            <button
              mat-button
              type="button"
              [class.active]="store.viewMode() === mode"
              [attr.aria-pressed]="store.viewMode() === mode"
              (click)="store.setViewMode(mode)"
            >
              {{ viewModeLabel(mode) }}
            </button>
          }
        </nav>
        <section class="lead-kanban" cdkDropListGroup [attr.aria-busy]="store.loading()">
          @for (stage of store.boardStages(); track stage.id) {
            <article class="lead-stage">
              <header>
                <h2>{{ stageLabel(stage) }}</h2>
                <span>{{ store.leadsFor(stage.id).length }}</span>
              </header>
              <div
                class="lead-drop-zone"
                cdkDropList
                [id]="stage.id"
                [cdkDropListData]="store.leadsFor(stage.id)"
                [cdkDropListDisabled]="stage.category === 'converted'"
                (cdkDropListDropped)="drop($event, stage)"
              >
                @for (lead of store.leadsFor(stage.id); track lead.id) {
                  <article
                    class="lead-card"
                    cdkDrag
                    [cdkDragData]="lead"
                    cdkDragPreviewClass="crm-kanban-drag-preview"
                    [cdkDragDisabled]="
                      !permissions.allows('leads.update') || lead.status === 'converted'
                    "
                  >
                    <button
                      class="drag-handle"
                      type="button"
                      cdkDragHandle
                      tabindex="-1"
                      aria-hidden="true"
                    >
                      &#8942;&#8942;
                    </button>
                    <h3>
                      <a [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
                    </h3>
                    <p>{{ lead.companyName || lead.email || i18n.t('leads.noDetails') }}</p>
                    @if (lead.expectedCloseDate) {
                      <time [attr.datetime]="lead.expectedCloseDate">{{
                        i18n.date(lead.expectedCloseDate)
                      }}</time>
                    }
                    @if (lead.status !== 'converted') {
                      <label class="visually-hidden" [for]="'kanban-stage-' + lead.id">{{
                        i18n.t('leads.stage')
                      }}</label>
                      <mat-select
                        panelClass="crm-select-panel"
                        [aria-label]="i18n.t('leads.stage')"
                        [id]="'kanban-stage-' + lead.id"
                        [ngModel]="lead.stageId"
                        [disabled]="!permissions.allows('leads.update')"
                        (ngModelChange)="moveFromSelect(lead, $event)"
                      >
                        @for (target of editableStages(); track target.id) {
                          <mat-option [value]="target.id">{{ stageLabel(target) }}</mat-option>
                        }
                      </mat-select>
                    } @else {
                      <span class="status-pill">{{ leadStageLabel(lead) }}</span>
                    }
                  </article>
                } @empty {
                  <p class="stage-empty">{{ i18n.t('leads.emptyStage') }}</p>
                }
                @if (store.nextCursorByStage()[stage.id]) {
                  <button
                    mat-button
                    type="button"
                    class="load-more"
                    [disabled]="store.loading()"
                    (click)="store.loadMoreStage(stage.id)"
                  >
                    {{ i18n.t('leads.loadMore') }}
                  </button>
                }
              </div>
            </article>
          }
        </section>
      } @else {
        <nav
          class="view-switcher floating-view-switcher segmented-control"
          [attr.aria-label]="i18n.t('leads.view.switcher')"
        >
          @for (mode of viewModes; track mode) {
            <button
              mat-button
              type="button"
              [class.active]="store.viewMode() === mode"
              [attr.aria-pressed]="store.viewMode() === mode"
              (click)="store.setViewMode(mode)"
            >
              {{ viewModeLabel(mode) }}
            </button>
          }
        </nav>
        <section class="panel lead-gantt" [attr.aria-busy]="store.loading()">
          <header class="gantt-range">
            <time>{{ i18n.date(timelineBounds().start) }}</time
            ><span>{{ store.leads().length }}</span
            ><time>{{ i18n.date(timelineBounds().end) }}</time>
          </header>
          <div class="gantt-grid">
            @for (lead of store.scheduledLeads(); track lead.id) {
              <a class="gantt-label" [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
              <div class="gantt-track">
                <a
                  class="gantt-bar"
                  [routerLink]="['/leads', lead.id]"
                  [style.inset-inline-start.%]="ganttStart(lead)"
                  [style.width.%]="ganttWidth(lead)"
                  ><span>{{ leadStageLabel(lead) }}</span></a
                >
              </div>
            } @empty {
              <p class="gantt-empty">{{ i18n.t('leads.unscheduled') }}</p>
            }
          </div>
          @if (unscheduledLeads().length) {
            <details class="unscheduled">
              <summary>{{ i18n.t('leads.unscheduled') }} · {{ unscheduledLeads().length }}</summary>
              <ul>
                @for (lead of unscheduledLeads(); track lead.id) {
                  <li>
                    <a [routerLink]="['/leads', lead.id]">{{ lead.name }}</a>
                  </li>
                }
              </ul>
            </details>
          }
        </section>
      }
      @if (selectedLead(); as lead) {
        <section class="panel assignment-panel">
          <app-record-assignments
            resourceType="lead"
            [resourceId]="lead.id"
            [version]="lead.version"
            (versionChange)="assignmentVersionChanged(lead, $event)"
          />
        </section>
      }
      @if (store.nextCursor()) {
        <button
          mat-stroked-button
          type="button"
          [disabled]="store.loading()"
          (click)="store.load(false)"
        >
          {{ i18n.t('leads.loadMore') }}
        </button>
      }
    </div>
  `,
  styleUrl: './leads.page.scss',
  styles: `
    .record-list article {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 0.85rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .record-list article:last-child {
      border: 0;
    }
    .assignment-panel {
      padding: 1rem;
    }
    .view-switcher {
      width: fit-content;
    }
    .lead-kanban {
      display: grid;
      grid-auto-columns: minmax(17rem, 1fr);
      grid-auto-flow: column;
      gap: 1rem;
      overflow-x: auto;
      padding-bottom: 0.5rem;
    }
    .lead-stage {
      min-width: 17rem;
      border: 1px solid var(--border);
      border-radius: var(--panel-radius);
      background: var(--surface-subtle);
    }
    .lead-stage > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0.8rem;
    }
    .lead-drop-zone {
      min-height: 10rem;
      padding: 0 0.65rem 0.65rem;
    }
    .lead-card {
      position: relative;
      margin-bottom: 0.6rem;
      padding: 0.8rem;
      border: 1px solid var(--border);
      border-radius: 0.65rem;
      background: var(--surface-raised);
    }
    .lead-card h3 a {
      color: inherit;
      text-decoration: none;
    }
    .lead-card h3 a:hover {
      text-decoration: underline;
      text-underline-offset: 0.18em;
    }
    .lead-card h3 {
      margin: 0;
      font-size: 0.9rem;
    }
    .lead-card p {
      margin: 0.35rem 0;
    }
    .lead-card time {
      display: block;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .lead-card .mat-mdc-select {
      width: 100%;
      margin-top: 0.65rem;
    }
    .drag-handle {
      position: absolute;
      inset: 0.25rem 0.25rem auto auto;
      display: grid;
      width: 1.75rem;
      height: 1.75rem;
      padding: 0;
      place-items: center;
      border: 0;
      border-radius: 0.4rem;
      color: var(--text-muted);
      background: transparent;
      cursor: grab;
      line-height: 1;
    }
    .drag-handle:active {
      cursor: grabbing;
    }
    .lead-card.cdk-drag-placeholder {
      border-color: color-mix(in srgb, var(--brand) 72%, var(--border));
      background: color-mix(in srgb, var(--brand) 8%, var(--surface-raised));
      box-shadow: none;
      opacity: 0.72;
    }
    .lead-card.cdk-drag-placeholder > * {
      visibility: hidden;
    }
    .lead-card.cdk-drag-animating,
    .lead-drop-zone.cdk-drop-list-dragging .lead-card:not(.cdk-drag-placeholder) {
      transition: transform var(--motion-standard) var(--ease-out);
    }
    .stage-empty {
      padding: 1rem;
      color: var(--text-muted);
      text-align: center;
    }
    @media (prefers-reduced-motion: reduce) {
      .lead-card.cdk-drag-animating,
      .lead-drop-zone.cdk-drop-list-dragging .lead-card:not(.cdk-drag-placeholder) {
        transition-duration: 1ms;
      }
    }
    .lead-gantt {
      padding: 1rem;
      overflow: hidden;
    }
    .gantt-range {
      display: flex;
      justify-content: space-between;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .gantt-grid {
      display: grid;
      grid-template-columns: minmax(9rem, 15rem) minmax(30rem, 1fr);
      align-items: center;
      gap: 0.5rem 1rem;
      margin-top: 1rem;
      overflow-x: auto;
    }
    .gantt-label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .gantt-track {
      position: relative;
      height: 2rem;
      border-radius: 0.4rem;
      background: var(--surface-subtle);
    }
    .gantt-bar {
      position: absolute;
      inset-block: 0.2rem;
      display: flex;
      align-items: center;
      min-width: 2%;
      overflow: hidden;
      padding: 0 0.5rem;
      border-radius: 0.35rem;
      color: var(--brand-contrast);
      background: var(--brand);
      font-size: 0.72rem;
      text-decoration: none;
    }
    .gantt-bar span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .gantt-empty {
      grid-column: 1 / -1;
      color: var(--text-muted);
    }
    .unscheduled {
      margin-top: 1rem;
    }
    .record-main,
    .record-meta {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      min-width: 0;
    }
    .record-avatar {
      display: grid;
      width: 2.25rem;
      height: 2.25rem;
      flex: 0 0 auto;
      place-items: center;
      border-radius: 0.6rem;
      color: var(--brand);
      background: var(--brand-soft);
      font-weight: 700;
    }
    h2 {
      margin: 0;
      font-size: 0.9rem;
    }
    article p {
      margin: 0.2rem 0 0;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    article .mat-mdc-select {
      min-height: 2.25rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      color: var(--text);
      background: var(--surface-raised);
    }
    @media (max-width: 760px) {
      .record-list article {
        align-items: stretch;
        flex-direction: column;
      }
      .record-meta {
        justify-content: space-between;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LeadsPage implements OnInit {
  readonly store = inject(LeadsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly selectedLead = signal<Lead | null>(null);
  readonly statuses = ['new', 'qualified', 'disqualified'] as const;
  readonly filterStatuses = ['new', 'qualified', 'disqualified', 'converted'] as const;
  readonly viewModes = ['list', 'kanban', 'gantt'] as const;
  readonly phoneValid = signal(true);
  readonly model = signal({
    name: '',
    email: '',
    phone: '',
    companyName: '',
    jobTitle: '',
    source: '',
    stageId: '',
  });
  readonly leadForm = form(this.model, (schema) => {
    required(schema.name);
    email(schema.email);
    validate(schema.phone, ({ value }) =>
      !value() || this.phoneValid()
        ? undefined
        : { kind: 'phone', message: this.i18n.t('common.validation.phone') },
    );
  });
  readonly nameInput = viewChild<ElementRef<HTMLInputElement>>('nameInput');
  readonly leadCreateTrigger = viewChild<ElementRef<HTMLButtonElement>>('leadCreateTrigger');
  private readonly injector = inject(Injector);

  ngOnInit(): void {
    void this.initialize();
  }

  openCreate(): void {
    this.store.formError.set(null);
    this.phoneValid.set(true);
    this.leadForm().reset();
    this.createOpen.set(true);
    focusAfterNextRender(this.injector, () => this.nameInput()?.nativeElement);
  }

  closeCreate(): void {
    this.createOpen.set(false);
    this.leadForm().reset();
    focusAfterNextRender(this.injector, () => this.leadCreateTrigger()?.nativeElement);
  }

  @HostListener('document:keydown.escape')
  closeCreateFromKeyboard(): void {
    if (this.createOpen()) this.closeCreate();
  }

  filter(event: Event): void {
    event.preventDefault();
    void this.store.load();
  }

  async retryStages(): Promise<void> {
    await this.store.loadStages();
    this.model.update((value) => ({ ...value, stageId: this.defaultStageId() }));
    if (!this.store.stageError()) await this.store.load();
  }

  inputValue(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }

  setFilterStatus(status: LeadStatus | ''): void {
    this.store.status.set(status);
  }

  setFilterStage(stageId: string): void {
    this.store.stageId.set(stageId);
  }

  applyFilters(): void {
    void this.store.load(true);
  }

  clearFilters(): void {
    this.store.query.set('');
    this.store.status.set('');
    this.store.stageId.set('');
    void this.store.load(true);
  }

  toggleStatus(status: LeadStatus): void {
    this.store.status.update((current) => (current === status ? '' : status));
    void this.store.load(true);
  }

  selectStage(stageId: string): void {
    this.store.stageId.update((current) => (current === stageId ? '' : stageId));
    void this.store.load(true);
  }

  statusCount(status: LeadStatus): number {
    return this.store.leads().filter((lead) => lead.status === status).length;
  }

  stageShare(stageId: string): number {
    const total = this.store.leads().length;
    return total ? Math.round((this.store.leadsFor(stageId).length / total) * 100) : 0;
  }

  viewModeLabel(mode: (typeof this.viewModes)[number]): string {
    return this.i18n.t(
      mode === 'list'
        ? 'leads.view.list'
        : mode === 'gantt'
          ? 'leads.view.gantt'
          : 'leads.view.kanban',
    );
  }

  changeStage(lead: Lead, event: Event): void {
    const stage = this.store
      .stages()
      .find((candidate) => candidate.id === (event.target as HTMLSelectElement).value);
    if (stage) void this.store.changeStage(lead, stage);
  }

  drop(event: CdkDragDrop<readonly Lead[]>, stage: LeadStage): void {
    const lead = event.item.data as Lead;
    if (lead.stageId !== stage.id) void this.store.changeStage(lead, stage);
  }

  moveFromSelect(lead: Lead, stageId: string): void {
    const stage = this.store.stages().find((candidate) => candidate.id === stageId);
    if (stage) void this.store.changeStage(lead, stage);
  }

  unscheduledLeads(): readonly Lead[] {
    return this.store.leads().filter((lead) => !lead.plannedStartDate && !lead.expectedCloseDate);
  }

  timelineBounds(): { start: string; end: string } {
    const dates = this.store
      .scheduledLeads()
      .flatMap((lead) => [lead.plannedStartDate, lead.expectedCloseDate])
      .filter((value): value is string => Boolean(value))
      .map((value) => new Date(`${value}T00:00:00`).getTime());
    const now = new Date();
    const start = dates.length ? Math.min(...dates) : now.getTime();
    const end = dates.length ? Math.max(...dates) : start + 86400000;
    return {
      start: new Date(start).toISOString(),
      end: new Date(Math.max(end, start + 86400000)).toISOString(),
    };
  }

  ganttStart(lead: Lead): number {
    const bounds = this.timelineBounds();
    const start = new Date(`${lead.plannedStartDate ?? lead.expectedCloseDate}T00:00:00`).getTime();
    return (
      ((start - new Date(bounds.start).getTime()) /
        (new Date(bounds.end).getTime() - new Date(bounds.start).getTime())) *
      100
    );
  }

  ganttWidth(lead: Lead): number {
    const bounds = this.timelineBounds();
    const start = new Date(`${lead.plannedStartDate ?? lead.expectedCloseDate}T00:00:00`).getTime();
    const end = new Date(`${lead.expectedCloseDate ?? lead.plannedStartDate}T00:00:00`).getTime();
    return Math.max(
      2,
      ((Math.max(end, start) - start + 86400000) /
        (new Date(bounds.end).getTime() - new Date(bounds.start).getTime())) *
        100,
    );
  }

  editableStages(): readonly LeadStage[] {
    return this.store
      .stages()
      .filter((stage) => stage.category !== 'converted' && stage.systemKey !== 'converted');
  }

  stageLabel(stage: LeadStage): string {
    return stage.systemKey ? this.i18n.t(this.statusKey(stage.systemKey)) : stage.displayName;
  }

  leadStageLabel(lead: Lead): string {
    const stage = this.store.stages().find((candidate) => candidate.id === lead.stageId);
    return stage ? this.stageLabel(stage) : this.i18n.t(this.statusKey(lead.status));
  }

  assignmentVersionChanged(lead: Lead, version: number): void {
    this.store.setVersion(lead.id, version);
    this.selectedLead.update((current) =>
      current?.id === lead.id ? { ...current, version } : current,
    );
  }

  statusKey(
    status: LeadStatus,
  ):
    | 'leads.status.new'
    | 'leads.status.qualified'
    | 'leads.status.disqualified'
    | 'leads.status.converted' {
    return `leads.status.${status}`;
  }

  async create(event: Event): Promise<void> {
    event.preventDefault();
    this.leadForm().markAsTouched();
    if (this.leadForm().invalid() || (!!this.model().phone && !this.phoneValid())) return;
    const value = this.model();
    const stage = this.store.stages().find((candidate) => candidate.id === value.stageId);
    if (!stage || stage.category === 'converted') return;
    const created = await this.store.create({
      name: value.name.trim(),
      email: trimmedOrNull(value.email),
      phone: trimmedOrNull(value.phone),
      companyName: trimmedOrNull(value.companyName),
      jobTitle: trimmedOrNull(value.jobTitle),
      source: trimmedOrNull(value.source),
      status: stage.category,
      stageId: stage.id,
      customFields: {},
    });
    if (!created) return;
    this.model.set({
      name: '',
      email: '',
      phone: '',
      companyName: '',
      jobTitle: '',
      source: '',
      stageId: this.defaultStageId(),
    });
    this.closeCreate();
  }

  private async initialize(): Promise<void> {
    await this.store.loadStages();
    this.model.update((value) => ({ ...value, stageId: this.defaultStageId() }));
    await this.store.load();
  }

  private defaultStageId(): string {
    return (
      this.store.stages().find((stage) => stage.category === 'new' && stage.isDefault)?.id ??
      this.editableStages()[0]?.id ??
      ''
    );
  }
}
