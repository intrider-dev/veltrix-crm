import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { Company, DuplicateCandidate, UpdateCompany } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { CompanyDetailsStore } from './company-details.store';

const company: Company = {
  id: '018f0000-0000-7000-8000-000000000001',
  name: 'Analytical Engines',
  domain: 'engines.example',
  industry: 'Research',
  status: 'active',
  address: {},
  customFields: {},
  version: 3,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const candidate: DuplicateCandidate = {
  id: '018f0000-0000-7000-8000-000000000002',
  displayName: 'Analytical Engine',
  domain: 'engines.example',
  reason: 'domain_exact',
  score: 1,
};

describe('CompanyDetailsStore', () => {
  it('loads the company by resource id and writes with its exact version', async () => {
    const updated = { ...company, name: 'Analytical Engines Ltd', version: 4 };
    const api = {
      getCompany: vi.fn().mockResolvedValue({ body: company, etag: '"3"' }),
      listActivities: vi.fn().mockResolvedValue([]),
      updateCompany: vi.fn().mockResolvedValue(updated),
    };
    configure(api);
    const store = TestBed.inject(CompanyDetailsStore);
    const body: UpdateCompany = {
      name: updated.name,
      domain: company.domain,
      industry: company.industry,
      status: 'active',
      address: {},
      customFields: {},
    };

    await store.load(company.id);
    await store.save(company.id, body);

    expect(api.getCompany).toHaveBeenCalledWith('workspace-1', company.id);
    expect(api.updateCompany).toHaveBeenCalledWith(
      'workspace-1',
      company.id,
      company.version,
      body,
    );
    expect(store.company()?.version).toBe(4);
  });

  it('fetches the duplicate version before merging into the loaded company', async () => {
    const source = { ...company, id: candidate.id, version: 7 };
    const api = {
      getCompany: vi
        .fn()
        .mockResolvedValueOnce({ body: company, etag: '"3"' })
        .mockResolvedValueOnce({ body: source, etag: '"7"' })
        .mockResolvedValueOnce({ body: { ...company, version: 4 }, etag: '"4"' }),
      listActivities: vi.fn().mockResolvedValue([]),
      companyDuplicates: vi.fn().mockResolvedValue([candidate]),
      mergeCompanies: vi.fn().mockResolvedValue({
        body: {
          targetId: company.id,
          targetVersion: 4,
          sourceId: candidate.id,
          sourceVersion: 8,
        },
        etag: '"4"',
      }),
    };
    configure(api);
    const store = TestBed.inject(CompanyDetailsStore);

    await store.load(company.id);
    await store.loadDuplicates(company.id);
    await store.mergeDuplicate(company.id, candidate.id);

    expect(api.mergeCompanies).toHaveBeenCalledWith('workspace-1', company.id, candidate.id, 7, 3);
    expect(store.company()?.version).toBe(4);
    expect(store.duplicates()).toEqual([]);
  });

  it('soft-deletes using the loaded version', async () => {
    const api = {
      getCompany: vi.fn().mockResolvedValue({ body: company, etag: '"3"' }),
      listActivities: vi.fn().mockResolvedValue([]),
      deleteCompany: vi.fn().mockResolvedValue(undefined),
    };
    configure(api);
    const store = TestBed.inject(CompanyDetailsStore);

    await store.load(company.id);
    await store.deleteCompany(company.id);

    expect(api.deleteCompany).toHaveBeenCalledWith('workspace-1', company.id, company.version);
  });
});

function configure(api: object): void {
  TestBed.configureTestingModule({
    providers: [
      CompanyDetailsStore,
      { provide: ApiClient, useValue: api },
      { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
    ],
  });
}
