import type { OnDestroy, OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

import type { WebhookDelivery, WebhookSubscription } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { isSafeWebhookUrl, uniqueTokens } from '../../shared/forms/feature-validation';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { WebhooksStore } from './webhooks.store';

type CopyState = 'idle' | 'copied' | 'failed';

@Component({
  selector: 'app-webhooks-page',
  imports: [ErrorPanelComponent, FormField, MatButtonModule, MatFormFieldModule, MatInputModule],
  providers: [WebhooksStore],
  templateUrl: './webhooks.page.html',
  styleUrl: './webhooks.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WebhooksPage implements OnInit, OnDestroy {
  readonly store = inject(WebhooksStore);
  readonly i18n = inject(I18nService);
  readonly permissions = inject(Permissions);
  readonly validationError = signal(false);
  readonly rotationConfirmation = signal<string | null>(null);
  readonly copyState = signal<CopyState>('idle');
  readonly safeDeliverySummary = safeDeliverySummary;
  readonly safeWebhookUrl = safeWebhookUrl;
  readonly model = signal({ url: '', eventTypes: 'contact.created, deal.stage_changed' });
  readonly webhookForm = form(this.model, (schema) => {
    required(schema.url);
    required(schema.eventTypes);
  });

  ngOnInit(): void {
    if (this.permissions.allows('settings.write')) void this.store.load();
  }

  ngOnDestroy(): void {
    this.store.dismissSecret();
  }

  async create(event: Event): Promise<void> {
    event.preventDefault();
    if (this.webhookForm().invalid()) return;
    const value = this.model();
    const events = uniqueTokens(value.eventTypes);
    if (!isSafeWebhookUrl(value.url) || events.length === 0) {
      this.validationError.set(true);
      return;
    }
    this.validationError.set(false);
    if (await this.store.create(value.url.trim(), events)) {
      this.model.set({ url: '', eventTypes: 'contact.created, deal.stage_changed' });
      this.copyState.set('idle');
    }
  }

  selectSubscription(event: Event): void {
    const value = (event.target as HTMLSelectElement).value;
    void this.store.filterDeliveries(value || null);
  }

  requestRotation(subscriptionId: string): void {
    this.rotationConfirmation.set(subscriptionId);
  }

  cancelRotation(): void {
    this.rotationConfirmation.set(null);
  }

  async confirmRotation(subscription: WebhookSubscription): Promise<void> {
    if (await this.store.rotate(subscription)) {
      this.rotationConfirmation.set(null);
      this.copyState.set('idle');
    }
  }

  async copySecret(secret: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(secret);
      this.copyState.set('copied');
    } catch {
      this.copyState.set('failed');
    }
  }

  closeSecret(): void {
    this.store.dismissSecret();
    this.copyState.set('idle');
  }

  statusKey(
    status: WebhookDelivery['status'],
  ):
    | 'integrations.webhooks.deliveries.status.pending'
    | 'integrations.webhooks.deliveries.status.delivering'
    | 'integrations.webhooks.deliveries.status.delivered'
    | 'integrations.webhooks.deliveries.status.retrying'
    | 'integrations.webhooks.deliveries.status.dead' {
    return `integrations.webhooks.deliveries.status.${status}`;
  }

  deliveryTime(delivery: WebhookDelivery): { readonly label: string; readonly value: string } {
    if (delivery.deliveredAt) {
      return {
        label: this.i18n.t('integrations.webhooks.deliveries.deliveredAt'),
        value: delivery.deliveredAt,
      };
    }
    if (delivery.nextAttemptAt) {
      return {
        label: this.i18n.t('integrations.webhooks.deliveries.nextAttemptAt'),
        value: delivery.nextAttemptAt,
      };
    }
    return {
      label: this.i18n.t('integrations.webhooks.deliveries.updatedAt'),
      value: delivery.updatedAt,
    };
  }
}

export function safeWebhookUrl(value: string): string {
  try {
    const parsed = new URL(value);
    parsed.username = '';
    parsed.password = '';
    parsed.hash = '';
    parsed.pathname = parsed.pathname
      .split('/')
      .map((segment) =>
        /^(?=.*[A-Za-z])(?=.*\d)[A-Za-z0-9_-]{24,}$/.test(segment) ? '[redacted]' : segment,
      )
      .join('/');
    const keys = [...new Set(parsed.searchParams.keys())];
    for (const key of keys) parsed.searchParams.set(key, '[redacted]');
    return parsed.toString();
  } catch {
    const queryIndex = value.indexOf('?');
    return (queryIndex < 0 ? value : `${value.slice(0, queryIndex)}?[redacted]`).slice(0, 200);
  }
}

export function safeDeliverySummary(value: string | undefined): string {
  if (!value) return '';
  const redacted = value
    .replace(/-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*/gi, '[private key redacted]')
    .replace(
      /("(?:authorization|api[_-]?key|access[_-]?token|refresh[_-]?token|session[_-]?token|cookie|password|secret)"\s*:\s*")[^"]*(")/gi,
      '$1[redacted]$2',
    )
    .replace(/\bwhsec_[A-Za-z0-9_-]{8,}\b/gi, 'whsec_[redacted]')
    .replace(/\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}\b/gi, '$1 [redacted]')
    .replace(
      /\b(authorization|api[_-]?key|access[_-]?token|refresh[_-]?token|session[_-]?token|cookie|token|password|secret)(["']?\s*[:=]\s*["']?)[^\s,"';&}]{3,}/gi,
      '$1$2[redacted]',
    )
    .replace(/\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/gi, '[email redacted]')
    .replace(/\+?[0-9][0-9 ()-]{7,}[0-9]/g, '[phone redacted]')
    .replace(/[\r\n\t]+/g, ' ')
    .replace(/\s{2,}/g, ' ')
    .trim();
  return redacted.length > 200 ? `${redacted.slice(0, 199)}…` : redacted;
}
