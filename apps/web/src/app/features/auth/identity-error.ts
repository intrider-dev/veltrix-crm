import { apiErrorMessage } from '../../core/api/api-error-message';
import type { I18nService } from '../../core/i18n/i18n.service';

export function identityErrorMessage(i18n: I18nService, error: unknown): string {
  return apiErrorMessage(i18n, error);
}
