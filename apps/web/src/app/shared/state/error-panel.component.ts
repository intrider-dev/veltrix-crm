import { ChangeDetectionStrategy, Component, inject, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { ApiError } from '../../core/api/api-error';
import { I18nService } from '../../core/i18n/i18n.service';

@Component({
  selector: 'app-error-panel',
  imports: [MatButtonModule],
  template: `
    <section class="error-panel" role="alert" aria-live="assertive">
      <strong>{{ message() }}</strong>
      @if (retryable()) {
        <button mat-button type="button" (click)="retry.emit()">
          {{ i18n.t('common.action.retry') }}
        </button>
      }
    </section>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ErrorPanelComponent {
  readonly i18n = inject(I18nService);
  readonly error = input<unknown>(null);
  readonly retryable = input(true);
  readonly retry = output<void>();

  readonly message = () => {
    const error = this.error();
    if (error instanceof ApiError)
      return this.i18n.problem(error.problem?.code ?? 'network', error.problem?.requestId);
    return this.i18n.t(navigator.onLine ? 'web.status.error' : 'web.status.offline');
  };
}
