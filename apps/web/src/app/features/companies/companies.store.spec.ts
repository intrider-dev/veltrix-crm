import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { Company, CompanyPage, SavedView, SavedViewInput } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { CompaniesStore } from './companies.store';

const company: Company = {
  id: '018f0000-0000-7000-8000-000000000001',
  name: 'Analytical Engines',
  domain: 'engines.example',
  industry: 'Research',
  status: 'active',
  version: 3,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

describe('CompaniesStore', () => {
  it('uses server filters and appends a cursor page', async () => {
    const second = { ...company, id: '018f0000-0000-7000-8000-000000000002' };
    const api = {
      listCompanies: vi
        .fn()
        .mockResolvedValueOnce({ items: [company], nextCursor: 'next' } satisfies CompanyPage)
        .mockResolvedValueOnce({ items: [second], nextCursor: null } satisfies CompanyPage),
    };
    configure(api);
    const store = TestBed.inject(CompaniesStore);
    store.query.set('engine');
    store.status.set('active');

    await store.load(true);
    await store.load(false);

    expect(api.listCompanies).toHaveBeenLastCalledWith('workspace-1', {
      cursor: 'next',
      query: 'engine',
      status: 'active',
    });
    expect(store.companies()).toEqual([company, second]);
  });

  it('persists and reapplies a company saved view', async () => {
    const saved: SavedView = {
      id: '018f0000-0000-7000-8000-000000000010',
      ownerId: '018f0000-0000-7000-8000-000000000011',
      entityType: 'company',
      name: 'Active research',
      definition: {
        filters: [
          { field: 'name', operator: 'contains', value: 'research' },
          { field: 'status', operator: 'eq', value: 'active' },
        ],
        sort: [{ field: 'updatedAt', direction: 'desc' }],
        columns: ['name'],
      },
      isShared: false,
      version: 1,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    const api = {
      listCompanies: vi.fn().mockResolvedValue({ items: [company], nextCursor: null }),
      createSavedView: vi
        .fn()
        .mockImplementation((_workspaceId: string, input: SavedViewInput) =>
          Promise.resolve({ ...saved, ...input }),
        ),
    };
    configure(api);
    const store = TestBed.inject(CompaniesStore);

    await store.applySavedView(saved);
    const created = await store.saveCurrentView('Active research');

    expect(api.listCompanies).toHaveBeenCalledWith('workspace-1', {
      cursor: undefined,
      query: 'research',
      status: 'active',
    });
    expect(api.createSavedView).toHaveBeenCalledWith('workspace-1', {
      entityType: 'company',
      name: 'Active research',
      definition: {
        filters: [
          { field: 'name', operator: 'contains', value: 'research' },
          { field: 'status', operator: 'eq', value: 'active' },
        ],
        sort: [{ field: 'updatedAt', direction: 'desc' }],
        columns: ['name', 'domain', 'industry', 'status'],
      },
      isShared: false,
    });
    expect(created.name).toBe('Active research');
  });

  it('restores a deleted company with the server-provided version', async () => {
    const deleted = {
      id: company.id,
      displayName: company.name,
      version: 4,
      deletedAt: '2026-01-02T00:00:00Z',
      createdAt: company.createdAt,
      updatedAt: '2026-01-02T00:00:00Z',
    };
    const api = {
      listCompanyTrash: vi.fn().mockResolvedValue({ items: [deleted], nextCursor: null }),
      restoreCompany: vi.fn().mockResolvedValue({ body: { ...company, version: 5 }, etag: '"5"' }),
    };
    configure(api);
    const store = TestBed.inject(CompaniesStore);

    await store.setMode('trash');
    await store.restore(deleted);

    expect(api.restoreCompany).toHaveBeenCalledWith('workspace-1', company.id, 4);
    expect(store.trash()).toEqual([]);
  });
});

function configure(api: object): void {
  TestBed.configureTestingModule({
    providers: [
      CompaniesStore,
      { provide: ApiClient, useValue: api },
      { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
    ],
  });
}
