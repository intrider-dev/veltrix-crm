import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { WebhookDelivery, WebhookSubscription } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { safeDeliverySummary, safeWebhookUrl } from './webhooks.page';
import { WebhooksStore } from './webhooks.store';

const subscription: WebhookSubscription = {
  id: '018f0000-0000-7000-8000-000000000001',
  url: 'https://hooks.example.test/crm?token=secret-query-value',
  eventTypes: ['contact.created'],
  enabled: true,
  version: 1,
  secretVersion: 1,
  timeoutSeconds: 10,
  maxAttempts: 8,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const delivery: WebhookDelivery = {
  id: '018f0000-0000-7000-8000-000000000002',
  subscriptionId: subscription.id,
  eventId: '018f0000-0000-7000-8000-000000000003',
  status: 'retrying',
  attempts: 3,
  nextAttemptAt: '2026-01-01T00:05:00Z',
  responseStatus: 503,
  responseSummary: 'temporarily unavailable',
  signatureVersion: 1,
  lastErrorCode: 'webhook.http.status',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:01:00Z',
};

describe('WebhooksStore', () => {
  it('loads and appends bounded cursor pages without duplicates', async () => {
    const older = {
      ...delivery,
      id: '018f0000-0000-7000-8000-000000000004',
      status: 'dead' as const,
    };
    const api = {
      listWebhooks: vi.fn().mockResolvedValue([subscription]),
      listWebhookDeliveries: vi
        .fn()
        .mockResolvedValueOnce({ items: [delivery], nextCursor: 'older' })
        .mockResolvedValueOnce({ items: [delivery, older] }),
    };
    const store = configureStore(api);

    await store.load();
    await store.loadMoreDeliveries();

    expect(api.listWebhookDeliveries).toHaveBeenNthCalledWith(1, 'workspace-1', {
      subscriptionId: undefined,
      cursor: undefined,
      limit: 25,
    });
    expect(api.listWebhookDeliveries).toHaveBeenNthCalledWith(2, 'workspace-1', {
      subscriptionId: undefined,
      cursor: 'older',
      limit: 25,
    });
    expect(store.deliveries().map((item) => item.id)).toEqual([delivery.id, older.id]);
    expect(store.nextCursor()).toBeNull();
  });

  it('filters by subscription and only retries retryable deliveries', async () => {
    const pending = { ...delivery, status: 'pending' as const, attempts: 0 };
    const api = {
      listWebhooks: vi.fn().mockResolvedValue([subscription]),
      listWebhookDeliveries: vi
        .fn()
        .mockResolvedValueOnce({ items: [delivery] })
        .mockResolvedValueOnce({ items: [delivery] })
        .mockResolvedValueOnce({ items: [pending] }),
      retryWebhookDelivery: vi.fn().mockResolvedValue(undefined),
    };
    const store = configureStore(api);
    await store.load();

    await store.filterDeliveries(subscription.id);
    const retried = await store.retryDelivery(delivery);
    const ignored = await store.retryDelivery(pending);

    expect(retried).toBe(true);
    expect(ignored).toBe(false);
    expect(api.retryWebhookDelivery).toHaveBeenCalledOnce();
    expect(api.listWebhookDeliveries).toHaveBeenNthCalledWith(2, 'workspace-1', {
      subscriptionId: subscription.id,
      cursor: undefined,
      limit: 25,
    });
    expect(store.deliveries()).toEqual([pending]);
  });

  it('keeps generated secrets only in the feature store and replaces them on rotation', async () => {
    const rotated = { ...subscription, version: 2, secretVersion: 2 };
    const api = {
      createWebhook: vi
        .fn()
        .mockResolvedValue({ webhook: subscription, signingSecret: 'whsec_created' }),
      rotateWebhookSecret: vi
        .fn()
        .mockResolvedValue({ webhook: rotated, signingSecret: 'whsec_rotated' }),
    };
    const store = configureStore(api);

    await store.create(subscription.url, subscription.eventTypes);
    expect(store.revealedSecret()).toEqual({
      value: 'whsec_created',
      subscriptionUrl: subscription.url,
      reason: 'created',
    });

    await store.rotate(subscription);
    expect(store.revealedSecret()?.value).toBe('whsec_rotated');
    expect(store.subscriptions()[0]?.secretVersion).toBe(2);

    store.dismissSecret();
    expect(store.revealedSecret()).toBeNull();
  });
});

describe('webhook log presentation safety', () => {
  it('redacts endpoint query values and response credentials', () => {
    expect(safeWebhookUrl('https://user:password@hooks.example.test/path?token=top-secret')).toBe(
      'https://hooks.example.test/path?token=%5Bredacted%5D',
    );
    const summary = safeDeliverySummary(
      'Authorization: Bearer bearer-value password=correct-horse whsec_abcdefghijklmnopqrstuvwxyz ada@example.test +1 (555) 123-4567',
    );
    expect(summary).not.toContain('bearer-value');
    expect(summary).not.toContain('correct-horse');
    expect(summary).not.toContain('whsec_abcdefghijklmnopqrstuvwxyz');
    expect(summary).not.toContain('ada@example.test');
    expect(summary).not.toContain('+1 (555) 123-4567');
  });

  it('bounds summaries rendered by the table', () => {
    expect(safeDeliverySummary('x'.repeat(500))).toHaveLength(200);
  });
});

function configureStore(api: object): WebhooksStore {
  TestBed.configureTestingModule({
    providers: [
      WebhooksStore,
      { provide: ApiClient, useValue: api },
      { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
    ],
  });
  return TestBed.inject(WebhooksStore);
}
