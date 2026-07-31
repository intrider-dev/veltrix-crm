import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type {
  AssignableWorkspaceRole,
  Permission,
  WorkspaceRole,
  WorkspaceRoleDefinition,
  WorkspaceRoleInput,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { RolesStore } from './roles.store';

interface RoleEditorModel {
  readonly name: string;
  readonly baseRole: AssignableWorkspaceRole;
  readonly permissions: readonly Permission[];
}

const editablePermissions: readonly Permission[] = [
  'records.read',
  'records.create',
  'records.update',
  'records.delete',
  'leads.read',
  'leads.create',
  'leads.update',
  'leads.delete',
  'deals.read',
  'deals.create',
  'deals.update',
  'deals.delete',
  'lead_stages.manage',
  'deal_stages.manage',
  'data.export',
  'reports.read',
  'audit.read',
  'settings.write',
  'members.read',
  'members.write',
];

@Component({
  selector: 'app-roles-page',
  imports: [ErrorPanelComponent, FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  providers: [RolesStore],
  template: `
    <div class="page settings-feature">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('roles.title') }}</h1>
          <p>{{ i18n.t('roles.subtitle') }}</p>
        </div>
        @if (permissions.allows('roles.write')) {
          <button mat-flat-button type="button" (click)="startCreate()">
            {{ i18n.t('roles.add') }}
          </button>
        }
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      @if (!permissions.allows('roles.write')) {
        <div class="permission-note" role="note">{{ i18n.t('roles.permissionDenied') }}</div>
      }

      @if (editorOpen() && permissions.allows('roles.write')) {
        <section class="panel role-editor">
          <form (submit)="save($event)" novalidate>
            <div class="editor-heading">
              <h2>{{ i18n.t(editing() ? 'roles.edit' : 'roles.create') }}</h2>
              <button mat-button type="button" (click)="closeEditor()">
                {{ i18n.t('roles.cancelEdit') }}
              </button>
            </div>
            <div class="editor-fields">
              <mat-form-field appearance="outline">
                <mat-label>{{ i18n.t('roles.name') }}</mat-label>
                <input matInput [formField]="roleForm.name" />
              </mat-form-field>
              <label class="native-field">
                <span>{{ i18n.t('roles.base') }}</span>
                <select [value]="model().baseRole" (change)="setBaseRole($event)">
                  @for (role of baseRoles; track role) {
                    <option [value]="role">{{ i18n.t(systemRoleKey(role)) }}</option>
                  }
                </select>
                <small>{{ i18n.t('roles.baseHint') }}</small>
              </label>
            </div>
            <fieldset class="permission-grid">
              <legend>{{ i18n.t('roles.permissions') }}</legend>
              @for (permission of editablePermissions; track permission) {
                <label [class.disabled]="!availableForBase(permission)">
                  <input
                    type="checkbox"
                    [checked]="model().permissions.includes(permission)"
                    [disabled]="!availableForBase(permission)"
                    (change)="togglePermission(permission, $event)"
                  />
                  <span>{{ i18n.t(permissionKey(permission)) }}</span>
                </label>
              }
            </fieldset>
            <div class="form-actions">
              <button
                mat-flat-button
                type="submit"
                [disabled]="store.saving() || roleForm().invalid()"
              >
                {{ i18n.t(editing() ? 'roles.save' : 'roles.add') }}
              </button>
            </div>
          </form>
        </section>
      }

      <section class="panel role-list" [attr.aria-busy]="store.loading()">
        @if (store.loading()) {
          <div class="list-skeleton">
            <div class="skeleton"></div>
            <div class="skeleton"></div>
          </div>
        } @else {
          @for (role of store.roles(); track role.id) {
            <article class="role-card">
              <div class="role-heading">
                <div>
                  <h2>{{ role.system ? i18n.t(systemRoleKey(role.baseRole)) : role.name }}</h2>
                  <span class="status-pill">{{
                    i18n.t(role.system ? 'roles.system' : 'roles.custom')
                  }}</span>
                </div>
                @if (!role.system && permissions.allows('roles.write')) {
                  <div class="card-actions">
                    <button mat-stroked-button type="button" (click)="startEdit(role)">
                      {{ i18n.t('roles.edit') }}
                    </button>
                    <button mat-button type="button" (click)="store.remove(role)">
                      {{ i18n.t('roles.delete') }}
                    </button>
                  </div>
                }
              </div>
              <p>{{ i18n.t('roles.base') }}: {{ i18n.t(systemRoleKey(role.baseRole)) }}</p>
              <div class="permission-chips">
                @for (permission of role.permissions; track permission) {
                  <span>{{ i18n.t(permissionKey(permission)) }}</span>
                }
              </div>
            </article>
          } @empty {
            <div class="empty-state">{{ i18n.t('roles.empty') }}</div>
          }
        }
      </section>
    </div>
  `,
  styles: `
    .settings-feature {
      max-width: 74rem;
    }
    .permission-note {
      padding: 0.8rem 1rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
      color: var(--text-muted);
      background: var(--surface-subtle);
    }
    .role-editor {
      padding: 1rem;
    }
    .editor-heading,
    .role-heading {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
    }
    .editor-heading h2,
    .role-card h2 {
      margin: 0;
      font-size: 1rem;
    }
    .editor-fields {
      display: grid;
      grid-template-columns: minmax(14rem, 1fr) minmax(14rem, 1fr);
      gap: 0.75rem;
      margin-top: 1rem;
    }
    .native-field small {
      color: var(--text-muted);
    }
    .permission-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.35rem;
      margin: 0.5rem 0 0;
      padding: 0.8rem;
      border: 1px solid var(--border);
      border-radius: var(--control-radius);
    }
    .permission-grid legend {
      padding: 0 0.35rem;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .permission-grid label {
      display: flex;
      align-items: center;
      gap: 0.55rem;
      min-height: 2.25rem;
      color: var(--text);
      font-size: 0.82rem;
    }
    .permission-grid label.disabled {
      color: var(--text-muted);
      opacity: 0.65;
    }
    .role-list {
      overflow: hidden;
    }
    .role-card {
      display: grid;
      grid-template-columns: minmax(15rem, 0.8fr) minmax(18rem, 1.2fr);
      align-items: start;
      gap: 0.85rem 1.5rem;
      padding: 1rem 1.1rem;
      border-bottom: 1px solid var(--border);
    }
    .role-card:last-child {
      border-bottom: 0;
    }
    .role-heading > div:first-child {
      display: flex;
      align-items: center;
      gap: 0.55rem;
    }
    .role-card p {
      grid-column: 1;
      margin: 0;
      color: var(--text-muted);
      font-size: 0.78rem;
    }
    .permission-chips {
      grid-column: 2;
      grid-row: 1 / span 2;
      align-self: center;
    }
    .card-actions,
    .permission-chips {
      display: flex;
      align-items: center;
      gap: 0.4rem;
      flex-wrap: wrap;
    }
    .permission-chips span {
      padding: 0.28rem 0.45rem;
      border-radius: 999px;
      color: var(--text-muted);
      background: var(--surface-subtle);
      font-size: 0.68rem;
    }
    .list-skeleton {
      display: grid;
      gap: 0.75rem;
      padding: 1rem;
    }
    @media (max-width: 760px) {
      .editor-fields,
      .permission-grid,
      .role-card {
        grid-template-columns: 1fr;
      }
      .role-heading {
        align-items: stretch;
        flex-direction: column;
      }
      .permission-chips {
        grid-column: auto;
        grid-row: auto;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RolesPage implements OnInit {
  readonly store = inject(RolesStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly editablePermissions = editablePermissions;
  readonly baseRoles: readonly AssignableWorkspaceRole[] = ['admin', 'manager', 'sales', 'viewer'];
  readonly editorOpen = signal(false);
  readonly editing = signal<WorkspaceRoleDefinition | null>(null);
  readonly model = signal<RoleEditorModel>({
    name: '',
    baseRole: 'sales',
    permissions: ['records.read'],
  });
  readonly roleForm = form(this.model, (schema) => required(schema.name));

  ngOnInit(): void {
    if (this.permissions.allows('members.read')) void this.store.load();
  }

  startCreate(): void {
    this.editing.set(null);
    this.model.set({ name: '', baseRole: 'sales', permissions: ['records.read'] });
    this.editorOpen.set(true);
  }

  startEdit(role: WorkspaceRoleDefinition): void {
    if (role.system) return;
    this.editing.set(role);
    this.model.set({
      name: role.name,
      baseRole: role.baseRole as AssignableWorkspaceRole,
      permissions: role.permissions.filter((permission) => permission !== 'roles.write'),
    });
    this.editorOpen.set(true);
  }

  closeEditor(): void {
    this.editorOpen.set(false);
    this.editing.set(null);
  }

  setBaseRole(event: Event): void {
    const baseRole = (event.target as HTMLSelectElement).value as AssignableWorkspaceRole;
    const available = new Set(this.systemPermissions(baseRole));
    this.model.update((model) => ({
      ...model,
      baseRole,
      permissions: model.permissions.filter((permission) => available.has(permission)),
    }));
  }

  togglePermission(permission: Permission, event: Event): void {
    const checked = (event.target as HTMLInputElement).checked;
    this.model.update((model) => ({
      ...model,
      permissions: checked
        ? [...new Set([...model.permissions, permission])]
        : model.permissions.filter((item) => item !== permission),
    }));
  }

  availableForBase(permission: Permission): boolean {
    return this.systemPermissions(this.model().baseRole).includes(permission);
  }

  private systemPermissions(role: AssignableWorkspaceRole): readonly Permission[] {
    return this.store.roles().find((item) => item.system && item.key === role)?.permissions ?? [];
  }

  async save(event: Event): Promise<void> {
    event.preventDefault();
    if (this.roleForm().invalid()) return;
    const model = this.model();
    const input: WorkspaceRoleInput = {
      name: model.name.trim(),
      baseRole: model.baseRole,
      permissions: [...model.permissions],
    };
    const current = this.editing();
    if (current) await this.store.update(current, input);
    else await this.store.create(input);
    this.closeEditor();
  }

  systemRoleKey(role: WorkspaceRole): `members.role.${WorkspaceRole}` {
    return `members.role.${role}`;
  }

  permissionKey(permission: Permission): AppMessageKey {
    return `roles.permission.${permission}` as AppMessageKey;
  }
}
