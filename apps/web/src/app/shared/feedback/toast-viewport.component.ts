import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { IconComponent } from '../icon/icon.component';
import { ToastService, type AppToast } from './toast.service';

@Component({
  selector: 'app-toast-viewport',
  imports: [IconComponent, MatButtonModule, RouterLink],
  template: `
    <section
      class="toast-viewport"
      aria-live="polite"
      [attr.aria-label]="i18n.t('common.nav.notifications')"
    >
      @for (toast of toasts.items(); track toast.id) {
        <article class="toast" role="status">
          <p>{{ message(toast) }}</p>
          <div class="actions">
            @if (toast.action && toast.actionLabelKey) {
              <button mat-button type="button" (click)="toasts.invokeAction(toast)">
                {{ label(toast.actionLabelKey) }}
              </button>
            }
            @if (toast.href) {
              <a mat-button [routerLink]="toast.href" (click)="toasts.dismiss(toast.id)">
                {{ i18n.t('notifications.open') }}
              </a>
            }
            <button
              mat-icon-button
              type="button"
              (click)="toasts.dismiss(toast.id)"
              [attr.aria-label]="i18n.t('common.action.close')"
            >
              <app-icon name="close" />
            </button>
          </div>
        </article>
      }
    </section>
  `,
  styles: `
    .toast-viewport {
      position: fixed;
      z-index: 120;
      top: 4.25rem;
      right: 1rem;
      display: grid;
      width: min(24rem, calc(100vw - 2rem));
      gap: 0.6rem;
      pointer-events: none;
    }
    .toast {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.75rem;
      padding: 0.75rem 0.65rem 0.75rem 0.9rem;
      border: 1px solid var(--border);
      border-radius: 0.75rem;
      background: color-mix(in srgb, var(--surface-raised) 96%, transparent);
      box-shadow: var(--shadow-lg);
      backdrop-filter: blur(14px);
      pointer-events: auto;
      animation: toast-in 180ms var(--ease-out);
    }
    p {
      margin: 0;
      line-height: 1.4;
    }
    .actions {
      display: inline-flex;
      align-items: center;
    }
    @keyframes toast-in {
      from {
        transform: translateY(-0.5rem) scale(0.98);
        opacity: 0;
      }
    }
    @media (max-width: 600px) {
      .toast-viewport {
        top: auto;
        right: 0.75rem;
        bottom: 0.75rem;
        width: calc(100vw - 1.5rem);
      }
    }
    @media (prefers-reduced-motion: reduce) {
      .toast {
        animation: none;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ToastViewportComponent {
  readonly toasts = inject(ToastService);
  readonly i18n = inject(I18nService);

  message(toast: AppToast): string {
    const params: Record<string, string | number | boolean> = {};
    for (const [key, value] of Object.entries(toast.messageParams)) {
      if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
        params[key] = value;
      }
    }
    return this.i18n.t(toast.messageKey as AppMessageKey, params);
  }

  label(key: AppMessageKey): string {
    return this.i18n.t(key);
  }
}
