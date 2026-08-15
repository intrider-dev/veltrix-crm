import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, email, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';

import type {
  WorkspaceMember,
  WorkspaceRole,
  WorkspaceRoleDefinition,
} from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { MembersStore } from './members.store';

@Component({
  selector: 'app-members-page',
  imports: [
    ErrorPanelComponent,
    FormField,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  providers: [MembersStore],
  template: `
    <div class="page settings-feature">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('members.title') }}</h1>
          <p>{{ i18n.t('members.subtitle') }}</p>
        </div>
      </header>
      @if (!permissions.allows('members.read')) {
        <div class="error-panel" role="alert">{{ i18n.t('members.permission') }}</div>
      } @else {
        @if (store.error()) {
          <app-error-panel [error]="store.error()" (retry)="store.load()" />
        }
        @if (permissions.allows('members.write')) {
          <section class="panel split-editor">
            <form (submit)="invite($event)" novalidate>
              <h2>{{ i18n.t('members.invite') }}</h2>
              <mat-form-field appearance="outline"
                ><mat-label>{{ i18n.t('common.field.email') }}</mat-label
                ><input matInput inputmode="email" [formField]="inviteForm.email"
              /></mat-form-field>
              <label class="native-field"
                ><span>{{ i18n.t('members.role') }}</span
                ><mat-select
                  panelClass="crm-select-panel"
                  [aria-label]="i18n.t('members.role')"
                  [formField]="inviteForm.role"
                >
                  @for (role of roles; track role) {
                    <mat-option [value]="role">{{ i18n.t(roleKey(role)) }}</mat-option>
                  }
                </mat-select></label
              >
              <button mat-flat-button type="submit" [disabled]="store.saving()">
                {{ i18n.t('members.sendInvite') }}
              </button>
            </form>
            <form (submit)="createDepartment($event)" novalidate>
              <h2>{{ i18n.t('members.departments') }}</h2>
              <mat-form-field appearance="outline"
                ><mat-label>{{ i18n.t('members.departmentName') }}</mat-label
                ><input matInput [formField]="departmentForm.name"
              /></mat-form-field>
              <button mat-stroked-button type="submit" [disabled]="store.saving()">
                {{ i18n.t('members.createDepartment') }}
              </button>
            </form>
          </section>
          @if (store.invitation(); as invitation) {
            <section class="secret-panel" role="status">
              <strong>{{ i18n.t('members.invitationReady') }}</strong>
              <p>{{ i18n.t('members.invitationOnce') }}</p>
              <code>{{ invitation.token }}</code>
            </section>
          }
        }
        <section class="panel member-list" [attr.aria-busy]="store.loading()">
          @if (store.loading()) {
            <div class="list-skeleton">
              <div class="skeleton"></div>
              <div class="skeleton"></div>
            </div>
          } @else {
            @for (member of store.members(); track member.id) {
              <article>
                <div>
                  <h2>{{ member.displayName }}</h2>
                  <p>
                    {{ member.email }} ·
                    {{ i18n.languageName(member.localeOverride || member.preferredLocale || 'en') }}
                  </p>
                </div>
                <div class="member-actions">
                  @if (permissions.allows('members.write')) {
                    <label class="visually-hidden" [for]="'role-' + member.id">{{
                      i18n.t('members.role')
                    }}</label>
                    @if (permissions.allows('roles.write') && member.role !== 'owner') {
                      <mat-select
                        panelClass="crm-select-panel"
                        [aria-label]="i18n.t('members.role')"
                        [id]="'role-' + member.id"
                        [value]="member.roleId"
                        (selectionChange)="assignRole(member, $event.value)"
                      >
                        @for (role of assignableRoles(); track role.id) {
                          <mat-option [value]="role.id">{{ roleLabel(role) }}</mat-option>
                        }
                      </mat-select>
                    } @else if (member.role !== 'owner') {
                      <mat-select
                        panelClass="crm-select-panel"
                        [aria-label]="i18n.t('members.role')"
                        [id]="'role-' + member.id"
                        [value]="member.role"
                        (selectionChange)="changeRole(member, $event.value)"
                      >
                        @for (role of roles; track role) {
                          <mat-option [value]="role">{{ i18n.t(roleKey(role)) }}</mat-option>
                        }
                      </mat-select>
                    } @else {
                      <span class="status-pill">{{ i18n.t(roleKey(member.role)) }}</span>
                    }
                    <button mat-stroked-button type="button" (click)="store.toggleStatus(member)">
                      {{
                        i18n.t(member.status === 'active' ? 'members.disable' : 'members.enable')
                      }}
                    </button>
                  } @else {
                    <span class="status-pill">{{ i18n.t(roleKey(member.role)) }}</span>
                  }
                </div>
              </article>
            } @empty {
              <div class="empty-state">{{ i18n.t('members.empty') }}</div>
            }
          }
        </section>
        @if (
          permissions.allows('members.write') &&
          store.departments().length &&
          store.members().length
        ) {
          <section class="panel department-membership">
            <h2>{{ i18n.t('members.addToDepartment') }}</h2>
            <label class="native-field"
              ><span>{{ i18n.t('members.department') }}</span
              ><mat-select
                panelClass="crm-select-panel"
                [aria-label]="i18n.t('members.department')"
                [value]="departmentId()"
                (selectionChange)="departmentId.set($event.value)"
              >
                @for (department of store.departments(); track department.id) {
                  <mat-option [value]="department.id">{{ department.name }}</mat-option>
                }
              </mat-select></label
            ><label class="native-field"
              ><span>{{ i18n.t('members.member') }}</span
              ><mat-select
                panelClass="crm-select-panel"
                [aria-label]="i18n.t('members.member')"
                [value]="membershipId()"
                (selectionChange)="membershipId.set($event.value)"
              >
                @for (member of store.members(); track member.id) {
                  <mat-option [value]="member.id">{{ member.displayName }}</mat-option>
                }
              </mat-select></label
            ><button mat-stroked-button type="button" (click)="addToDepartment()">
              {{ i18n.t('common.action.add') }}
            </button>
          </section>
        }
      }
    </div>
  `,
  styles: `
    .settings-feature {
      max-width: 70rem;
    }
    .split-editor {
      display: grid;
      grid-template-columns: 1fr 1fr;
      overflow: hidden;
    }
    .split-editor form {
      display: grid;
      align-content: start;
      gap: 0.75rem;
      padding: 1rem;
    }
    .split-editor form + form {
      border-inline-start: 1px solid var(--border);
    }
    .split-editor h2,
    .department-membership h2 {
      margin: 0;
      font-size: 1rem;
    }
    .secret-panel {
      padding: 1rem;
      border: 1px solid color-mix(in srgb, var(--brand) 30%, transparent);
      border-radius: 0.65rem;
      background: var(--brand-soft);
    }
    .secret-panel p {
      margin: 0.3rem 0 0.65rem;
      color: var(--text-muted);
    }
    .secret-panel code {
      display: block;
      overflow-wrap: anywhere;
      user-select: all;
    }
    .member-list article {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 0.85rem 1rem;
      border-bottom: 1px solid var(--border);
    }
    .member-list article:last-child {
      border: 0;
    }
    article h2 {
      margin: 0;
      font-size: 0.9rem;
    }
    article p {
      margin: 0.2rem 0 0;
      color: var(--text-muted);
      font-size: 0.75rem;
    }
    .member-actions,
    .department-membership {
      align-items: center;
      gap: 0.6rem;
    }
    .member-actions {
      display: flex;
      flex-wrap: wrap;
      justify-content: flex-end;
    }
    .member-actions .mat-mdc-select {
      min-height: 2.35rem;
      padding: 0 2.2rem 0 0.7rem;
      border: 1px solid var(--border);
      border-radius: 0.4rem;
      color: var(--text);
      background: var(--surface-raised);
    }
    .department-membership {
      display: grid;
      grid-template-columns: minmax(12rem, 0.7fr) repeat(2, minmax(13rem, 1fr)) auto;
      padding: 1rem;
    }
    @media (max-width: 700px) {
      .split-editor {
        grid-template-columns: 1fr;
      }
      .split-editor form + form {
        border-top: 1px solid var(--border);
        border-inline-start: 0;
      }
      .member-list article {
        align-items: stretch;
        flex-direction: column;
      }
      .member-actions {
        justify-content: flex-start;
      }
      .department-membership {
        grid-template-columns: 1fr;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MembersPage implements OnInit {
  readonly store = inject(MembersStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly roles: readonly WorkspaceRole[] = ['owner', 'admin', 'manager', 'sales', 'viewer'];
  readonly inviteModel = signal<{ email: string; role: WorkspaceRole }>({
    email: '',
    role: 'sales',
  });
  readonly inviteForm = form(this.inviteModel, (schema) => {
    required(schema.email);
    email(schema.email);
  });
  readonly departmentModel = signal({ name: '' });
  readonly departmentForm = form(this.departmentModel, (schema) => required(schema.name));
  readonly departmentId = signal('');
  readonly membershipId = signal('');
  ngOnInit(): void {
    if (this.permissions.allows('members.read'))
      void this.store.load().then(() => {
        this.departmentId.set(this.store.departments()[0]?.id ?? '');
        this.membershipId.set(this.store.members()[0]?.id ?? '');
      });
  }
  roleKey(role: WorkspaceRole): `members.role.${WorkspaceRole}` {
    return `members.role.${role}`;
  }
  changeRole(member: WorkspaceMember, role: WorkspaceRole): void {
    void this.store.changeRole(member, role);
  }
  assignableRoles(): readonly WorkspaceRoleDefinition[] {
    return this.store.roles().filter((role) => role.baseRole !== 'owner');
  }
  roleLabel(role: WorkspaceRoleDefinition): string {
    return role.system ? this.i18n.t(this.roleKey(role.baseRole)) : role.name;
  }
  assignRole(member: WorkspaceMember, roleId: string): void {
    const role = this.store.roles().find((item) => item.id === roleId);
    if (role) void this.store.assignRole(member, role);
  }
  async invite(event: Event): Promise<void> {
    event.preventDefault();
    if (this.inviteForm().invalid()) return;
    const value = this.inviteModel();
    await this.store.invite(value.email.trim(), value.role);
    this.inviteModel.set({ email: '', role: 'sales' });
  }
  async createDepartment(event: Event): Promise<void> {
    event.preventDefault();
    if (this.departmentForm().invalid()) return;
    await this.store.createDepartment(this.departmentModel().name.trim());
    this.departmentModel.set({ name: '' });
    this.departmentId.set(this.store.departments().at(-1)?.id ?? '');
  }
  async addToDepartment(): Promise<void> {
    if (this.departmentId() && this.membershipId())
      await this.store.addDepartmentMember(this.departmentId(), this.membershipId());
  }
}
