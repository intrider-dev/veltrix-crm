import { TestBed } from '@angular/core/testing';

import {
  DRAFT_BACKEND,
  DRAFT_CLOCK,
  DRAFT_LIMITS,
  DraftQuotaError,
  DraftService,
  type DraftBackend,
  type DraftClock,
  type DraftKey,
  type DraftRecord,
} from './draft.service';

class MemoryDraftBackend implements DraftBackend {
  readonly records = new Map<string, DraftRecord>();
  list(): Promise<DraftRecord[]> {
    return Promise.resolve([...this.records.values()]);
  }
  put(record: DraftRecord): Promise<void> {
    this.records.set(record.id, record);
    return Promise.resolve();
  }
  delete(id: string): Promise<void> {
    this.records.delete(id);
    return Promise.resolve();
  }
}

describe('DraftService', () => {
  const key: DraftKey = { workspaceId: 'workspace-1', feature: 'contact', recordId: 'new' };
  let backend: MemoryDraftBackend;
  let clock: DraftClock & { value: number };
  let service: DraftService;

  beforeEach(() => {
    backend = new MemoryDraftBackend();
    clock = {
      value: 1_000,
      now() {
        return this.value;
      },
    };
    TestBed.configureTestingModule({
      providers: [
        DraftService,
        { provide: DRAFT_BACKEND, useValue: backend },
        { provide: DRAFT_CLOCK, useValue: clock },
        {
          provide: DRAFT_LIMITS,
          useValue: { maxEntries: 2, maxEntryBytes: 64, maxTotalBytes: 96, ttlMs: 100 },
        },
      ],
    });
    service = TestBed.inject(DraftService);
  });

  it('expires and removes stale versioned drafts', async () => {
    await service.save(key, { name: 'Ada' });
    clock.value += 101;
    expect(await service.load(key)).toBeNull();
    expect(backend.records.size).toBe(0);
  });

  it('rejects an individual draft above its byte quota', async () => {
    await expect(service.save(key, { body: 'x'.repeat(80) })).rejects.toEqual(
      new DraftQuotaError('entry-too-large'),
    );
  });

  it('evicts the oldest draft when the bounded entry quota is reached', async () => {
    const second = { ...key, recordId: 'second' };
    const third = { ...key, recordId: 'third' };
    await service.save(key, { name: 'first' });
    clock.value += 1;
    await service.save(second, { name: 'second' });
    clock.value += 1;
    await service.save(third, { name: 'third' });
    expect(await service.load(key)).toBeNull();
    expect(await service.load(second)).toEqual({ name: 'second' });
    expect(await service.load(third)).toEqual({ name: 'third' });
  });

  it('clears only after a successful submit and retains a failed draft', async () => {
    await service.save(key, { name: 'recoverable' });
    await expect(
      service.submitWithDraft(key, () => Promise.reject(new Error('offline'))),
    ).rejects.toThrow('offline');
    expect(await service.load(key)).toEqual({ name: 'recoverable' });

    const result = await service.submitWithDraft(key, () => Promise.resolve('created'));
    expect(result).toBe('created');
    expect(await service.load(key)).toBeNull();
  });
});
