import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { FormField, email, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type { Lead, LeadStage, LeadStatus } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { trimmedOrNull } from '../../shared/forms/feature-validation';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { LeadsStore } from './leads.store';

@Component({
  selector: 'app-leads-page',
  imports: [
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RecordAssignmentsComponent,
  ],
  providers: [LeadsStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.leads') }}</h1>
          <p>{{ i18n.t('leads.subtitle') }}</p>
        </div>
        @if (permissions.allows('leads.create')) {
          <button mat-flat-button type="button" (click)="openCreate()">
            <app-icon name="add" />{{ i18n.t('leads.add') }}
          </button>
        }
      </header>

      <form class="panel feature-toolbar" (submit)="filter($event)" role="search">
        <label class="native-field grow">
          <span>{{ i18n.t('leads.search') }}</span>
          <input
            type="search"
            [value]="store.query()"
            (input)="store.query.set(inputValue($event))"
          />
        </label>
        <label class="native-field">
          <span>{{ i18n.t('common.field.status') }}</span>
          <select [value]="store.status()" (change)="setFilterStatus($event)">
            <option value="">{{ i18n.t('leads.status.all') }}</option>
            @for (status of statuses; track status) {
              <option [value]="status">{{ i18n.t(statusKey(status)) }}</option>
            }
            <option value="converted">{{ i18n.t('leads.status.converted') }}</option>
          </select>
        </label>
        <button mat-stroked-button type="submit">{{ i18n.t('common.action.search') }}</button>
      </form>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      @if (createOpen()) {
        <section class="panel editor" aria-labelledby="new-lead-title">
          <header>
            <h2 id="new-lead-title">{{ i18n.t('leads.createTitle') }}</h2>
            <button mat-button type="button" (click)="createOpen.set(false)">
              {{ i18n.t('common.action.close') }}
            </button>
          </header>
          <form class="feature-form" (submit)="create($event)" novalidate>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
              <input #nameInput matInput [formField]="leadForm.name" autocomplete="off" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.email') }}</mat-label>
              <input matInput [formField]="leadForm.email" inputmode="email" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.phone') }}</mat-label>
              <input matInput [formField]="leadForm.phone" inputmode="tel" />
            </mat-form-field>
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
              <select [formField]="leadForm.stageId">
                @for (stage of editableStages(); track stage.id) {
                  <option [value]="stage.id">{{ stageLabel(stage) }}</option>
                }
              </select>
            </label>
            <div class="form-actions">
              <button mat-button type="button" (click)="createOpen.set(false)">
                {{ i18n.t('common.action.cancel') }}
              </button>
              <button mat-flat-button type="submit" [disabled]="store.saving()">
                {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.create') }}
              </button>
            </div>
          </form>
        </section>
      }

      <section class="panel record-list" [attr.aria-busy]="store.loading()">
        @if (store.loading() && store.leads().length === 0) {
          <div class="list-skeleton" aria-label="Loading">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else if (store.leads().length === 0) {
          <div class="empty-state">{{ i18n.t('leads.empty') }}</div>
        } @else {
          @for (lead of store.leads(); track lead.id) {
            <article>
              <div class="record-main">
                <div class="record-avatar" aria-hidden="true">{{ lead.name.charAt(0) }}</div>
                <div>
                  <h2>{{ lead.name }}</h2>
                  <p>{{ lead.companyName || lead.email || i18n.t('leads.noDetails') }}</p>
                </div>
              </div>
              <div class="record-meta">
                @if (permissions.allows('leads.update') && lead.status !== 'converted') {
                  <label class="visually-hidden" [for]="'lead-status-' + lead.id">{{
                    i18n.t('leads.stage')
                  }}</label>
                  <select
                    [id]="'lead-status-' + lead.id"
                    [value]="lead.stageId"
                    [disabled]="store.isMoving(lead.id)"
                    (change)="changeStage(lead, $event)"
                  >
                    @for (stage of editableStages(); track stage.id) {
                      <option [value]="stage.id">{{ stageLabel(stage) }}</option>
                    }
                  </select>
                  <button
                    mat-stroked-button
                    type="button"
                    [disabled]="store.saving()"
                    (click)="store.convert(lead)"
                  >
                    {{ i18n.t('leads.convert') }}
                  </button>
                } @else {
                  <span class="status-pill">{{ leadStageLabel(lead) }}</span>
                }
                <button mat-button type="button" (click)="selectedLead.set(lead)">
                  {{ i18n.t('assignments.manage') }}
                </button>
              </div>
            </article>
          }
        }
      </section>
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
  styles: `
    .editor > header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0.75rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .editor h2 {
      margin: 0;
      font-size: 1rem;
    }
    .feature-form {
      grid-template-columns: repeat(3, minmax(11rem, 1fr));
      min-width: 0;
      padding: 1rem;
    }
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
    article select {
      min-height: 2.25rem;
      appearance: none;
      padding-inline: 0.7rem 2.1rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      color: var(--text);
      background: var(--surface-raised);
      background-image:
        linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
        linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
      background-position:
        calc(100% - 0.9rem) 50%,
        calc(100% - 0.65rem) 50%;
      background-size:
        0.28rem 0.28rem,
        0.28rem 0.28rem;
      background-repeat: no-repeat;
    }
    @media (max-width: 760px) {
      .feature-form {
        grid-template-columns: 1fr;
      }
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
  });
  readonly nameInput = viewChild<ElementRef<HTMLInputElement>>('nameInput');
  private readonly injector = inject(Injector);

  ngOnInit(): void {
    void this.initialize();
  }

  openCreate(): void {
    this.createOpen.set(true);
    focusAfterNextRender(this.injector, () => this.nameInput()?.nativeElement);
  }

  filter(event: Event): void {
    event.preventDefault();
    void this.store.load();
  }

  inputValue(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }

  setFilterStatus(event: Event): void {
    this.store.status.set((event.target as HTMLSelectElement).value as LeadStatus | '');
  }

  changeStage(lead: Lead, event: Event): void {
    const stage = this.store
      .stages()
      .find((candidate) => candidate.id === (event.target as HTMLSelectElement).value);
    if (stage) void this.store.changeStage(lead, stage);
  }

  editableStages(): readonly LeadStage[] {
    return this.store.stages().filter((stage) => stage.category !== 'converted');
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
    if (this.leadForm().invalid()) return;
    const value = this.model();
    const stage = this.store.stages().find((candidate) => candidate.id === value.stageId);
    if (!stage || stage.category === 'converted') return;
    await this.store.create({
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
    this.model.set({
      name: '',
      email: '',
      phone: '',
      companyName: '',
      jobTitle: '',
      source: '',
      stageId: this.defaultStageId(),
    });
    this.createOpen.set(false);
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
