import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, input, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { Router, RouterLink } from '@angular/router';

import type {
  Activity,
  ProjectAssignment,
  ProjectAssignmentInput,
  ProjectInput,
} from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { AttachmentPanelComponent } from '../../shared/attachments/attachment-panel.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ProjectDetailsStore } from './project-details.store';

interface ProjectEditorModel {
  readonly name: string;
  readonly description: string;
  readonly status: ProjectInput['status'];
  readonly visibility: ProjectInput['visibility'];
  readonly plannedStartDate: string;
  readonly targetEndDate: string;
}

interface ProjectTaskModel {
  readonly title: string;
  readonly body: string;
  readonly dueAt: string;
  readonly assigneeId: string;
  readonly priority: 'low' | 'normal' | 'high';
}

interface AssignmentEditorModel {
  readonly kind: ProjectAssignmentInput['kind'];
  readonly subjectType: ProjectAssignmentInput['subjectType'];
  readonly subjectId: string;
}

@Component({
  selector: 'app-project-details-page',
  imports: [
    AttachmentPanelComponent,
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    RouterLink,
  ],
  providers: [ProjectDetailsStore],
  template: `
    <div class="page project-details">
      <a class="back-link" routerLink="/projects"
        ><app-icon name="back" />{{ i18n.t('projects.back') }}</a
      >

      @if (store.loading()) {
        <div class="panel skeleton hero-skeleton"></div>
      } @else if (store.error() && !store.project()) {
        <app-error-panel [error]="store.error()" (retry)="load()" />
      } @else if (store.project(); as project) {
        <header class="panel project-hero">
          <span class="project-mark" aria-hidden="true"><app-icon name="project" /></span>
          <div>
            <p class="eyebrow">{{ i18n.t('projects.title') }}</p>
            <h1>{{ project.name }}</h1>
            <p>{{ project.description || '—' }}</p>
          </div>
          <span class="status-pill">{{ i18n.t(statusKey(project.status)) }}</span>
        </header>

        @if (store.conflict()) {
          <section class="conflict" role="alert">
            <span>{{ i18n.t('projects.error.conflict') }}</span>
            <button mat-button type="button" (click)="load()">
              {{ i18n.t('common.action.retry') }}
            </button>
          </section>
        }
        @if (store.error()) {
          <app-error-panel [error]="store.error()" (retry)="load()" />
        }

        <div class="project-layout">
          <div class="main-stack">
            <section class="panel section-panel" aria-labelledby="project-overview-title">
              <header>
                <h2 id="project-overview-title">{{ i18n.t('projects.overview') }}</h2>
              </header>
              <form class="project-form" (submit)="save($event)">
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('common.field.name') }}</mat-label
                  ><input matInput [formField]="projectForm.name"
                /></mat-form-field>
                <mat-form-field appearance="outline"
                  ><mat-label>{{ i18n.t('projects.description') }}</mat-label
                  ><textarea matInput rows="3" [formField]="projectForm.description"></textarea>
                </mat-form-field>
                <div class="form-grid">
                  <label class="native-field"
                    ><span>{{ i18n.t('common.field.status') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('common.field.status')"
                      [formField]="projectForm.status"
                    >
                      <mat-option value="planned">{{
                        i18n.t('projects.status.planned')
                      }}</mat-option>
                      <mat-option value="active">{{ i18n.t('projects.status.active') }}</mat-option>
                      <mat-option value="on_hold">{{
                        i18n.t('projects.status.on_hold')
                      }}</mat-option>
                      <mat-option value="completed">{{
                        i18n.t('projects.status.completed')
                      }}</mat-option>
                      <mat-option value="archived">{{
                        i18n.t('projects.status.archived')
                      }}</mat-option>
                    </mat-select></label
                  >
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.visibility.workspace') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('projects.visibility.workspace')"
                      [formField]="projectForm.visibility"
                    >
                      <mat-option value="workspace">
                        {{ i18n.t('projects.visibility.workspace') }}
                      </mat-option>
                      <mat-option value="restricted">
                        {{ i18n.t('projects.visibility.restricted') }}
                      </mat-option>
                    </mat-select></label
                  >
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.plannedStart') }}</span
                    ><input type="date" [formField]="projectForm.plannedStartDate"
                  /></label>
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.targetEnd') }}</span
                    ><input type="date" [formField]="projectForm.targetEndDate"
                  /></label>
                </div>
                @if (project.capabilities.canEdit) {
                  <div class="form-actions">
                    <button mat-flat-button type="submit" [disabled]="store.saving()">
                      {{ i18n.t(store.saving() ? 'web.form.saving' : 'common.action.save') }}
                    </button>
                  </div>
                }
              </form>
            </section>

            <section class="panel section-panel task-section" aria-labelledby="project-tasks-title">
              <header>
                <h2 id="project-tasks-title">{{ i18n.t('projects.tasks.title') }}</h2>
              </header>
              @if (project.capabilities.canEdit) {
                <form class="task-form" (submit)="addTask($event)">
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('projects.tasks.titleField') }}</mat-label
                    ><input matInput [formField]="taskForm.title"
                  /></mat-form-field>
                  <mat-form-field appearance="outline"
                    ><mat-label>{{ i18n.t('projects.tasks.body') }}</mat-label
                    ><textarea matInput rows="2" [formField]="taskForm.body"></textarea>
                  </mat-form-field>
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.tasks.due') }}</span
                    ><input type="datetime-local" [formField]="taskForm.dueAt"
                  /></label>
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.tasks.assignee') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('projects.tasks.assignee')"
                      [formField]="taskForm.assigneeId"
                    >
                      <mat-option value="">—</mat-option>
                      @for (member of store.members(); track member.id) {
                        <mat-option [value]="member.userId">{{ member.displayName }}</mat-option>
                      }
                    </mat-select></label
                  >
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.tasks.priority.title') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('projects.tasks.priority.title')"
                      [formField]="taskForm.priority"
                    >
                      <mat-option value="low">{{
                        i18n.t('projects.tasks.priority.low')
                      }}</mat-option>
                      <mat-option value="normal">{{
                        i18n.t('projects.tasks.priority.normal')
                      }}</mat-option>
                      <mat-option value="high">{{
                        i18n.t('projects.tasks.priority.high')
                      }}</mat-option>
                    </mat-select></label
                  >
                  <div class="form-actions">
                    <button mat-flat-button type="submit" [disabled]="store.saving()">
                      <app-icon name="add" />{{ i18n.t('projects.tasks.add') }}
                    </button>
                  </div>
                </form>
              }
              <div class="task-list">
                @for (task of store.activities(); track task.id) {
                  <article [class.completed]="task.status === 'completed'">
                    <span class="task-check" aria-hidden="true">{{
                      task.status === 'completed' ? '✓' : ''
                    }}</span>
                    <div>
                      <h3>{{ task.title }}</h3>
                      @if (task.body) {
                        <p>{{ task.body }}</p>
                      }
                      <footer>
                        <span>{{ i18n.t(taskStatusKey(task)) }}</span>
                        @if (task.dueAt) {
                          <time [attr.datetime]="task.dueAt">{{
                            i18n.date(task.dueAt, { dateStyle: 'medium', timeStyle: 'short' })
                          }}</time>
                        }
                      </footer>
                    </div>
                    @if (
                      task.type === 'task' && task.status === 'open' && project.capabilities.canEdit
                    ) {
                      <button mat-button type="button" (click)="completeTask(task)">
                        {{ i18n.t('projects.tasks.complete') }}
                      </button>
                    }
                  </article>
                } @empty {
                  <div class="empty-state">{{ i18n.t('projects.tasks.empty') }}</div>
                }
              </div>
            </section>

            <app-attachment-panel entityType="project" [entityId]="project.id" />
          </div>

          <aside class="side-stack">
            <section class="panel section-panel" aria-labelledby="project-team-title">
              <header>
                <h2 id="project-team-title">{{ i18n.t('projects.assignments.title') }}</h2>
              </header>
              <ul class="assignment-list">
                @for (assignment of store.assignments(); track assignment.id) {
                  <li>
                    <span class="avatar">{{ initials(assignment.displayName) }}</span>
                    <div>
                      <strong>{{ assignment.displayName }}</strong
                      ><small
                        >{{ i18n.t(assignmentKindKey(assignment)) }} ·
                        {{ i18n.t(assignmentSubjectKey(assignment)) }}</small
                      >
                    </div>
                    @if (project.capabilities.canManage) {
                      <button mat-button type="button" (click)="removeAssignment(assignment.id)">
                        {{ i18n.t('common.action.delete') }}
                      </button>
                    }
                  </li>
                } @empty {
                  <li class="empty-state">{{ i18n.t('projects.assignments.empty') }}</li>
                }
              </ul>
              @if (project.capabilities.canManage) {
                <form class="assignment-form" (submit)="addAssignment($event)">
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.assignments.kind') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('projects.assignments.kind')"
                      [formField]="assignmentForm.kind"
                    >
                      <mat-option value="responsible">
                        {{ i18n.t('projects.assignments.responsible') }}
                      </mat-option>
                      <mat-option value="watcher">{{
                        i18n.t('projects.assignments.watcher')
                      }}</mat-option>
                    </mat-select></label
                  >
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.assignments.subject') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('projects.assignments.subject')"
                      [formField]="assignmentForm.subjectType"
                      (selectionChange)="
                        assignmentModel.update((value) => ({ ...value, subjectId: '' }))
                      "
                    >
                      <mat-option value="user">{{
                        i18n.t('projects.assignments.user')
                      }}</mat-option>
                      <mat-option value="department">
                        {{ i18n.t('projects.assignments.department') }}
                      </mat-option>
                    </mat-select></label
                  >
                  <label class="native-field"
                    ><span>{{ i18n.t('projects.assignments.subject') }}</span
                    ><mat-select
                      panelClass="crm-select-panel"
                      [aria-label]="i18n.t('projects.assignments.subject')"
                      [formField]="assignmentForm.subjectId"
                    >
                      <mat-option value="">—</mat-option>
                      @if (assignmentModel().subjectType === 'user') {
                        @for (member of store.members(); track member.id) {
                          <mat-option [value]="member.userId">{{ member.displayName }}</mat-option>
                        }
                      } @else {
                        @for (department of store.departments(); track department.id) {
                          <mat-option [value]="department.id">{{ department.name }}</mat-option>
                        }
                      }
                    </mat-select></label
                  >
                  <button mat-stroked-button type="submit" [disabled]="store.saving()">
                    <app-icon name="add" />{{ i18n.t('projects.assignments.add') }}
                  </button>
                </form>
              }
            </section>

            @if (project.capabilities.canManage) {
              <section class="panel danger-zone">
                <p>{{ i18n.t('projects.deleteConfirm') }}</p>
                <button mat-button type="button" class="danger-action" (click)="deleteProject()">
                  {{ i18n.t('projects.delete') }}
                </button>
              </section>
            }
          </aside>
        </div>
      }
    </div>
  `,
  styleUrl: './project-details.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectDetailsPage implements OnInit {
  readonly id = input.required<string>();
  readonly store = inject(ProjectDetailsStore);
  readonly i18n = inject(I18nService);
  private readonly router = inject(Router);

  readonly projectModel = signal<ProjectEditorModel>({
    name: '',
    description: '',
    status: 'planned',
    visibility: 'workspace',
    plannedStartDate: '',
    targetEndDate: '',
  });
  readonly projectForm = form(this.projectModel, (schema) => required(schema.name));
  readonly taskModel = signal<ProjectTaskModel>({
    title: '',
    body: '',
    dueAt: '',
    assigneeId: '',
    priority: 'normal',
  });
  readonly taskForm = form(this.taskModel, (schema) => required(schema.title));
  readonly assignmentModel = signal<AssignmentEditorModel>({
    kind: 'responsible',
    subjectType: 'user',
    subjectId: '',
  });
  readonly assignmentForm = form(this.assignmentModel, (schema) => required(schema.subjectId));

  ngOnInit(): void {
    void this.load();
  }

  async load(): Promise<void> {
    await this.store.load(this.id());
    const project = this.store.project();
    if (project)
      this.projectModel.set({
        name: project.name,
        description: project.description,
        status: project.status,
        visibility: project.visibility,
        plannedStartDate: project.plannedStartDate ?? '',
        targetEndDate: project.targetEndDate ?? '',
      });
  }

  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.projectForm().invalid()) {
      this.projectForm().markAsTouched();
      return;
    }
    const value = this.projectModel();
    await this.store.save({
      name: value.name.trim(),
      description: value.description.trim(),
      status: value.status,
      visibility: value.visibility,
      plannedStartDate: value.plannedStartDate || null,
      targetEndDate: value.targetEndDate || null,
      ownerUserId: this.store.project()?.ownerUserId ?? null,
    });
  }

  async addTask(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.taskForm().invalid()) {
      this.taskForm().markAsTouched();
      return;
    }
    const value = this.taskModel();
    await this.store.addTask({
      type: 'task',
      title: value.title.trim(),
      body: value.body.trim() || null,
      relatedType: 'project',
      relatedId: this.id(),
      assigneeId: value.assigneeId || null,
      priority: value.priority,
      dueAt: value.dueAt ? new Date(value.dueAt).toISOString() : null,
    });
    this.taskModel.set({ title: '', body: '', dueAt: '', assigneeId: '', priority: 'normal' });
  }

  async completeTask(task: Activity): Promise<void> {
    await this.store.completeTask(task);
  }

  async addAssignment(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.assignmentForm().invalid()) {
      this.assignmentForm().markAsTouched();
      return;
    }
    const value = this.assignmentModel();
    await this.store.addAssignment(value);
    this.assignmentModel.update((current) => ({ ...current, subjectId: '' }));
  }

  async removeAssignment(id: string): Promise<void> {
    await this.store.removeAssignment(id);
  }

  async deleteProject(): Promise<void> {
    if (!window.confirm(this.i18n.t('projects.deleteConfirm'))) return;
    await this.store.remove();
    await this.router.navigateByUrl('/projects');
  }

  statusKey(status: ProjectInput['status']): `projects.status.${ProjectInput['status']}` {
    return `projects.status.${status}`;
  }
  taskStatusKey(task: Activity): 'projects.tasks.status.completed' | 'projects.tasks.status.open' {
    return task.status === 'completed'
      ? 'projects.tasks.status.completed'
      : 'projects.tasks.status.open';
  }
  assignmentKindKey(
    assignment: ProjectAssignment,
  ): 'projects.assignments.responsible' | 'projects.assignments.watcher' {
    return assignment.kind === 'responsible'
      ? 'projects.assignments.responsible'
      : 'projects.assignments.watcher';
  }
  assignmentSubjectKey(
    assignment: ProjectAssignment,
  ): 'projects.assignments.user' | 'projects.assignments.department' {
    return assignment.subjectType === 'user'
      ? 'projects.assignments.user'
      : 'projects.assignments.department';
  }
  initials(name: string): string {
    return name
      .split(/\s+/u)
      .slice(0, 2)
      .map((part) => part.slice(0, 1).toUpperCase())
      .join('');
  }
}
