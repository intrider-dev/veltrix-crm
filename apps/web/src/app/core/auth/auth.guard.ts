import { inject } from '@angular/core';
import type { CanActivateFn } from '@angular/router';
import { Router } from '@angular/router';

import { WorkspaceStore } from '../workspace/workspace.store';
import { AuthStore } from './auth.store';

export const authGuard: CanActivateFn = async (_route, state) => {
  const auth = inject(AuthStore);
  const workspace = inject(WorkspaceStore);
  const router = inject(Router);
  if (!(await auth.ensureSession())) {
    return router.createUrlTree(['/login'], { queryParams: { returnUrl: state.url } });
  }
  await workspace.whenReady();
  return true;
};

export const anonymousOnlyGuard: CanActivateFn = async (route) => {
  const auth = inject(AuthStore);
  const router = inject(Router);
  return (await auth.ensureSession())
    ? router.parseUrl(safeLocalReturnUrl(route.queryParamMap.get('returnUrl')))
    : true;
};

export function safeLocalReturnUrl(value: string | null): string {
  return value?.startsWith('/') && !value.startsWith('//') && !value.startsWith('/login')
    ? value
    : '/dashboard';
}
