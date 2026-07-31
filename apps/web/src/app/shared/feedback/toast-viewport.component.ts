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
        <article
          class="toast"
          [class.error]="toast.tone === 'error'"
          [attr.role]="toast.tone === 'error' ? 'alert' : 'status'"
        >
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
      left: 50%;
      display: grid;
      width: min(24rem, calc(100vw - 2rem));
      gap: 0.6rem;
      transform: translateX(-50%);
      pointer-events: none;
    }
    .toast {
      position: relative;
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.75rem;
      overflow: hidden;
      padding: 0.8rem 0.65rem 0.8rem 1rem;
      border: 1px solid color-mix(in srgb, var(--border) 76%, transparent);
      border-radius: var(--radius-panel);
      background: color-mix(in srgb, var(--surface-raised) 96%, transparent);
      box-shadow: var(--shadow-lg);
      backdrop-filter: blur(14px);
      pointer-events: auto;
      color: var(--text);
      transition: transform 180ms var(--ease-out);

      @starting-style {
        transform: translateY(-0.5rem) scale(0.98);
      }
    }
    .toast::before {
      position: absolute;
      inset: 0 auto 0 0;
      width: 0.25rem;
      background: var(--signal);
      content: '';
    }
    .toast.error {
      border-color: color-mix(in srgb, var(--danger) 52%, var(--border));
    }
    .toast.error::before {
      background: var(--danger);
    }
    p {
      margin: 0;
      line-height: 1.4;
    }
    .actions {
      display: inline-flex;
      align-items: center;
    }
    @media (max-width: 600px) {
      .toast-viewport {
        top: 4.25rem;
        left: 50%;
        bottom: auto;
        width: calc(100vw - 1.5rem);
      }
    }
    @media (prefers-reduced-motion: reduce) {
      .toast {
        transition: none;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ToastViewportComponent {
  readonly toasts = inject(ToastService);
  readonly i18n = inject(I18nService);

  message(toast: AppToast): string {
    if (toast.problemCode) return this.i18n.problem(toast.problemCode, toast.requestId);
    const params: Record<string, string | number | boolean> = {};
    for (const [key, value] of Object.entries(toast.messageParams)) {
      if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
        params[key] = value;
      }
    }
    return this.i18n.t((toast.messageKey ?? 'web.status.error') as AppMessageKey, params);
  }

  label(key: AppMessageKey): string {
    return this.i18n.t(key);
  }
}
