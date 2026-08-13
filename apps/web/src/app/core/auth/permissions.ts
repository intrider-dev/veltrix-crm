import { inject, Injectable } from '@angular/core';

import type { Permission as ApiPermission } from '../api/api.types';
import { WorkspaceStore } from '../workspace/workspace.store';

export type Permission = ApiPermission;

export function effectivePermissionsAllow(
  permissions: readonly Permission[] | null | undefined,
  permission: Permission,
): boolean {
  return permissions?.includes(permission) ?? false;
}

@Injectable({ providedIn: 'root' })
export class Permissions {
  private readonly workspace = inject(WorkspaceStore);

  allows(permission: Permission): boolean {
    return effectivePermissionsAllow(this.workspace.active()?.permissions, permission);
  }
}
