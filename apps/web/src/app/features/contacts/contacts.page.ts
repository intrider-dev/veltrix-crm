import { CdkTrapFocus } from '@angular/cdk/a11y';
import type { ElementRef, OnDestroy, OnInit } from '@angular/core';
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
import { FormField, email, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { Router } from '@angular/router';
import { AgGridAngular } from 'ag-grid-angular';
import {
  ClientSideRowModelModule,
  RowSelectionModule,
  themeQuartz,
  type CellKeyDownEvent,
  type ColDef,
  type FullWidthCellKeyDownEvent,
  type GridApi,
  type GridReadyEvent,
  type RowClickedEvent,
  type RowSelectionOptions,
  type SelectionChangedEvent,
} from 'ag-grid-community';

import type {
  Contact,
  ContactImportMapping,
  ContactImportPreview,
  CreateContact,
  SavedView,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { DraftQuotaError, DraftService, type DraftKey } from '../../core/drafts/draft.service';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { PhoneInputComponent } from '../../shared/forms/phone-input.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ContactsStore, type DeletedContact } from './contacts.store';

type ImportField = Exclude<keyof ContactImportMapping, 'customFields'>;

// AG Grid themes allocate generated CSS and internal class-name metadata.
// Reuse one immutable theme across route instances instead of rebuilding it
// whenever the contacts list is reattached.
const CONTACTS_GRID_THEME = themeQuartz.withParams({
  accentColor: '#506fdd',
  backgroundColor: 'var(--surface-raised)',
  borderColor: 'var(--border)',
  foregroundColor: 'var(--text)',
  headerBackgroundColor: 'var(--surface-subtle)',
  oddRowBackgroundColor: 'var(--surface-raised)',
  rowHoverColor: 'var(--surface-selected)',
  fontFamily: 'system-ui, sans-serif',
  fontSize: 13,
  spacing: 6,
  wrapperBorderRadius: 0,
});

@Component({
  selector: 'app-contacts-page',
  imports: [
    AgGridAngular,
    CdkTrapFocus,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    PhoneInputComponent,
  ],
  providers: [ContactsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div class="title-block">
          <div class="title-line">
            <h1>{{ i18n.t('contacts.contacts.title') }}</h1>
            <span>{{ resultCount() }}</span>
          </div>
          <p>{{ i18n.t('contacts.contacts.subtitle') }}</p>
        </div>
        @if (store.mode() === 'contacts') {
          <form class="contact-search" role="search" (submit)="search($event)">
            <app-icon name="search" />
            <label class="visually-hidden" for="contact-quick-search">{{
              i18n.t('contacts.contacts.quickSearch')
            }}</label>
            <input
              id="contact-quick-search"
              type="search"
              [placeholder]="i18n.t('contacts.contacts.search')"
              [value]="store.query()"
              (input)="setQuery($event)"
            />
            <button type="submit" [attr.aria-label]="i18n.t('contacts.contacts.quickSearch')">
              <app-icon name="chevron" />
            </button>
          </form>
        }
        <div class="header-actions">
          @if (permissions.allows('records.create')) {
            <button mat-stroked-button type="button" (click)="openImport()">
              {{ i18n.t('contacts.import.action') }}
            </button>
          }
          @if (permissions.allows('data.export') && store.mode() === 'contacts') {
            <button
              mat-stroked-button
              type="button"
              [disabled]="store.operationPending()"
              (click)="exportContacts()"
            >
              {{ i18n.t('contacts.export.action') }}
            </button>
          }
          @if (permissions.allows('records.create')) {
            <button #addContactButton mat-flat-button type="button" (click)="openCreate()">
              <app-icon name="add" />{{ i18n.t('contacts.contacts.add') }}
            </button>
          }
        </div>
      </header>

      @if (store.mode() === 'contacts') {
        <section class="contact-summary" [attr.aria-label]="i18n.t('contacts.summary.title')">
          <article>
            <span>{{ i18n.t('contacts.summary.loaded') }}</span>
            <strong>{{ loadedCount() }}</strong>
          </article>
          <article>
            <span>{{ i18n.t('common.status.active') }}</span>
            <strong>{{ activeCount() }}</strong>
          </article>
          <article>
            <span>{{ i18n.t('contacts.summary.withCompany') }}</span>
            <strong>{{ withCompanyCount() }}</strong>
          </article>
          <article>
            <span>{{ i18n.t('contacts.summary.withEmail') }}</span>
            <strong>{{ withEmailCount() }}</strong>
          </article>
        </section>
      }

      <div class="contacts-workspace">
        <form
          class="panel filter-toolbar contacts-filter-toolbar"
          (submit)="search($event)"
          role="search"
          [attr.aria-label]="i18n.t('contacts.filter.toolbarLabel')"
        >
          <header>
            <h2>{{ i18n.t('contacts.filter.title') }}</h2>
            <span>{{ i18n.t('contacts.filter.serverBacked') }}</span>
          </header>
          <label class="filter-field filter-search">
            <span class="visually-hidden">{{ i18n.t('contacts.contacts.search') }}</span>
            <span class="filter-search-control">
              <app-icon name="search" />
              <input
                type="search"
                [placeholder]="i18n.t('contacts.contacts.search')"
                [value]="store.query()"
                (input)="setQuery($event)"
              />
            </span>
          </label>
          <label class="filter-field">
            <span>{{ i18n.t('common.field.status') }}</span>
            <mat-select
              panelClass="crm-select-panel"
              [aria-label]="i18n.t('common.field.status')"
              [value]="store.status()"
              (selectionChange)="setStatus($event.value)"
            >
              <mat-option value="all">{{ i18n.t('contacts.filter.allStatuses') }}</mat-option>
              <mat-option value="active">{{ i18n.t('common.status.active') }}</mat-option>
              <mat-option value="inactive">{{ i18n.t('common.status.inactive') }}</mat-option>
            </mat-select>
          </label>
          <label class="filter-field saved-view-picker">
            <span>{{ i18n.t('contacts.savedViews.title') }}</span>
            <mat-select
              panelClass="crm-select-panel"
              [aria-label]="i18n.t('contacts.savedViews.title')"
              [value]="selectedSavedView()?.id ?? ''"
              (selectionChange)="applySavedView($event.value)"
            >
              <mat-option value="">
                {{ i18n.t('contacts.savedViews.current') }}
              </mat-option>
              @for (view of store.savedViews(); track view.id) {
                <mat-option [value]="view.id">
                  {{ view.name }}
                </mat-option>
              }
            </mat-select>
          </label>
          <button mat-stroked-button type="submit">{{ i18n.t('common.action.search') }}</button>
          @if (permissions.allows('records.update')) {
            <button mat-button type="button" (click)="openSavedViewEditor()">
              {{ i18n.t('contacts.savedViews.saveCurrent') }}
            </button>
            @if (selectedSavedView(); as selectedView) {
              <button
                mat-button
                type="button"
                class="danger-action"
                [disabled]="store.operationPending()"
                (click)="deleteSavedView(selectedView)"
              >
                {{ i18n.t('contacts.savedViews.delete') }}
              </button>
            }
          }
        </form>

        <section class="contacts-content">
          <header class="list-toolbar">
            <div
              class="view-switcher segmented-control"
              role="group"
              [attr.aria-label]="i18n.t('contacts.view.label')"
            >
              <button
                type="button"
                [class.active]="store.mode() === 'contacts'"
                [attr.aria-pressed]="store.mode() === 'contacts'"
                (click)="setMode('contacts')"
              >
                {{ i18n.t('contacts.contacts.view.all') }}
              </button>
              <button
                type="button"
                [class.active]="store.mode() === 'trash'"
                [attr.aria-pressed]="store.mode() === 'trash'"
                (click)="setMode('trash')"
              >
                {{ i18n.t('contacts.trash.title') }}
              </button>
            </div>
            <span>{{ i18n.t('contacts.summary.loadedCount', { count: resultCount() }) }}</span>
          </header>

          @if (store.mode() === 'contacts' && savedViewEditorOpen()) {
            <section
              class="saved-views panel"
              [attr.aria-label]="i18n.t('contacts.savedViews.title')"
            >
              <form class="saved-view-form" (submit)="saveCurrentView($event)">
                <label>
                  <span>{{ i18n.t('contacts.savedViews.name') }}</span>
                  <input
                    #savedViewNameInput
                    type="text"
                    maxlength="120"
                    [value]="savedViewName()"
                    (input)="setSavedViewName($event)"
                  />
                </label>
                <button mat-flat-button type="submit" [disabled]="store.operationPending()">
                  {{ i18n.t('contacts.savedViews.save') }}
                </button>
                <button mat-button type="button" (click)="closeSavedViewEditor()">
                  {{ i18n.t('common.action.cancel') }}
                </button>
                @if (savedViewError()) {
                  <span class="inline-error" role="alert">{{ savedViewError() }}</span>
                }
              </form>
            </section>
          }

          @if (store.operationError()) {
            <section class="persistent-message error-message" role="alert">
              <span>{{ operationErrorMessage() }}</span>
              <button mat-button type="button" (click)="store.clearOperationResult()">
                {{ i18n.t('common.action.close') }}
              </button>
            </section>
          }
          @if (store.operationResult(); as result) {
            <section class="persistent-message success-message" role="status">
              <span>{{ i18n.t('contacts.bulk.completed', { count: result.updated }) }}</span>
              <button mat-button type="button" (click)="store.clearOperationResult()">
                {{ i18n.t('common.action.close') }}
              </button>
            </section>
          }

          @if (store.mode() === 'contacts' && selectedRows().length > 0) {
            <section class="bulk-bar panel" [attr.aria-label]="i18n.t('contacts.bulk.title')">
              <strong>{{
                i18n.t('contacts.bulk.selected', { count: selectedRows().length })
              }}</strong>
              @if (permissions.allows('records.update')) {
                @if (store.members().length > 0) {
                  <label class="compact-field">
                    <span>{{ i18n.t('contacts.bulk.assign') }}</span>
                    <mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('contacts.bulk.assign')"
                      [value]="selectedOwnerId()"
                      (selectionChange)="setSelectedOwner($event.value)"
                    >
                      <mat-option value="">{{ i18n.t('contacts.bulk.unassigned') }}</mat-option>
                      @for (member of store.members(); track member.id) {
                        <mat-option [value]="member.userId">{{ member.displayName }}</mat-option>
                      }
                    </mat-select>
                  </label>
                  <button
                    mat-button
                    type="button"
                    [disabled]="store.operationPending()"
                    (click)="bulkAssign()"
                  >
                    {{ i18n.t('contacts.bulk.applyOwner') }}
                  </button>
                }
                @if (store.tags().length > 0) {
                  <label class="compact-field">
                    <span>{{ i18n.t('contacts.bulk.tag') }}</span>
                    <mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('contacts.bulk.tag')"
                      [value]="selectedTagId()"
                      (selectionChange)="setSelectedTag($event.value)"
                    >
                      <mat-option value="">{{ i18n.t('contacts.bulk.chooseTag') }}</mat-option>
                      @for (tag of store.tags(); track tag.id) {
                        <mat-option [value]="tag.id">{{ tag.name }}</mat-option>
                      }
                    </mat-select>
                  </label>
                  <label class="compact-field">
                    <span>{{ i18n.t('contacts.bulk.tagMode') }}</span>
                    <mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('contacts.bulk.tagMode')"
                      [value]="tagMode()"
                      (selectionChange)="setTagMode($event.value)"
                    >
                      <mat-option value="add">{{ i18n.t('contacts.bulk.tagAdd') }}</mat-option>
                      <mat-option value="remove">{{
                        i18n.t('contacts.bulk.tagRemove')
                      }}</mat-option>
                      <mat-option value="replace">{{
                        i18n.t('contacts.bulk.tagReplace')
                      }}</mat-option>
                    </mat-select>
                  </label>
                  <button
                    mat-button
                    type="button"
                    [disabled]="!selectedTagId() || store.operationPending()"
                    (click)="bulkTag()"
                  >
                    {{ i18n.t('contacts.bulk.applyTag') }}
                  </button>
                }
              }
              @if (permissions.allows('records.delete')) {
                @if (bulkDeleteConfirm()) {
                  <span class="delete-confirm" role="alert">
                    {{ i18n.t('contacts.bulk.deleteConfirm', { count: selectedRows().length }) }}
                  </span>
                  <button mat-button type="button" (click)="bulkDeleteConfirm.set(false)">
                    {{ i18n.t('common.action.cancel') }}
                  </button>
                  <button
                    #bulkDeleteConfirmButton
                    mat-flat-button
                    type="button"
                    class="danger-button"
                    [disabled]="store.operationPending()"
                    (click)="bulkDelete()"
                  >
                    {{ i18n.t('contacts.bulk.confirmDelete') }}
                  </button>
                } @else {
                  <button
                    mat-button
                    type="button"
                    class="danger-action"
                    (click)="confirmBulkDelete()"
                  >
                    {{ i18n.t('contacts.bulk.delete') }}
                  </button>
                }
              }
            </section>
          }

          @if (store.error()) {
            <app-error-panel [error]="store.error()" (retry)="store.load()" />
          }

          @if (store.mode() === 'contacts') {
            <section class="grid-panel panel" [attr.aria-busy]="store.loading()">
              @if (store.loading() && store.contacts().length === 0) {
                <div class="grid-skeleton">
                  <div class="skeleton"></div>
                  <div class="skeleton"></div>
                  <div class="skeleton"></div>
                  <div class="skeleton"></div>
                </div>
              } @else if (store.contacts().length === 0) {
                <div class="empty-state">{{ i18n.t('contacts.contacts.empty') }}</div>
              } @else {
                <ag-grid-angular
                  [theme]="gridTheme"
                  [modules]="gridModules"
                  [rowData]="gridRows()"
                  [columnDefs]="columns"
                  [defaultColDef]="defaultColumn"
                  [getRowId]="getRowId"
                  [rowSelection]="rowSelection()"
                  [animateRows]="false"
                  [rowHeight]="44"
                  [headerHeight]="42"
                  [ariaLabel]="
                    i18n.t(
                      permissions.allows('records.update') || permissions.allows('records.delete')
                        ? 'contacts.grid.label'
                        : 'contacts.grid.readonlyLabel'
                    )
                  "
                  (gridReady)="onGridReady($event)"
                  (selectionChanged)="selectionChanged($event)"
                  (rowClicked)="openRow($event)"
                  (cellKeyDown)="openRowFromKeyboard($event)"
                />
              }
            </section>
            @if (store.nextCursor()) {
              <button
                mat-stroked-button
                type="button"
                class="load-more"
                [disabled]="store.loading()"
                (click)="store.load(false)"
              >
                {{ i18n.t('web.contact.loadMore') }}
              </button>
            }
          } @else {
            <section class="trash-panel panel" [attr.aria-busy]="store.loading()">
              <header>
                <div>
                  <h2>{{ i18n.t('contacts.trash.title') }}</h2>
                  <p>{{ i18n.t('contacts.trash.description') }}</p>
                </div>
              </header>
              @if (store.loading() && store.trash().length === 0) {
                <div class="skeleton trash-skeleton"></div>
              } @else if (store.trash().length === 0) {
                <div class="empty-state">{{ i18n.t('contacts.trash.empty') }}</div>
              } @else {
                <div class="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th scope="col">{{ i18n.t('common.field.name') }}</th>
                        <th scope="col">{{ i18n.t('common.field.email') }}</th>
                        <th scope="col">{{ i18n.t('contacts.trash.deletedAt') }}</th>
                        <th scope="col">
                          <span class="visually-hidden">{{
                            i18n.t('contacts.trash.actions')
                          }}</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      @for (record of store.trash(); track record.id) {
                        <tr>
                          <td>{{ record.displayName }}</td>
                          <td>{{ record.email ?? '—' }}</td>
                          <td>
                            {{
                              i18n.date(record.deletedAt, {
                                dateStyle: 'medium',
                                timeStyle: 'short',
                              })
                            }}
                          </td>
                          <td>
                            @if (permissions.allows('records.delete')) {
                              <button
                                mat-button
                                type="button"
                                [disabled]="store.operationPending()"
                                (click)="restoreContact(record)"
                              >
                                {{ i18n.t('contacts.trash.restore') }}
                              </button>
                            }
                          </td>
                        </tr>
                      }
                    </tbody>
                  </table>
                </div>
              }
            </section>
            @if (store.trashNextCursor()) {
              <button
                mat-stroked-button
                type="button"
                class="load-more"
                [disabled]="store.loading()"
                (click)="store.load(false)"
              >
                {{ i18n.t('contacts.trash.loadMore') }}
              </button>
            }
          }
        </section>
      </div>
    </div>

    @if (importOpen()) {
      <button
        class="drawer-scrim"
        type="button"
        (click)="closeImport()"
        [attr.aria-label]="i18n.t('common.action.close')"
      ></button>
      <aside
        class="create-drawer import-drawer"
        role="dialog"
        aria-modal="true"
        cdkTrapFocus
        [cdkTrapFocusAutoCapture]="true"
        aria-labelledby="import-contact-title"
      >
        <header>
          <div>
            <h2 id="import-contact-title">{{ i18n.t('contacts.import.title') }}</h2>
            <p>{{ i18n.t('contacts.import.description') }}</p>
          </div>
          <button
            mat-icon-button
            type="button"
            (click)="closeImport()"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>

        @if (!store.importPreview()) {
          <label class="file-picker">
            <span>{{ i18n.t('contacts.import.chooseFile') }}</span>
            <input
              #importFileInput
              type="file"
              accept=".csv,text/csv"
              [disabled]="store.importBusy()"
              (change)="previewImport($event)"
            />
          </label>
          <small>{{ i18n.t('contacts.import.fileHint') }}</small>
        } @else if (store.importPreview(); as preview) {
          <section class="import-summary" aria-live="polite">
            <strong>{{ i18n.t('contacts.import.rowsFound', { count: preview.totalRows }) }}</strong>
            <span>{{
              i18n.t('contacts.import.columnsFound', { count: preview.headers.length })
            }}</span>
          </section>

          <div class="preview-table table-scroll">
            <table>
              <thead>
                <tr>
                  @for (header of preview.headers; track header) {
                    <th scope="col">{{ header }}</th>
                  }
                </tr>
              </thead>
              <tbody>
                @for (row of preview.sampleRows; track $index) {
                  <tr>
                    @for (header of preview.headers; track header) {
                      <td>{{ row[header] }}</td>
                    }
                  </tr>
                }
              </tbody>
            </table>
          </div>

          @if (!store.importStatus()) {
            <form class="mapping-form" (submit)="queueImport($event)">
              <h3>{{ i18n.t('contacts.import.mappingTitle') }}</h3>
              <p>{{ i18n.t('contacts.import.mappingDescription') }}</p>
              <div class="mapping-grid">
                @for (field of importFields; track field.key) {
                  <label>
                    <span>
                      {{ i18n.t(field.label) }}
                      @if (field.required) {
                        <span aria-hidden="true">*</span>
                      }
                    </span>
                    <mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t(field.label)"
                      [value]="mappingValue(field.key)"
                      (selectionChange)="setMapping(field.key, $event.value)"
                    >
                      <mat-option value="">{{ i18n.t('contacts.import.notMapped') }}</mat-option>
                      @for (header of preview.headers; track header) {
                        <mat-option [value]="header">{{ header }}</mat-option>
                      }
                    </mat-select>
                  </label>
                }
                @for (field of store.customFields(); track field.id) {
                  <label>
                    <span>{{ i18n.t('contacts.import.customField', { name: field.label }) }}</span>
                    <mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('contacts.import.customField', { name: field.label })"
                      [value]="customMappingValue(field.fieldKey)"
                      (selectionChange)="setCustomMapping(field.fieldKey, $event.value)"
                    >
                      <mat-option value="">{{ i18n.t('contacts.import.notMapped') }}</mat-option>
                      @for (header of preview.headers; track header) {
                        <mat-option [value]="header">{{ header }}</mat-option>
                      }
                    </mat-select>
                  </label>
                }
              </div>
              @if (importMappingError()) {
                <div class="form-error" role="alert">{{ i18n.t(importMappingError()!) }}</div>
              }
              <div class="drawer-actions">
                <button mat-button type="button" (click)="store.resetImport()">
                  {{ i18n.t('contacts.import.chooseAnother') }}
                </button>
                <button mat-flat-button type="submit" [disabled]="store.importBusy()">
                  {{ i18n.t('contacts.import.start') }}
                </button>
              </div>
            </form>
          }
        }

        @if (store.importStatus(); as status) {
          <section class="import-progress" aria-live="polite">
            <h3>{{ i18n.t(importStatusKey(status.status)) }}</h3>
            <progress [max]="status.totalRows || 1" [value]="status.processedRows"></progress>
            <p>
              {{
                i18n.t('contacts.import.progress', {
                  processed: status.processedRows,
                  total: status.totalRows,
                })
              }}
            </p>
            <dl>
              <div>
                <dt>{{ i18n.t('contacts.import.created') }}</dt>
                <dd>{{ status.createdRows }}</dd>
              </div>
              <div>
                <dt>{{ i18n.t('contacts.import.errors') }}</dt>
                <dd>{{ status.errorRows }}</dd>
              </div>
            </dl>
            @if (status.errorRows > 0 && store.importErrorsUrl()) {
              <a
                mat-stroked-button
                [href]="store.importErrorsUrl()"
                download="contact-import-errors.csv"
                >{{ i18n.t('contacts.import.downloadErrors') }}</a
              >
            }
            @if (status.status === 'completed' || status.status === 'failed') {
              <button mat-button type="button" (click)="startAnotherImport()">
                {{ i18n.t('contacts.import.another') }}
              </button>
            }
          </section>
        }
        @if (store.importError()) {
          <div class="form-error import-retry" role="alert">
            <span>{{ importErrorMessage() }}</span>
            @if (store.importPreview() && store.importStatus()?.status !== 'failed') {
              <button mat-button type="button" (click)="store.retryImport()">
                {{ i18n.t('common.action.retry') }}
              </button>
            }
          </div>
        }
      </aside>
    }

    @if (createOpen()) {
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
        aria-labelledby="create-contact-title"
      >
        <header>
          <h2 id="create-contact-title">{{ i18n.t('contacts.contacts.createTitle') }}</h2>
          <button
            mat-icon-button
            type="button"
            (click)="closeCreate()"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>
        <form (submit)="create($event)" (input)="scheduleDraft()" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('contacts.contacts.firstName') }}</mat-label>
            <input matInput #firstName [formField]="contactForm.firstName" />
            @if (contactForm.firstName().touched() && contactForm.firstName().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('contacts.contacts.lastName') }}</mat-label>
            <input matInput [formField]="contactForm.lastName" />
            @if (contactForm.lastName().touched() && contactForm.lastName().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.required') }}</mat-error>
            }
          </mat-form-field>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('common.field.email') }}</mat-label>
            <input matInput type="email" [formField]="contactForm.email" />
            @if (contactForm.email().touched() && contactForm.email().invalid()) {
              <mat-error>{{ i18n.t('auth.validation.email') }}</mat-error>
            }
          </mat-form-field>
          <app-phone-input
            [formField]="contactForm.phone"
            [label]="i18n.t('common.field.phone')"
            (validityChange)="phoneValid.set($event)"
          />
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('contacts.contacts.jobTitle') }}</mat-label>
            <input matInput [formField]="contactForm.jobTitle" />
          </mat-form-field>
          @if (createError()) {
            <div class="form-error" role="alert">{{ createError() }}</div>
          }
          @if (draftStatus()) {
            <small class="draft-status" role="status">{{ i18n.t(draftStatus()!) }}</small>
          }
          <div class="drawer-actions">
            <button mat-button type="button" (click)="closeCreate()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="submit" [disabled]="store.creating()">
              {{ i18n.t(store.creating() ? 'web.form.saving' : 'common.action.create') }}
            </button>
          </div>
        </form>
      </aside>
    }
  `,
  styleUrl: './contacts.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ContactsPage implements OnInit, OnDestroy {
  readonly store = inject(ContactsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly importOpen = signal(false);
  readonly createError = signal<string | null>(null);
  readonly draftStatus = signal<AppMessageKey | null>(null);
  readonly selectedRows = signal<readonly Contact[]>([]);
  readonly selectedOwnerId = signal('');
  readonly selectedTagId = signal('');
  readonly tagMode = signal<'add' | 'remove' | 'replace'>('add');
  readonly bulkDeleteConfirm = signal(false);
  readonly savedViewEditorOpen = signal(false);
  readonly savedViewName = signal('');
  readonly savedViewError = signal<string | null>(null);
  readonly selectedSavedView = signal<SavedView | null>(null);
  readonly importMapping = signal<ContactImportMapping>({
    firstName: '',
    lastName: '',
    customFields: {},
  });
  readonly importMappingError = signal<AppMessageKey | null>(null);
  readonly firstNameInput = viewChild<ElementRef<HTMLInputElement>>('firstName');
  readonly addContactButton = viewChild<ElementRef<HTMLButtonElement>>('addContactButton');
  readonly savedViewNameInput = viewChild<ElementRef<HTMLInputElement>>('savedViewNameInput');
  readonly importFileInput = viewChild<ElementRef<HTMLInputElement>>('importFileInput');
  readonly bulkDeleteConfirmButton =
    viewChild<ElementRef<HTMLButtonElement>>('bulkDeleteConfirmButton');
  readonly phoneValid = signal(true);
  readonly model = signal({ firstName: '', lastName: '', email: '', phone: '', jobTitle: '' });
  readonly contactForm = form(this.model, (schema) => {
    required(schema.firstName);
    required(schema.lastName);
    email(schema.email);
    validate(schema.phone, ({ value }) =>
      !value() || this.phoneValid()
        ? undefined
        : { kind: 'phone', message: this.i18n.t('common.validation.phone') },
    );
  });
  readonly gridModules = [ClientSideRowModelModule, RowSelectionModule];
  readonly gridTheme = CONTACTS_GRID_THEME;
  readonly defaultColumn: ColDef<Contact> = {
    flex: 1,
    minWidth: 130,
    resizable: true,
    sortable: true,
  };
  readonly columns: ColDef<Contact>[] = [
    {
      field: 'displayName',
      minWidth: 190,
      headerValueGetter: () => this.i18n.t('common.field.name'),
    },
    {
      field: 'companyName',
      minWidth: 160,
      headerValueGetter: () => this.i18n.t('contacts.contacts.company'),
    },
    {
      field: 'phone',
      minWidth: 150,
      headerValueGetter: () => this.i18n.t('common.field.phone'),
    },
    {
      field: 'email',
      minWidth: 190,
      headerValueGetter: () => this.i18n.t('common.field.email'),
    },
    {
      colId: 'owner',
      minWidth: 150,
      headerValueGetter: () => this.i18n.t('common.field.owner'),
      valueGetter: ({ data }) => this.ownerName(data?.ownerId),
    },
    {
      field: 'status',
      minWidth: 110,
      maxWidth: 130,
      headerValueGetter: () => this.i18n.t('common.field.status'),
      valueFormatter: ({ value }) =>
        value === 'active'
          ? this.i18n.t('common.status.active')
          : this.i18n.t('common.status.inactive'),
    },
    {
      field: 'createdAt',
      minWidth: 140,
      headerValueGetter: () => this.i18n.t('contacts.contacts.created'),
      valueFormatter: ({ value }) =>
        typeof value === 'string' ? this.i18n.date(value, { dateStyle: 'medium' }) : '',
    },
  ];
  readonly rowSelection = computed<RowSelectionOptions<Contact>>(() => ({
    mode: 'multiRow',
    checkboxes:
      this.permissions.allows('records.update') || this.permissions.allows('records.delete'),
    headerCheckbox:
      this.permissions.allows('records.update') || this.permissions.allows('records.delete'),
    enableClickSelection: false,
    selectAll: 'all',
    isRowSelectable: () =>
      this.permissions.allows('records.update') || this.permissions.allows('records.delete'),
  }));
  readonly getRowId = ({ data }: { data: Contact }) => data.id;
  readonly gridRows = computed(() => [...this.store.contacts()]);
  readonly loadedCount = computed(() => this.store.contacts().length);
  readonly activeCount = computed(
    () => this.store.contacts().filter((contact) => contact.status === 'active').length,
  );
  readonly withCompanyCount = computed(
    () => this.store.contacts().filter((contact) => Boolean(contact.companyId)).length,
  );
  readonly withEmailCount = computed(
    () => this.store.contacts().filter((contact) => Boolean(contact.email)).length,
  );
  readonly resultCount = computed(() =>
    this.store.mode() === 'contacts' ? this.store.contacts().length : this.store.trash().length,
  );
  readonly importFields: ReadonlyArray<{
    key: ImportField;
    label: AppMessageKey;
    required: boolean;
  }> = [
    { key: 'firstName', label: 'contacts.contacts.firstName', required: true },
    { key: 'lastName', label: 'contacts.contacts.lastName', required: true },
    { key: 'email', label: 'common.field.email', required: false },
    { key: 'phone', label: 'common.field.phone', required: false },
    { key: 'jobTitle', label: 'contacts.contacts.jobTitle', required: false },
    { key: 'companyName', label: 'contacts.contacts.company', required: false },
    { key: 'ownerEmail', label: 'contacts.import.ownerEmail', required: false },
    { key: 'status', label: 'common.field.status', required: false },
    { key: 'source', label: 'contacts.contacts.source', required: false },
  ];

  private readonly router = inject(Router);
  private readonly drafts = inject(DraftService);
  private readonly workspace = inject(WorkspaceStore);
  private readonly injector = inject(Injector);
  private draftTimer: ReturnType<typeof setTimeout> | null = null;
  private gridApi: GridApi<Contact> | null = null;

  ownerName(ownerId: string | null | undefined): string {
    if (!ownerId) return '—';
    return this.store.members().find((member) => member.userId === ownerId)?.displayName ?? '—';
  }

  ngOnInit(): void {
    void Promise.all([
      this.store.load(),
      this.store.loadReferences(this.permissions.allows('members.read')),
    ]);
  }

  ngOnDestroy(): void {
    this.cancelScheduledDraft();
    if (this.gridApi && !this.gridApi.isDestroyed()) this.gridApi.destroy();
    this.gridApi = null;
  }

  setQuery(event: Event): void {
    this.store.query.set((event.target as HTMLInputElement).value);
  }

  setStatus(status: string): void {
    if (status === 'all' || status === 'active' || status === 'inactive')
      this.store.status.set(status);
  }

  search(event: SubmitEvent): void {
    event.preventDefault();
    this.clearSelection();
    void this.store.load();
  }

  setMode(mode: 'contacts' | 'trash'): void {
    this.clearSelection();
    void this.store.setMode(mode);
  }

  applySavedView(id: string): void {
    const view = this.store.savedViews().find((item) => item.id === id) ?? null;
    this.selectedSavedView.set(view);
    this.clearSelection();
    void this.store.applySavedView(view);
  }

  openSavedViewEditor(): void {
    this.savedViewError.set(null);
    this.savedViewEditorOpen.set(true);
    focusAfterNextRender(this.injector, () => this.savedViewNameInput()?.nativeElement);
  }

  closeSavedViewEditor(): void {
    this.savedViewEditorOpen.set(false);
    this.savedViewName.set('');
    this.savedViewError.set(null);
  }

  setSavedViewName(event: Event): void {
    this.savedViewName.set((event.target as HTMLInputElement).value);
  }

  async saveCurrentView(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    const name = this.savedViewName().trim();
    if (!name) {
      this.savedViewError.set(this.i18n.t('contacts.savedViews.nameRequired'));
      return;
    }
    try {
      const view = await this.store.createSavedView(name);
      this.selectedSavedView.set(view);
      this.closeSavedViewEditor();
    } catch {
      this.savedViewError.set(this.operationErrorMessage());
    }
  }

  async deleteSavedView(view: SavedView): Promise<void> {
    try {
      await this.store.deleteSavedView(view);
      this.selectedSavedView.set(null);
    } catch {
      // The persistent operation panel owns the localized error.
    }
  }

  onGridReady(event: GridReadyEvent<Contact>): void {
    this.gridApi = event.api;
  }

  selectionChanged(event: SelectionChangedEvent<Contact>): void {
    this.selectedRows.set(
      (event.selectedNodes ?? [])
        .map((node) => node.data)
        .filter((contact): contact is Contact => contact !== undefined),
    );
    this.bulkDeleteConfirm.set(false);
  }

  setSelectedOwner(ownerId: string): void {
    this.selectedOwnerId.set(ownerId);
  }

  setSelectedTag(tagId: string): void {
    this.selectedTagId.set(tagId);
  }

  setTagMode(mode: string): void {
    if (mode === 'add' || mode === 'remove' || mode === 'replace') this.tagMode.set(mode);
  }

  async bulkAssign(): Promise<void> {
    try {
      await this.store.bulkAssign(this.selectedRows(), this.selectedOwnerId() || null);
      this.clearSelection();
    } catch {
      // The persistent operation panel owns the localized error.
    }
  }

  async bulkTag(): Promise<void> {
    if (!this.selectedTagId()) return;
    try {
      await this.store.bulkTag(this.selectedRows(), [this.selectedTagId()], this.tagMode());
      this.clearSelection();
    } catch {
      // The persistent operation panel owns the localized error.
    }
  }

  confirmBulkDelete(): void {
    this.bulkDeleteConfirm.set(true);
    focusAfterNextRender(this.injector, () => this.bulkDeleteConfirmButton()?.nativeElement);
  }

  async bulkDelete(): Promise<void> {
    try {
      await this.store.bulkDelete(this.selectedRows());
      this.clearSelection();
    } catch {
      // The persistent operation panel owns the localized error.
    } finally {
      this.bulkDeleteConfirm.set(false);
    }
  }

  async restoreContact(record: DeletedContact): Promise<void> {
    try {
      await this.store.restore(record);
    } catch {
      // The persistent operation panel owns the localized error.
    }
  }

  async exportContacts(): Promise<void> {
    try {
      const blob = await this.store.exportCurrentView();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `contacts-${new Date().toISOString().slice(0, 10)}.csv`;
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 0);
    } catch {
      // The persistent operation panel owns the localized error.
    }
  }

  openImport(): void {
    const status = this.store.importStatus();
    if (status?.status === 'completed' || status?.status === 'failed') this.store.resetImport();
    this.importOpen.set(true);
    focusAfterNextRender(this.injector, () => this.importFileInput()?.nativeElement);
  }

  closeImport(): void {
    this.importOpen.set(false);
    focusAfterNextRender(this.injector, () => this.addContactButton()?.nativeElement);
  }

  async previewImport(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.item(0);
    if (!file) return;
    try {
      const preview = await this.store.previewImport(file);
      this.applySuggestedMapping(preview);
    } catch {
      // The import panel owns the localized error.
    }
  }

  mappingValue(field: ImportField): string {
    return this.importMapping()[field] ?? '';
  }

  setMapping(field: ImportField, value: string): void {
    this.importMapping.update((current) => ({ ...current, [field]: value }));
    this.importMappingError.set(null);
  }

  customMappingValue(fieldKey: string): string {
    return this.importMapping().customFields?.[fieldKey] ?? '';
  }

  setCustomMapping(fieldKey: string, value: string): void {
    this.importMapping.update((current) => {
      const customFields = { ...current.customFields };
      if (value) customFields[fieldKey] = value;
      else delete customFields[fieldKey];
      return { ...current, customFields };
    });
    this.importMappingError.set(null);
  }

  async queueImport(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    const mapping = this.importMapping();
    if (!mapping.firstName || !mapping.lastName) {
      this.importMappingError.set('contacts.import.namesRequired');
      return;
    }
    const mappedHeaders = Object.values(mapping).filter(
      (value): value is string => typeof value === 'string' && value.length > 0,
    );
    mappedHeaders.push(...Object.values(mapping.customFields ?? {}).filter(Boolean));
    if (new Set(mappedHeaders).size !== mappedHeaders.length) {
      this.importMappingError.set('contacts.import.uniqueHeaders');
      return;
    }
    try {
      await this.store.queueImport(mapping);
    } catch {
      // The import panel owns the localized error.
    }
  }

  startAnotherImport(): void {
    this.store.resetImport();
    this.importMapping.set({ firstName: '', lastName: '', customFields: {} });
    focusAfterNextRender(this.injector, () => this.importFileInput()?.nativeElement);
  }

  importStatusKey(status: string): AppMessageKey {
    switch (status) {
      case 'queued':
        return 'contacts.import.status.queued';
      case 'processing':
        return 'contacts.import.status.processing';
      case 'completed':
        return 'contacts.import.status.completed';
      case 'failed':
        return 'contacts.import.status.failed';
      default:
        return 'contacts.import.status.preview';
    }
  }

  operationErrorMessage(): string {
    const error = this.store.operationError();
    return this.i18n.problem(error instanceof Error ? error.message : 'generic');
  }

  importErrorMessage(): string {
    const error = this.store.importError();
    return this.i18n.problem(error instanceof Error ? error.message : 'generic');
  }

  async openCreate(): Promise<void> {
    this.createError.set(null);
    this.phoneValid.set(true);
    this.createOpen.set(true);
    const draft = await this.drafts.load<ContactDraftModel>(this.draftKey());
    if (draft) {
      this.model.set(draft);
      this.draftStatus.set('pwa.draftRestored');
    }
    focusAfterNextRender(this.injector, () => this.firstNameInput()?.nativeElement);
  }

  closeCreate(): void {
    if (!this.store.creating()) {
      this.createOpen.set(false);
      focusAfterNextRender(this.injector, () => this.addContactButton()?.nativeElement);
    }
  }

  @HostListener('document:keydown.escape')
  closeOnEscape(): void {
    if (this.createOpen()) this.closeCreate();
    else if (this.importOpen()) this.closeImport();
  }

  scheduleDraft(): void {
    if (this.draftTimer !== null) clearTimeout(this.draftTimer);
    this.draftTimer = setTimeout(() => {
      this.draftTimer = null;
      void this.saveDraft();
    }, 250);
  }

  openRow(event: RowClickedEvent<Contact>): void {
    const target = event.event?.target;
    if (target instanceof Element && target.closest('.ag-selection-checkbox')) return;
    if (event.data) void this.router.navigate(['/contacts', event.data.id]);
  }

  openRowFromKeyboard(event: CellKeyDownEvent<Contact> | FullWidthCellKeyDownEvent<Contact>): void {
    if (event.event instanceof KeyboardEvent && event.event.key === 'Enter' && event.data) {
      void this.router.navigate(['/contacts', event.data.id]);
    }
  }

  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.createError.set(null);
    if (this.contactForm().invalid() || (!!this.model().phone && !this.phoneValid())) {
      this.contactForm().markAsTouched();
      return;
    }
    const value = this.model();
    const body: CreateContact = {
      firstName: value.firstName.trim(),
      lastName: value.lastName.trim(),
      email: value.email.trim() || null,
      phone: value.phone.trim() || null,
      jobTitle: value.jobTitle.trim() || null,
      status: 'active',
    };
    this.cancelScheduledDraft();
    await this.saveDraft();
    try {
      const contact = await this.drafts.submitWithDraft(this.draftKey(), () =>
        this.store.create(body),
      );
      this.createOpen.set(false);
      this.draftStatus.set(null);
      this.model.set({ firstName: '', lastName: '', email: '', phone: '', jobTitle: '' });
      await this.router.navigate(['/contacts', contact.id]);
    } catch (error) {
      this.createError.set(
        this.i18n.problem(error instanceof Error ? error.message : 'contact.create'),
      );
    }
  }

  private clearSelection(): void {
    this.gridApi?.deselectAll();
    this.selectedRows.set([]);
    this.bulkDeleteConfirm.set(false);
  }

  private applySuggestedMapping(preview: ContactImportPreview): void {
    const suggested = preview.suggestedMapping;
    this.importMapping.set({
      firstName: suggested['firstName'] ?? '',
      lastName: suggested['lastName'] ?? '',
      email: suggested['email'] ?? '',
      phone: suggested['phone'] ?? '',
      jobTitle: suggested['jobTitle'] ?? '',
      companyName: suggested['companyName'] ?? '',
      ownerEmail: suggested['ownerEmail'] ?? '',
      status: suggested['status'] ?? '',
      source: suggested['source'] ?? '',
      customFields: {},
    });
    this.importMappingError.set(null);
  }

  private draftKey(): DraftKey {
    return {
      workspaceId: this.workspace.id() ?? 'unavailable',
      feature: 'contact-create',
      recordId: 'new',
    };
  }

  private async saveDraft(): Promise<void> {
    if (!this.workspace.id()) return;
    try {
      await this.drafts.save(this.draftKey(), this.model());
      this.draftStatus.set('pwa.draftSaved');
    } catch (error) {
      if (error instanceof DraftQuotaError) {
        this.draftStatus.set(
          error.reason === 'entry-too-large' ? 'pwa.draftTooLarge' : 'pwa.storageFull',
        );
      }
    }
  }

  private cancelScheduledDraft(): void {
    if (this.draftTimer === null) return;
    clearTimeout(this.draftTimer);
    this.draftTimer = null;
  }
}

type ContactDraftModel = {
  firstName: string;
  lastName: string;
  email: string;
  phone: string;
  jobTitle: string;
};
