import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { ContactDetails, DuplicateCandidate } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ContactDetailsStore } from './contact-details.store';

const current: ContactDetails = {
  id: '018f0000-0000-7000-8000-000000000001',
  firstName: 'Ada',
  lastName: 'Lovelace',
  displayName: 'Ada Lovelace',
  email: 'ada@example.test',
  status: 'active',
  version: 3,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const candidate: DuplicateCandidate = {
  id: '018f0000-0000-7000-8000-000000000002',
  displayName: 'Ada B. Lovelace',
  email: 'ada@example.test',
  reason: 'email_exact',
  score: 1,
};

describe('ContactDetailsStore', () => {
  it('fetches the source version before merging and reloads the surviving contact', async () => {
    const source = { ...current, id: candidate.id, version: 7 };
    const api = {
      getContact: vi
        .fn()
        .mockResolvedValueOnce({ body: current, etag: '"3"' })
        .mockResolvedValueOnce({ body: source, etag: '"7"' })
        .mockResolvedValueOnce({ body: { ...current, version: 4 }, etag: '"4"' }),
      listActivities: vi.fn().mockResolvedValue([]),
      contactDuplicates: vi.fn().mockResolvedValue([candidate]),
      mergeContacts: vi.fn().mockResolvedValue({
        body: {
          targetId: current.id,
          targetVersion: 4,
          sourceId: candidate.id,
          sourceVersion: 8,
        },
        etag: '"4"',
      }),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactDetailsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactDetailsStore);

    await store.load(current.id);
    await store.loadDuplicates(current.id);
    await store.mergeDuplicate(current.id, candidate.id);

    expect(api.mergeContacts).toHaveBeenCalledWith('workspace-1', current.id, candidate.id, 7, 3);
    expect(store.contact()?.version).toBe(4);
    expect(store.duplicates()).toEqual([]);
  });

  it('soft-deletes using the exact loaded version', async () => {
    const api = {
      getContact: vi.fn().mockResolvedValue({ body: current, etag: '"3"' }),
      listActivities: vi.fn().mockResolvedValue([]),
      deleteContact: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactDetailsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactDetailsStore);

    await store.load(current.id);
    await store.deleteContact(current.id);

    expect(api.deleteContact).toHaveBeenCalledWith('workspace-1', current.id, current.version);
  });
});
