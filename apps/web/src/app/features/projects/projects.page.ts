import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';

import type { ProjectInput } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
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
        <button mat-flat-button type="button" (click)="createOpen.set(true)">
          <app-icon name="add" />{{ i18n.t('projects.add') }}
        </button>
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
        (click)="createOpen.set(false)"
        [attr.aria-label]="i18n.t('common.action.close')"
      ></button>
      <aside
        class="create-drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="project-create-title"
      >
        <header>
          <h2 id="project-create-title">{{ i18n.t('projects.createTitle') }}</h2>
          <button
            mat-icon-button
            type="button"
            (click)="createOpen.set(false)"
            [attr.aria-label]="i18n.t('common.action.close')"
          >
            <app-icon name="close" />
          </button>
        </header>
        <form (submit)="create($event)">
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('common.field.name') }}</mat-label
            ><input matInput [formField]="projectForm.name"
          /></mat-form-field>
          <mat-form-field appearance="outline"
            ><mat-label>{{ i18n.t('projects.description') }}</mat-label
            ><textarea matInput rows="4" [formField]="projectForm.description"></textarea>
          </mat-form-field>
          <div class="date-grid">
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('projects.plannedStart') }}</mat-label
              ><input matInput type="date" [formField]="projectForm.plannedStartDate"
            /></mat-form-field>
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('projects.targetEnd') }}</mat-label
              ><input matInput type="date" [formField]="projectForm.targetEndDate"
            /></mat-form-field>
          </div>
          <label class="native-field"
            ><span>{{ i18n.t('projects.visibility.workspace') }}</span
            ><select [formField]="projectForm.visibility">
              <option value="workspace">{{ i18n.t('projects.visibility.workspace') }}</option>
              <option value="restricted">{{ i18n.t('projects.visibility.restricted') }}</option>
            </select></label
          >
          <div class="drawer-actions">
            <button mat-button type="button" (click)="createOpen.set(false)">
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
  readonly createOpen = signal(false);
  readonly projectModel = signal<ProjectCreateModel>({
    name: '',
    description: '',
    plannedStartDate: '',
    targetEndDate: '',
    visibility: 'workspace',
  });
  readonly projectForm = form(this.projectModel, (schema) => required(schema.name));

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

  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.projectForm().invalid()) {
      this.projectForm().markAsTouched();
      return;
    }
    const value = this.projectModel();
    await this.store.create({
      name: value.name.trim(),
      description: value.description.trim(),
      status: 'planned',
      visibility: value.visibility,
      plannedStartDate: value.plannedStartDate || null,
      targetEndDate: value.targetEndDate || null,
      ownerUserId: null,
    });
    this.createOpen.set(false);
    this.projectModel.set({
      name: '',
      description: '',
      plannedStartDate: '',
      targetEndDate: '',
      visibility: 'workspace',
    });
  }
}
