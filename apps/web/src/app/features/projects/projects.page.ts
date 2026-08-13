import { CdkTrapFocus } from '@angular/cdk/a11y';
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
import { FormField, form, maxLength, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';

import { apiErrorMessage } from '../../core/api/api-error-message';
import type { ProjectInput } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ProjectsStore } from './projects.store';

interface ProjectCreateModel {
  readonly name: string;
  readonly description: string;
  readonly plannedStartDate: string;
  readonly targetEndDate: string;
  readonly visibility: ProjectInput['visibility'];
}

@Component({
  selector: 'app-projects-page',
  imports: [
    CdkTrapFocus,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RouterLink,
  ],
  providers: [ProjectsStore],
  template: `
    <div class="page projects-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('projects.title') }}</h1>
          <p>{{ i18n.t('projects.subtitle') }}</p>
        </div>
        @if (permissions.allows('records.create')) {
          <button #projectCreateTrigger mat-flat-button type="button" (click)="openCreate()">
            <app-icon name="add" />{{ i18n.t('projects.add') }}
          </button>
        }
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      <section class="project-grid" [attr.aria-busy]="store.loading()">
        @for (project of store.items(); track project.id) {
          <a class="panel project-card" [routerLink]="['/projects', project.id]">
            <header>
              <span class="project-mark" aria-hidden="true"><app-icon name="project" /></span>
              <span class="status-pill">{{ i18n.t(statusKey(project.status)) }}</span>
            </header>
            <h2>{{ project.name }}</h2>
            <p>{{ project.description || '—' }}</p>
            <footer>
              <span>{{ i18n.t(visibilityKey(project.visibility)) }}</span>
              @if (project.targetEndDate) {
                <time [attr.datetime]="project.targetEndDate">{{
                  i18n.date(project.targetEndDate)
                }}</time>
              }
            </footer>
          </a>
        } @empty {
          @if (!store.loading()) {
            <div class="empty-state panel">{{ i18n.t('projects.empty') }}</div>
          }
        }
      </section>
      @if (store.nextCursor()) {
        <button mat-button type="button" (click)="store.load(false)">
          {{ i18n.t('common.action.viewAll') }}
        </button>
      }
    </div>

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
        aria-labelledby="project-create-title"
      >
        <header>
          <h2 id="project-create-title">{{ i18n.t('projects.createTitle') }}</h2>
          <button
            mat-icon-button
            type="button"
            (click)="closeCreate()"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>
        <form (submit)="create($event)" novalidate>
          <mat-form-field appearance="outline">
            <mat-label>{{ i18n.t('common.field.name') }}</mat-label>
            <input
              #projectNameInput
              matInput
              [formField]="projectForm.name"
              [attr.aria-invalid]="createAttempted() && projectForm.name().invalid()"
              [attr.aria-describedby]="
                createAttempted() && projectForm.name().invalid() ? 'project-name-error' : null
              "
            />
          </mat-form-field>
          @if (createAttempted() && projectForm.name().invalid()) {
            <p class="field-error" id="project-name-error" role="alert">
              {{ i18n.t('common.validation.name') }}
            </p>
          }
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('projects.description') }}</mat-label
            ><textarea
              matInput
              rows="4"
              [formField]="projectForm.description"
              [attr.aria-invalid]="createAttempted() && projectForm.description().invalid()"
              [attr.aria-describedby]="
                createAttempted() && projectForm.description().invalid()
                  ? 'project-description-error'
                  : null
              "
            ></textarea>
          </mat-form-field>
          @if (createAttempted() && projectForm.description().invalid()) {
            <p class="field-error" id="project-description-error" role="alert">
              {{ i18n.t('projects.error.descriptionLength') }}
            </p>
          }
          <div class="date-grid">
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('projects.plannedStart') }}</mat-label
              ><input
                matInput
                type="date"
                [formField]="projectForm.plannedStartDate"
                [attr.aria-invalid]="dateRangeInvalid() ? 'true' : null"
                [attr.aria-describedby]="dateRangeInvalid() ? 'project-date-error' : null"
            /></mat-form-field>
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('projects.targetEnd') }}</mat-label
              ><input
                matInput
                type="date"
                [formField]="projectForm.targetEndDate"
                [attr.aria-invalid]="dateRangeInvalid() ? 'true' : null"
                [attr.aria-describedby]="dateRangeInvalid() ? 'project-date-error' : null"
            /></mat-form-field>
          </div>
          <label class="native-field"
            ><span>{{ i18n.t('projects.visibility.workspace') }}</span
            ><select [formField]="projectForm.visibility">
              <option value="workspace">{{ i18n.t('projects.visibility.workspace') }}</option>
              <option value="restricted">{{ i18n.t('projects.visibility.restricted') }}</option>
            </select></label
          >
          @if (createError()) {
            <div class="form-error" id="project-date-error" role="alert">
              {{ createError() }}
            </div>
          }
          <div class="drawer-actions">
            <button mat-button type="button" (click)="closeCreate()">
              {{ i18n.t('common.action.cancel') }}
            </button>
            <button mat-flat-button type="submit" [disabled]="store.saving()">
              {{ i18n.t('common.action.create') }}
            </button>
          </div>
        </form>
      </aside>
    }
  `,
  styleUrl: './projects.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectsPage implements OnInit {
  readonly store = inject(ProjectsStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly createOpen = signal(false);
  readonly createError = signal<string | null>(null);
  readonly dateRangeInvalid = signal(false);
  readonly createAttempted = signal(false);
  readonly projectModel = signal<ProjectCreateModel>({
    name: '',
    description: '',
    plannedStartDate: '',
    targetEndDate: '',
    visibility: 'workspace',
  });
  readonly projectForm = form(this.projectModel, (schema) => {
    required(schema.name);
    maxLength(schema.name, 200);
    maxLength(schema.description, 20_000);
  });
  readonly projectCreateTrigger = viewChild<ElementRef<HTMLButtonElement>>('projectCreateTrigger');
  readonly projectNameInput = viewChild<ElementRef<HTMLInputElement>>('projectNameInput');
  private readonly injector = inject(Injector);

  ngOnInit(): void {
    void this.store.load();
  }

  statusKey(status: ProjectInput['status']): `projects.status.${ProjectInput['status']}` {
    return `projects.status.${status}`;
  }

  visibilityKey(
    visibility: ProjectInput['visibility'],
  ): `projects.visibility.${ProjectInput['visibility']}` {
    return `projects.visibility.${visibility}`;
  }

  openCreate(): void {
    this.createError.set(null);
    this.dateRangeInvalid.set(false);
    this.createAttempted.set(false);
    this.createOpen.set(true);
    focusAfterNextRender(this.injector, () => this.projectNameInput()?.nativeElement);
  }

  closeCreate(): void {
    this.createOpen.set(false);
    this.createError.set(null);
    this.dateRangeInvalid.set(false);
    this.createAttempted.set(false);
    focusAfterNextRender(this.injector, () => this.projectCreateTrigger()?.nativeElement);
  }

  @HostListener('document:keydown.escape')
  closeCreateFromKeyboard(): void {
    if (this.createOpen()) this.closeCreate();
  }

  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.createError.set(null);
    this.dateRangeInvalid.set(false);
    this.createAttempted.set(true);
    this.projectModel.update((value) => ({
      ...value,
      name: value.name.trim(),
      description: value.description.trim(),
    }));
    if (this.projectForm().invalid()) {
      this.projectForm.name().markAsTouched();
      this.projectForm.description().markAsTouched();
      this.projectForm.plannedStartDate().markAsTouched();
      this.projectForm.targetEndDate().markAsTouched();
      this.projectForm.visibility().markAsTouched();
      return;
    }
    const value = this.projectModel();
    if (
      value.plannedStartDate &&
      value.targetEndDate &&
      value.plannedStartDate > value.targetEndDate
    ) {
      this.dateRangeInvalid.set(true);
      this.createError.set(this.i18n.t('projects.error.dateRange'));
      return;
    }
    try {
      const created = await this.store.create({
        name: value.name,
        description: value.description,
        status: 'planned',
        visibility: value.visibility,
        plannedStartDate: value.plannedStartDate || null,
        targetEndDate: value.targetEndDate || null,
        ownerUserId: null,
      });
      if (!created) {
        this.createError.set(this.i18n.t('projects.error.workspaceRequired'));
        return;
      }
      this.closeCreate();
      this.projectModel.set({
        name: '',
        description: '',
        plannedStartDate: '',
        targetEndDate: '',
        visibility: 'workspace',
      });
    } catch (error) {
      this.createError.set(apiErrorMessage(this.i18n, error, 'validation'));
    }
  }
}
