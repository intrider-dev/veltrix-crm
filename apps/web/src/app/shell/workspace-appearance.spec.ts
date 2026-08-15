import { describe, expect, it } from 'vitest';

import { usesDarkWorkspace } from './workspace-appearance';

describe('usesDarkWorkspace', () => {
  it.each([
    '/dashboard',
    '/contacts/0194214e-cd00-7e00-84ca-68fa65be1e3e',
    '/companies?status=active',
    '/leads',
    '/deals/0194214e-cd00-7e00-84ca-68fa65be1e3e',
    '/activities',
    '/calendar#today',
  ])('keeps the dense workspace presentation for %s', (url) => {
    expect(usesDarkWorkspace(url)).toBe(true);
  });

  it.each(['/settings', '/settings/security', '/projects', '/reports', '/notifications'])(
    'lets the selected appearance control %s',
    (url) => {
      expect(usesDarkWorkspace(url)).toBe(false);
    },
  );

  it('does not match a route that only shares a prefix', () => {
    expect(usesDarkWorkspace('/contacts-import')).toBe(false);
  });
});
