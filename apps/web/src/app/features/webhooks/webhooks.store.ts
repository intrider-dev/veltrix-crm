import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { WebhookDelivery, WebhookSubscription } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

export interface RevealedWebhookSecret {
  readonly value: string;
  readonly subscriptionUrl: string;
  readonly reason: 'created' | 'rotated';
}

const deliveryPageSize = 25;

@Injectable()
export class WebhooksStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private deliveryRequestSequence = 0;

  readonly subscriptions = signal<readonly WebhookSubscription[]>([]);
  readonly deliveries = signal<readonly WebhookDelivery[]>([]);
  readonly selectedSubscriptionId = signal<string | null>(null);
  readonly nextCursor = signal<string | null>(null);
  readonly revealedSecret = signal<RevealedWebhookSecret | null>(null);
  readonly loading = signal(false);
  readonly deliveriesLoading = signal(false);
  readonly loadingMore = signal(false);
  readonly saving = signal(false);
  readonly mutatingSubscriptionIds = signal<ReadonlySet<string>>(new Set());
  readonly retryingDeliveryIds = signal<ReadonlySet<string>>(new Set());
  readonly error = signal<unknown>(null);
  readonly deliveryError = signal<unknown>(null);

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) {
      this.reset();
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.dismissSecret();
    try {
      const subscriptions = await this.api.listWebhooks(workspaceId);
      this.subscriptions.set(subscriptions);
      if (
        this.selectedSubscriptionId() &&
        !subscriptions.some((item) => item.id === this.selectedSubscriptionId())
      ) {
        this.selectedSubscriptionId.set(null);
      }
      await this.loadDeliveries(true);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async filterDeliveries(subscriptionId: string | null): Promise<void> {
    if (subscriptionId === this.selectedSubscriptionId()) return;
    this.selectedSubscriptionId.set(subscriptionId);
    await this.loadDeliveries(true);
  }

  async loadMoreDeliveries(): Promise<void> {
    if (!this.nextCursor() || this.loadingMore()) return;
    await this.loadDeliveries(false);
  }

  async create(url: string, eventTypes: readonly string[]): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.saving.set(true);
    this.error.set(null);
    this.dismissSecret();
    try {
      const result = await this.api.createWebhook(workspaceId, {
        url,
        eventTypes,
        enabled: true,
        timeoutSeconds: 10,
        maxAttempts: 8,
      });
      this.subscriptions.update((items) => [result.webhook, ...items]);
      this.revealedSecret.set({
        value: result.signingSecret,
        subscriptionUrl: result.webhook.url,
        reason: 'created',
      });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async toggle(subscription: WebhookSubscription): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.isMutatingSubscription(subscription.id)) return;
    const previous = subscription;
    this.setSubscriptionMutating(subscription.id, true);
    this.error.set(null);
    this.replaceSubscription({ ...subscription, enabled: !subscription.enabled });
    try {
      this.replaceSubscription(
        await this.api.setWebhookEnabled(workspaceId, subscription, !subscription.enabled),
      );
    } catch (error) {
      this.replaceSubscription(previous);
      this.error.set(error);
    } finally {
      this.setSubscriptionMutating(subscription.id, false);
    }
  }

  async rotate(subscription: WebhookSubscription): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.isMutatingSubscription(subscription.id)) return false;
    this.dismissSecret();
    this.error.set(null);
    this.setSubscriptionMutating(subscription.id, true);
    try {
      const result = await this.api.rotateWebhookSecret(workspaceId, subscription);
      this.replaceSubscription(result.webhook);
      this.revealedSecret.set({
        value: result.signingSecret,
        subscriptionUrl: result.webhook.url,
        reason: 'rotated',
      });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.setSubscriptionMutating(subscription.id, false);
    }
  }

  async retryDelivery(delivery: WebhookDelivery): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || !this.canRetry(delivery) || this.retryingDeliveryIds().has(delivery.id)) {
      return false;
    }
    this.deliveryError.set(null);
    this.setDeliveryRetrying(delivery.id, true);
    try {
      await this.api.retryWebhookDelivery(workspaceId, delivery.id);
      await this.loadDeliveries(true);
      return true;
    } catch (error) {
      this.deliveryError.set(error);
      return false;
    } finally {
      this.setDeliveryRetrying(delivery.id, false);
    }
  }

  canRetry(delivery: WebhookDelivery): boolean {
    return delivery.status === 'retrying' || delivery.status === 'dead';
  }

  isMutatingSubscription(subscriptionId: string): boolean {
    return this.mutatingSubscriptionIds().has(subscriptionId);
  }

  subscriptionUrl(subscriptionId: string): string {
    return this.subscriptions().find((item) => item.id === subscriptionId)?.url ?? subscriptionId;
  }

  dismissSecret(): void {
    this.revealedSecret.set(null);
  }

  private async loadDeliveries(reset: boolean): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const cursor = reset ? undefined : (this.nextCursor() ?? undefined);
    if (!reset && !cursor) return;
    const sequence = ++this.deliveryRequestSequence;
    if (reset) {
      this.deliveriesLoading.set(true);
      this.deliveryError.set(null);
    } else {
      this.loadingMore.set(true);
    }
    try {
      const page = await this.api.listWebhookDeliveries(workspaceId, {
        subscriptionId: this.selectedSubscriptionId() ?? undefined,
        cursor,
        limit: deliveryPageSize,
      });
      if (sequence !== this.deliveryRequestSequence) return;
      this.nextCursor.set(page.nextCursor ?? null);
      if (reset) {
        this.deliveries.set(page.items);
      } else {
        this.deliveries.update((current) => {
          const known = new Set(current.map((item) => item.id));
          return [...current, ...page.items.filter((item) => !known.has(item.id))];
        });
      }
    } catch (error) {
      if (sequence === this.deliveryRequestSequence) this.deliveryError.set(error);
    } finally {
      if (sequence === this.deliveryRequestSequence) {
        this.deliveriesLoading.set(false);
        this.loadingMore.set(false);
      }
    }
  }

  private replaceSubscription(value: WebhookSubscription): void {
    this.subscriptions.update((items) =>
      items.map((item) => (item.id === value.id ? value : item)),
    );
  }

  private setSubscriptionMutating(subscriptionId: string, active: boolean): void {
    this.mutatingSubscriptionIds.update((current) => {
      const next = new Set(current);
      if (active) next.add(subscriptionId);
      else next.delete(subscriptionId);
      return next;
    });
  }

  private setDeliveryRetrying(deliveryId: string, active: boolean): void {
    this.retryingDeliveryIds.update((current) => {
      const next = new Set(current);
      if (active) next.add(deliveryId);
      else next.delete(deliveryId);
      return next;
    });
  }

  private reset(): void {
    this.deliveryRequestSequence++;
    this.subscriptions.set([]);
    this.deliveries.set([]);
    this.selectedSubscriptionId.set(null);
    this.nextCursor.set(null);
    this.dismissSecret();
  }
}
