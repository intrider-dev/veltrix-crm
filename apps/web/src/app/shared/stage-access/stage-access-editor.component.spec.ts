import { describe, expect, it } from 'vitest';

import { isStageAccessRoleEditable } from './stage-access-role';

describe('isStageAccessRoleEditable', () => {
  it('keeps a custom admin-based role editable', () => {
    expect(isStageAccessRoleEditable({ system: false, baseRole: 'admin' })).toBe(true);
  });

  it('hides only system owner and admin roles with the backend bypass', () => {
    expect(isStageAccessRoleEditable({ system: true, baseRole: 'owner' })).toBe(false);
    expect(isStageAccessRoleEditable({ system: true, baseRole: 'admin' })).toBe(false);
    expect(isStageAccessRoleEditable({ system: true, baseRole: 'manager' })).toBe(true);
  });
});
