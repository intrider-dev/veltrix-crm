import { effectivePermissionsAllow } from './permissions';

describe('effectivePermissionsAllow', () => {
  it('uses only server-provided effective permissions', () => {
    const permissions = ['records.read', 'reports.read'] as const;
    expect(effectivePermissionsAllow(permissions, 'records.read')).toBe(true);
    expect(effectivePermissionsAllow(permissions, 'records.update')).toBe(false);
    expect(effectivePermissionsAllow(undefined, 'records.read')).toBe(false);
  });
});
