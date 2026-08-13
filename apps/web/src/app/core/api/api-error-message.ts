import type { I18nService } from '../i18n/i18n.service';
import { ApiError } from './api-error';

export function apiErrorMessage(
  i18n: I18nService,
  error: unknown,
  fallbackCode = 'network',
): string {
  if (error instanceof ApiError) {
    return i18n.problem(error.problem?.code ?? fallbackCode, error.problem?.requestId);
  }
  return i18n.problem(error instanceof Error ? error.message : fallbackCode);
}
