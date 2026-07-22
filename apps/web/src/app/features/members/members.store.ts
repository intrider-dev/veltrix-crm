import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  MembershipMutation,
  Department,
  WorkspaceInvitation,
  WorkspaceMember,
  WorkspaceRole,
  WorkspaceRoleDefinition,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class MembersStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly members = signal<readonly WorkspaceMember[]>([]);
  readonly departments = signal<readonly Department[]>([]);
  readonly roles = signal<readonly WorkspaceRoleDefinition[]>([]);
  readonly invitation = signal<WorkspaceInvitation | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [members, departments, roles] = await Promise.all([
        this.api.listMembers(id),
        this.api.listDepartments(id),
        this.api.listWorkspaceRoles(id),
      ]);
      this.members.set(members);
      this.departments.set(departments);
      this.roles.set(roles);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async invite(email: string, role: WorkspaceRole): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    this.error.set(null);
    this.invitation.set(null);
    try {
      this.invitation.set(await this.api.inviteMember(id, email, role));
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async changeRole(member: WorkspaceMember, role: WorkspaceRole): Promise<void> {
    const id = this.workspace.id();
    if (!id || member.role === role) return;
    try {
      this.merge(await this.api.updateMemberRole(id, member.id, role));
    } catch (error) {
      this.error.set(error);
    }
  }

  async assignRole(member: WorkspaceMember, role: WorkspaceRoleDefinition): Promise<void> {
    const id = this.workspace.id();
    if (!id || member.roleId === role.id) return;
    try {
      const update = await this.api.assignWorkspaceRole(id, member.id, role.id);
      this.merge({ ...update, roleName: role.name });
    } catch (error) {
      this.error.set(error);
    }
  }

  async toggleStatus(member: WorkspaceMember): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    try {
      this.merge(
        await this.api.updateMemberStatus(
          id,
          member.id,
          member.status === 'active' ? 'disabled' : 'active',
        ),
      );
    } catch (error) {
      this.error.set(error);
    }
  }

  async createDepartment(name: string): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    try {
      const created = await this.api.createDepartment(id, name);
      this.departments.update((items) => [...items, created]);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async addDepartmentMember(departmentId: string, membershipId: string): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    try {
      await this.api.addDepartmentMember(id, departmentId, membershipId);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  private merge(update: MembershipMutation & { readonly roleName?: string }): void {
    this.members.update((items) =>
      items.map((item) => (item.id === update.id ? { ...item, ...update } : item)),
    );
  }
}
