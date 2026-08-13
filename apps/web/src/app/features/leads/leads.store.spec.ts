import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { Lead, LeadStage } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';
import { LeadsStore } from './leads.store';

const timestamp = '2026-07-23T00:00:00Z';
const stages: readonly LeadStage[] = [
  {
    id: 'stage-new',
    name: 'New',
    displayName: 'New',
    category: 'new',
    color: '#2563eb',
    position: 0,
    systemKey: 'new',
    isDefault: true,
    version: 1,
    createdAt: timestamp,
    updatedAt: timestamp,
  },
  {
    id: 'stage-qualified',
    name: 'Qualified',
    displayName: 'Qualified',
    category: 'qualified',
    color: '#16a34a',
    position: 1,
    systemKey: 'qualified',
    isDefault: true,
    version: 1,
    createdAt: timestamp,
    updatedAt: timestamp,
  },
];
const lead: Lead = {
  id: 'lead-1',
  name: 'Morgan Lee',
  status: 'new',
  stageId: 'stage-new',
  customFields: {},
  version: 3,
  createdAt: timestamp,
  updatedAt: timestamp,
};

describe('LeadsStore', () => {
  beforeEach(() => localStorage.clear());

  it('loads a bounded page per visible Kanban stage', async () => {
    const api = {
      listLeads: vi.fn().mockImplementation((_workspaceId: string, options: { stageId?: string }) =>
        Promise.resolve({
          items: options.stageId === lead.stageId ? [lead] : [],
          nextCursor: options.stageId === lead.stageId ? 'next-new' : undefined,
        }),
      ),
    };
    TestBed.configureTestingModule({
      providers: [
        LeadsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: { show: vi.fn(), showError: vi.fn() } },
      ],
    });
    const store = TestBed.inject(LeadsStore);
    store.stages.set(stages);

    await store.setViewMode('kanban');

    expect(api.listLeads).toHaveBeenCalledTimes(2);
    expect(api.listLeads).toHaveBeenCalledWith(
      'workspace-1',
      expect.objectContaining({ stageId: 'stage-new', limit: 25 }),
    );
    expect(api.listLeads).toHaveBeenCalledWith(
      'workspace-1',
      expect.objectContaining({ stageId: 'stage-qualified', limit: 25 }),
    );
    expect(store.leads()).toEqual([lead]);
    expect(store.nextCursorByStage()['stage-new']).toBe('next-new');
  });

  it('rolls a failed stage move back without replacing the list load error', async () => {
    const mutationError = new Error('move failed');
    const toasts = { show: vi.fn(), showError: vi.fn() };
    const api = { moveLeadStage: vi.fn().mockRejectedValue(mutationError) };
    TestBed.configureTestingModule({
      providers: [
        LeadsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: toasts },
      ],
    });
    const store = TestBed.inject(LeadsStore);
    store.leads.set([lead]);

    await store.changeStage(lead, stages[1]);

    expect(store.leads()).toEqual([lead]);
    expect(store.loadError()).toBeNull();
    expect(toasts.showError).toHaveBeenCalledWith(mutationError);
  });

  it('keeps a failed create form open through a dedicated form error', async () => {
    const createError = new Error('invalid lead');
    const api = { createLead: vi.fn().mockRejectedValue(createError) };
    TestBed.configureTestingModule({
      providers: [
        LeadsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: { show: vi.fn(), showError: vi.fn() } },
      ],
    });
    const store = TestBed.inject(LeadsStore);

    const created = await store.create({
      name: 'Morgan Lee',
      status: 'new',
      stageId: 'stage-new',
      customFields: {},
    });

    expect(created).toBe(false);
    expect(store.formError()).toBe(createError);
    expect(store.loadError()).toBeNull();
  });

  it('preserves a stage-load failure while the record list still loads', async () => {
    const stageError = new Error('stages unavailable');
    const api = {
      listLeadStages: vi.fn().mockRejectedValue(stageError),
      listLeads: vi.fn().mockResolvedValue({ items: [lead] }),
    };
    TestBed.configureTestingModule({
      providers: [
        LeadsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: { show: vi.fn(), showError: vi.fn() } },
      ],
    });
    const store = TestBed.inject(LeadsStore);

    await store.loadStages();
    await store.load();

    expect(store.stageError()).toBe(stageError);
    expect(store.loadError()).toBeNull();
    expect(store.leads()).toEqual([lead]);
  });
});
