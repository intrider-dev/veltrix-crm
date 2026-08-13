import type { WorkspaceRoleDefinition } from '../../core/api/api.types';

export function isStageAccessRoleEditable(
  role: Pick<WorkspaceRoleDefinition, 'system' | 'baseRole'>,
): boolean {
  return !(role.system && (role.baseRole === 'owner' || role.baseRole === 'admin'));
}
