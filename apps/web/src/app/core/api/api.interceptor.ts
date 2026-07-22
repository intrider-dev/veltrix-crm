import type { HttpInterceptorFn } from '@angular/common/http';
import { productConfig } from '@veltrix-crm/product-config';

const unsafeMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

export const apiInterceptor: HttpInterceptorFn = (request, next) => {
  if (!request.url.startsWith('/api/')) return next(request);

  let headers = request.headers.set('Accept', 'application/json');
  if (unsafeMethods.has(request.method)) {
    const csrf = readCookie(`${productConfig.cookiePrefix}_csrf`);
    if (csrf) headers = headers.set('X-CSRF-Token', csrf);
  }

  return next(request.clone({ credentials: 'same-origin', headers }));
};

function readCookie(name: string): string | null {
  const prefix = `${encodeURIComponent(name)}=`;
  for (const part of document.cookie.split(';')) {
    const cookie = part.trim();
    if (cookie.startsWith(prefix)) return decodeURIComponent(cookie.slice(prefix.length));
  }
  return null;
}
