import { ApiError } from '../../core/api/api-error';
import type { I18nService } from '../../core/i18n/i18n.service';

export function identityErrorMessage(i18n: I18nService, error: unknown): string {
  if (error instanceof ApiError) {
    return i18n.problem(error.problem?.code ?? 'network', error.problem?.requestId);
  }
  return i18n.problem(error instanceof Error ? error.message : 'network');
}
