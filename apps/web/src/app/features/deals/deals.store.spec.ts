import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { Deal } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { DealsStore } from './deals.store';

const deal: Deal = {
  id: 'deal-1',
  name: 'Renewal',
  pipelineId: 'pipeline-1',
  stageId: 'stage-1',
  amountMinor: 250000,
  currency: 'USD',
  position: 0,
  status: 'open',
  version: 3,
  updatedAt: '2026-01-01T00:00:00Z',
};

describe('DealsStore', () => {
  beforeEach(() => localStorage.clear());

  it('updates optimistically and rolls back when a move fails', async () => {
    let rejectMove: (reason: Error) => void = () => undefined;
    const movePromise = new Promise<Deal>((_resolve, reject) => {
      rejectMove = reject;
    });
    const api = { moveDeal: vi.fn().mockReturnValue(movePromise) };
    TestBed.configureTestingModule({
      providers: [
        DealsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(DealsStore);
    store.deals.set([deal]);

    const pending = store.move(deal.id, 'stage-2', 1);
    expect(store.deals()[0]?.stageId).toBe('stage-2');
    rejectMove(new Error('network'));
    await pending;

    expect(store.deals()[0]).toEqual(deal);
    expect(store.error()).toBeInstanceOf(Error);
  });

  it('loads a bounded pipeline page and persists list view preference', async () => {
    const api = {
      listDeals: vi.fn().mockResolvedValue({ items: [deal], nextCursor: 'next-page' }),
    };
    TestBed.configureTestingModule({
      providers: [
        DealsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(DealsStore);
    store.pipelines.set([
      {
        id: 'pipeline-1',
        name: 'Sales',
        displayName: 'Sales',
        isDefault: true,
        version: 1,
        stages: [],
        createdAt: '2026-07-22T00:00:00Z',
        updatedAt: '2026-07-22T00:00:00Z',
      },
    ]);
    store.activePipelineId.set('pipeline-1');

    await store.setViewMode('list');

    expect(api.listDeals).toHaveBeenCalledWith(
      'workspace-1',
      'pipeline-1',
      undefined,
      undefined,
      50,
    );
    expect(store.listDeals()).toEqual([deal]);
    expect(store.listNextCursor()).toBe('next-page');
    expect(localStorage.getItem('veltrix.deals.view')).toBe('list');
  });
});
