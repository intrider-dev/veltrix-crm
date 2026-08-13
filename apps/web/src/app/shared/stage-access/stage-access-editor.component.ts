import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  StageAccessRequest,
  StageAccessRule,
  WorkspaceRoleDefinition,
} from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../feedback/toast.service';
import { ErrorPanelComponent } from '../state/error-panel.component';
import { isStageAccessRoleEditable } from './stage-access-role';

interface StageAccessDraft {
  readonly role: WorkspaceRoleDefinition;
  readonly explicit: boolean;
  readonly canView: boolean;
  readonly canEnter: boolean;
  readonly canLeave: boolean;
}

@Component({
  selector: 'app-stage-access-editor',
  imports: [ErrorPanelComponent, MatButtonModule, MatCheckboxModule],
  template: `
    <section class="panel access-editor" aria-labelledby="stage-access-title">
      <header>
        <div>
          <h2 id="stage-access-title">{{ i18n.t('stageAccess.title', { name: stageName() }) }}</h2>
          <p>{{ i18n.t('stageAccess.subtitle') }}</p>
        </div>
        <button mat-button type="button" (click)="closed.emit()">
          {{ i18n.t('common.action.close') }}
        </button>
      </header>

      @if (error()) {
        <app-error-panel [error]="error()" (retry)="load()" />
      }

      @if (loading()) {
        <p class="loading" role="status">{{ i18n.t('common.app.loading') }}</p>
      } @else {
        <div class="rule-table" role="table" [attr.aria-label]="i18n.t('stageAccess.tableLabel')">
          <div class="rule-header" role="row">
            <span role="columnheader">{{ i18n.t('stageAccess.role') }}</span>
            <span role="columnheader">{{ i18n.t('stageAccess.restrict') }}</span>
            <span role="columnheader">{{ i18n.t('stageAccess.view') }}</span>
            <span role="columnheader">{{ i18n.t('stageAccess.enter') }}</span>
            <span role="columnheader">{{ i18n.t('stageAccess.leave') }}</span>
          </div>
          @for (draft of drafts(); track draft.role.id; let index = $index) {
            <div class="rule-row" role="row">
              <div role="cell">
                <strong>{{ draft.role.name }}</strong>
                <small>{{ i18n.t(baseRoleKey(draft.role.baseRole)) }}</small>
              </div>
              <mat-checkbox
                role="cell"
                [checked]="draft.explicit"
                [attr.aria-label]="i18n.t('stageAccess.restrictLabel', { role: draft.role.name })"
                (change)="setFlag(index, 'explicit', $event.checked)"
              />
              <mat-checkbox
                role="cell"
                [checked]="draft.canView"
                [disabled]="!draft.explicit"
                [attr.aria-label]="i18n.t('stageAccess.viewLabel', { role: draft.role.name })"
                (change)="setFlag(index, 'canView', $event.checked)"
              />
              <mat-checkbox
                role="cell"
                [checked]="draft.canEnter"
                [disabled]="!draft.explicit"
                [attr.aria-label]="i18n.t('stageAccess.enterLabel', { role: draft.role.name })"
                (change)="setFlag(index, 'canEnter', $event.checked)"
              />
              <mat-checkbox
                role="cell"
                [checked]="draft.canLeave"
                [disabled]="!draft.explicit"
                [attr.aria-label]="i18n.t('stageAccess.leaveLabel', { role: draft.role.name })"
                (change)="setFlag(index, 'canLeave', $event.checked)"
              />
            </div>
          } @empty {
            <p class="empty-state">{{ i18n.t('stageAccess.empty') }}</p>
          }
        </div>
        <p class="inheritance-note">{{ i18n.t('stageAccess.inheritance') }}</p>
        <div class="actions">
          <button mat-flat-button type="button" [disabled]="saving()" (click)="save()">
            {{ i18n.t('common.action.save') }}
          </button>
        </div>
      }
    </section>
  `,
  styles: `
    .access-editor {
      overflow: hidden;
    }
    header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 1rem;
      padding: 1rem;
      border-bottom: 1px solid var(--border);
    }
    h2,
    p {
      margin: 0;
    }
    h2 {
      font-size: 1rem;
    }
    header p,
    .inheritance-note,
    small {
      color: var(--text-muted);
    }
    header p {
      margin-top: 0.25rem;
      font-size: 0.85rem;
    }
    .loading,
    .inheritance-note,
    .actions {
      padding: 0.85rem 1rem;
    }
    .rule-header,
    .rule-row {
      display: grid;
      grid-template-columns: minmax(12rem, 1fr) repeat(4, minmax(5rem, 0.35fr));
      align-items: center;
      min-height: 3.25rem;
      padding: 0 1rem;
      border-bottom: 1px solid var(--border);
    }
    .rule-header {
      min-height: 2.5rem;
      color: var(--text-muted);
      background: var(--surface-subtle);
      font-size: 0.75rem;
      font-weight: 650;
    }
    .rule-row > div {
      display: grid;
      gap: 0.15rem;
    }
    .rule-row mat-checkbox,
    .rule-header span:not(:first-child) {
      justify-self: center;
    }
    .inheritance-note {
      font-size: 0.8rem;
    }
    .actions {
      display: flex;
      justify-content: flex-end;
      padding-top: 0;
    }
    @media (max-width: 760px) {
      .rule-table {
        overflow-x: auto;
      }
      .rule-header,
      .rule-row {
        min-width: 42rem;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StageAccessEditorComponent implements OnInit {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly toast = inject(ToastService);

  readonly i18n = inject(I18nService);
  readonly stageId = input.required<string>();
  readonly stageName = input.required<string>();
  readonly kind = input.required<'lead' | 'deal'>();
  readonly closed = output<void>();
  readonly drafts = signal<readonly StageAccessDraft[]>([]);
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  ngOnInit(): void {
    void this.load();
  }

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [roles, rules] = await Promise.all([
        this.api.listWorkspaceRoles(workspaceId),
        this.kind() === 'lead'
          ? this.api.listLeadStageAccess(workspaceId, this.stageId())
          : this.api.listPipelineStageAccess(workspaceId, this.stageId()),
      ]);
      const byRole = new Map(rules.map((rule) => [rule.roleId, rule]));
      this.drafts.set(
        roles
          .filter(isStageAccessRoleEditable)
          .map((role) => stageAccessDraft(role, byRole.get(role.id))),
      );
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  setFlag(
    index: number,
    flag: 'explicit' | 'canView' | 'canEnter' | 'canLeave',
    value: boolean,
  ): void {
    this.drafts.update((drafts) =>
      drafts.map((draft, draftIndex) => {
        if (draftIndex !== index) return draft;
        if (flag === 'explicit' && !value) {
          return { ...draft, explicit: false, canView: false, canEnter: false, canLeave: false };
        }
        return { ...draft, [flag]: value };
      }),
    );
  }

  async save(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const body: StageAccessRequest = {
      rules: this.drafts()
        .filter((draft) => draft.explicit)
        .map((draft) => ({
          roleId: draft.role.id,
          canView: draft.canView,
          canEnter: draft.canEnter,
          canLeave: draft.canLeave,
        })),
    };
    this.saving.set(true);
    this.error.set(null);
    try {
      if (this.kind() === 'lead') {
        await this.api.replaceLeadStageAccess(workspaceId, this.stageId(), body);
      } else {
        await this.api.replacePipelineStageAccess(workspaceId, this.stageId(), body);
      }
      this.toast.show({ messageKey: 'stageAccess.saved', messageParams: {} });
      this.closed.emit();
    } catch (error) {
      this.error.set(error);
    } finally {
      this.saving.set(false);
    }
  }

  baseRoleKey(
    role: WorkspaceRoleDefinition['baseRole'],
  ): `members.role.${WorkspaceRoleDefinition['baseRole']}` {
    return `members.role.${role}`;
  }
}

function stageAccessDraft(
  role: WorkspaceRoleDefinition,
  rule: StageAccessRule | undefined,
): StageAccessDraft {
  return {
    role,
    explicit: rule !== undefined,
    canView: rule?.canView ?? false,
    canEnter: rule?.canEnter ?? false,
    canLeave: rule?.canLeave ?? false,
  };
}
