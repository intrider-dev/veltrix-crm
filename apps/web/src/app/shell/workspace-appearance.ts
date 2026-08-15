const DARK_WORKSPACE_ROUTES = [
  '/dashboard',
  '/contacts',
  '/companies',
  '/leads',
  '/deals',
  '/activities',
  '/calendar',
] as const;

export function usesDarkWorkspace(url: string): boolean {
  const path = url.split(/[?#]/, 1)[0] || '/';
  return DARK_WORKSPACE_ROUTES.some((route) => path === route || path.startsWith(`${route}/`));
}
