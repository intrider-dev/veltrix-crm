import { inject } from '@angular/core';
import type { ResolveFn } from '@angular/router';

import { I18nService } from './i18n.service';

export function i18nNamespaces(namespaces: readonly string[]): ResolveFn<void> {
  return () => inject(I18nService).loadNamespaces(namespaces);
}
