import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type { Activity, CreateActivity } from '../../core/api/api.types';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { RecordAssignmentsComponent } from '../../shared/assignments/record-assignments.component';
import { IconComponent } from '../../shared/icon/icon.component';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ActivitiesStore } from './activities.store';

@Component({
  selector: 'app-activities-page',
  imports: [
    ErrorPanelComponent,
    FormField,
    IconComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    RecordAssignmentsComponent,
  ],
  providers: [ActivitiesStore],
  template: `
    <div class="page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('common.nav.activities') }}</h1>
          <p>{{ i18n.plural('common.resultCount', store.activities().length) }}</p>
        </div>
        <button mat-flat-button type="button" (click)="createOpen.set(!createOpen())">
          <app-icon name="add" />{{ i18n.t('activities.activity.add') }}
        </button>
      </header>
      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }
      @if (createOpen()) {
        <section class="panel create-panel">
          <form (submit)="create($event)">
            <label
              >{{ i18n.t('web.activity.type')
              }}<select [formField]="activityForm.type">
                <option value="task">{{ i18n.t('activities.activity.task') }}</option>
                <option value="call">{{ i18n.t('activities.activity.call') }}</option>
                <option value="meeting">{{ i18n.t('activities.activity.meeting') }}</option>
                <option value="note">{{ i18n.t('activities.activity.note') }}</option>
              </select></label
            >
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('activities.activity.title') }}</mat-label
              ><input matInput [formField]="activityForm.title"
            /></mat-form-field>
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('activities.activity.body') }}</mat-label
              ><textarea matInput rows="2" [formField]="activityForm.body"></textarea>
            </mat-form-field>
            <mat-form-field appearance="outline"
              ><mat-label>{{ i18n.t('activities.activity.due') }}</mat-label
              ><input matInput type="datetime-local" [formField]="activityForm.dueAt"
            /></mat-form-field>
            <label
              >{{ i18n.t('activities.activity.priority')
              }}<select [formField]="activityForm.priority">
                <option value="low">{{ i18n.t('activities.priority.low') }}</option>
                <option value="normal">{{ i18n.t('activities.priority.normal') }}</option>
                <option value="high">{{ i18n.t('activities.priority.high') }}</option>
              </select></label
            >
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
      <section class="panel activity-list">
        @if (store.loading()) {
          <div class="list-skeleton">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else if (store.activities().length === 0) {
          <div class="empty-state">{{ i18n.t('dashboard.dashboard.emptyActivity') }}</div>
        } @else {
          @for (activity of store.activities(); track activity.id) {
            <article>
              <span class="type-mark" aria-hidden="true"><app-icon name="activity" /></span>
              <div>
                <header>
                  <h2>{{ activity.title }}</h2>
                  <span>{{ i18n.t(typeKey(activity)) }}</span>
                </header>
                @if (activity.body) {
                  <p>{{ activity.body }}</p>
                }
                <footer>
                  <time [attr.datetime]="activity.dueAt || activity.occurredAt">{{
                    i18n.date(activity.dueAt || activity.occurredAt, {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })
                  }}</time
                  ><span>{{
                    i18n.t(
                      activity.status === 'completed'
                        ? 'common.status.completed'
                        : 'common.status.open'
                    )
                  }}</span>
                  @if (activity.type === 'task' && activity.status === 'open') {
                    <button mat-button type="button" (click)="store.complete(activity)">
                      {{ i18n.t('activities.activity.complete') }}
                    </button>
                  }
                  @if (activity.type === 'task') {
                    <button mat-button type="button" (click)="toggleAssignments(activity)">
                      {{ i18n.t('assignments.manage') }}
                    </button>
                  }
                </footer>
                @if (selectedTaskId() === activity.id) {
                  <div class="task-assignments">
                    <app-record-assignments
                      resourceType="task"
                      [resourceId]="activity.id"
                      [version]="activity.version"
                      (versionChange)="store.setVersion(activity.id, $event)"
                    />
                  </div>
                }
              </div>
            </article>
          }
        }
      </section>
    </div>
  `,
  styles: `
    .create-panel {
      padding: 1rem;
    }
    .create-panel form {
      display: grid;
      grid-template-columns: 10rem minmax(12rem, 1fr) minmax(12rem, 1.2fr) minmax(13rem, 1fr) 9rem;
      align-items: start;
      gap: 0.75rem;
    }
    .create-panel label {
      display: grid;
      gap: 0.35rem;
      color: var(--text-muted);
      font-size: 0.72rem;
    }
    .create-panel select {
      min-height: 3.5rem;
      padding: 0 0.6rem;
      border: 1px solid var(--border);
      border-radius: 0.3rem;
      color: var(--text);
      background: var(--surface-raised);
    }
    .form-actions {
      grid-column: 1 / -1;
      display: flex;
      justify-content: flex-end;
      gap: 0.5rem;
    }
    .activity-list {
      overflow: hidden;
    }
    .activity-list > article {
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 0.8rem;
      padding: 1rem;
      border-bottom: 1px solid var(--border);
    }
    .activity-list > article:last-child {
      border: 0;
    }
    .type-mark {
      display: grid;
      place-items: center;
      width: 2rem;
      height: 2rem;
      border-radius: 0.55rem;
      color: var(--brand);
      background: var(--brand-soft);
    }
    .activity-list article header,
    .activity-list footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
    }
    .task-assignments {
      margin-top: 0.8rem;
      padding: 0.8rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      background: var(--surface-subtle);
    }
    .activity-list h2 {
      margin: 0;
      font-size: 0.9rem;
    }
    .activity-list header span {
      color: var(--text-faint);
      font-size: 0.7rem;
      text-transform: uppercase;
    }
    .activity-list p {
      margin: 0.4rem 0;
      color: var(--text-muted);
    }
    .activity-list footer {
      justify-content: flex-start;
      color: var(--text-faint);
      font-size: 0.72rem;
    }
    .list-skeleton {
      display: grid;
      gap: 0.75rem;
      padding: 1rem;
    }
    .list-skeleton > div {
      min-height: 4rem;
    }
    @media (max-width: 1050px) {
      .create-panel form {
        grid-template-columns: 1fr 1fr;
      }
    }
    .form-actions {
      grid-column: 1 / -1;
    }
    @media (max-width: 600px) {
      .create-panel form {
        grid-template-columns: 1fr;
      }
      .form-actions {
        grid-column: 1;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ActivitiesPage implements OnInit {
  readonly store = inject(ActivitiesStore);
  readonly i18n = inject(I18nService);
  readonly createOpen = signal(false);
  readonly selectedTaskId = signal<string | null>(null);
  readonly model = signal<{
    type: CreateActivity['type'];
    title: string;
    body: string;
    dueAt: string;
    priority: CreateActivity['priority'];
  }>({ type: 'task', title: '', body: '', dueAt: '', priority: 'normal' });
  readonly activityForm = form(this.model, (schema) => required(schema.title));
  ngOnInit(): void {
    void this.store.load();
  }
  typeKey(activity: Activity): AppMessageKey {
    return `activities.activity.${activity.type}` as AppMessageKey;
  }
  toggleAssignments(activity: Activity): void {
    this.selectedTaskId.update((current) => (current === activity.id ? null : activity.id));
  }
  async create(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.activityForm().invalid()) {
      this.activityForm().markAsTouched();
      return;
    }
    const value = this.model();
    const body: CreateActivity = {
      type: value.type,
      title: value.title.trim(),
      body: value.body.trim() || null,
      priority: value.priority,
      dueAt: value.dueAt ? new Date(value.dueAt).toISOString() : null,
    };
    await this.store.create(body);
    this.model.set({ type: 'task', title: '', body: '', dueAt: '', priority: 'normal' });
    this.createOpen.set(false);
  }
}
