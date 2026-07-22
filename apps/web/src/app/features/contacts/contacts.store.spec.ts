import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  Contact,
  ContactImportPreview,
  ContactImportStatus,
  ContactPage,
  CreateContact,
  SavedView,
  SavedViewInput,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ContactsStore } from './contacts.store';

const contact: Contact = {
  id: '018f0000-0000-7000-8000-000000000001',
  firstName: 'Ada',
  lastName: 'Lovelace',
  displayName: 'Ada Lovelace',
  email: 'ada@example.test',
  status: 'active',
  version: 1,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

describe('ContactsStore', () => {
  it('appends a cursor page and prepends a newly created contact', async () => {
    const firstPage: ContactPage = { items: [contact], nextCursor: 'next' };
    const second = {
      ...contact,
      id: '018f0000-0000-7000-8000-000000000002',
      firstName: 'Grace',
      displayName: 'Grace Hopper',
    };
    const api = {
      listContacts: vi
        .fn()
        .mockResolvedValueOnce(firstPage)
        .mockResolvedValueOnce({ items: [second], nextCursor: null }),
      createContact: vi
        .fn()
        .mockResolvedValue({ ...contact, id: '018f0000-0000-7000-8000-000000000003' }),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactsStore);

    await store.load();
    await store.load(false);
    const body: CreateContact = { firstName: 'New', lastName: 'Contact', status: 'active' };
    const created = await store.create(body);

    expect(api.listContacts).toHaveBeenLastCalledWith('workspace-1', {
      cursor: 'next',
      query: undefined,
      status: undefined,
    });
    expect(store.contacts().map((item) => item.id)).toEqual([created.id, contact.id, second.id]);
  });

  it('applies and persists bounded server-side filters as a saved view', async () => {
    const view: SavedView = {
      id: '018f0000-0000-7000-8000-000000000010',
      ownerId: '018f0000-0000-7000-8000-000000000020',
      entityType: 'contact',
      name: 'Active Ada',
      definition: {
        filters: [
          { field: 'displayName', operator: 'contains', value: 'Ada' },
          { field: 'status', operator: 'eq', value: 'active' },
        ],
        sort: [{ field: 'updatedAt', direction: 'desc' }],
        columns: ['displayName', 'email'],
      },
      isShared: false,
      version: 1,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    const api = {
      listContacts: vi.fn().mockResolvedValue({ items: [contact], nextCursor: null }),
      createSavedView: vi
        .fn()
        .mockImplementation((_workspaceId: string, input: SavedViewInput) =>
          Promise.resolve({ ...view, ...input }),
        ),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactsStore);

    await store.applySavedView(view);
    const created = await store.createSavedView('Active Ada');

    expect(api.listContacts).toHaveBeenCalledWith('workspace-1', {
      cursor: undefined,
      query: 'Ada',
      status: 'active',
    });
    expect(api.createSavedView).toHaveBeenCalledWith('workspace-1', {
      entityType: 'contact',
      name: 'Active Ada',
      definition: {
        filters: [
          { field: 'displayName', operator: 'contains', value: 'Ada' },
          { field: 'status', operator: 'eq', value: 'active' },
        ],
        sort: [{ field: 'updatedAt', direction: 'desc' }],
        columns: ['displayName', 'email', 'companyId', 'status'],
      },
      isShared: false,
    });
    expect(created.name).toBe('Active Ada');
  });

  it('loads contact tags and custom-field definitions without requesting forbidden members', async () => {
    const tag = {
      id: '018f0000-0000-7000-8000-000000000040',
      name: 'Priority',
      color: '#506fdd',
      version: 1,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    const customField = {
      id: '018f0000-0000-7000-8000-000000000041',
      entityType: 'contact' as const,
      fieldKey: 'segment',
      label: 'Segment',
      valueType: 'text' as const,
      validation: {},
      options: [],
      schemaVersion: 1,
      version: 1,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    const api = {
      listSavedViews: vi.fn().mockResolvedValue([]),
      listTags: vi.fn().mockResolvedValue([tag]),
      listCustomFields: vi.fn().mockResolvedValue([customField]),
      listMembers: vi.fn().mockResolvedValue([]),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactsStore);

    await store.loadReferences(false);

    expect(api.listSavedViews).toHaveBeenCalledWith('workspace-1', 'contact');
    expect(api.listCustomFields).toHaveBeenCalledWith('workspace-1', 'contact');
    expect(api.listMembers).not.toHaveBeenCalled();
    expect(store.tags()).toEqual([tag]);
    expect(store.customFields()).toEqual([customField]);
  });

  it('sends versioned bulk mutations and refreshes only the current bounded page', async () => {
    const result = {
      operationId: '018f0000-0000-7000-8000-000000000099',
      updated: 1,
    };
    const api = {
      listContacts: vi.fn().mockResolvedValue({ items: [], nextCursor: null }),
      bulkAssignContacts: vi.fn().mockResolvedValue(result),
      bulkTagContacts: vi.fn().mockResolvedValue(result),
      bulkDeleteContacts: vi.fn().mockResolvedValue(result),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactsStore);

    await store.bulkAssign([contact], null);
    await store.bulkTag([contact], ['tag-1'], 'add');
    await store.bulkDelete([contact]);

    const versioned = [{ id: contact.id, version: contact.version }];
    expect(api.bulkAssignContacts).toHaveBeenCalledWith('workspace-1', versioned, null);
    expect(api.bulkTagContacts).toHaveBeenCalledWith('workspace-1', versioned, ['tag-1'], 'add');
    expect(api.bulkDeleteContacts).toHaveBeenCalledWith('workspace-1', versioned);
    expect(api.listContacts).toHaveBeenCalledTimes(3);
    expect(store.operationResult()).toEqual(result);
  });

  it('uses the staged preview mapping and refreshes contacts after a completed import', async () => {
    const preview: ContactImportPreview = {
      id: '018f0000-0000-7000-8000-000000000030',
      entityType: 'contact',
      headers: ['First name', 'Last name'],
      sampleRows: [{ 'First name': 'Ada', 'Last name': 'Lovelace' }],
      totalRows: 1,
      status: 'preview',
      suggestedMapping: { firstName: 'First name', lastName: 'Last name' },
    };
    const completed: ContactImportStatus = {
      id: preview.id,
      entityType: 'contact',
      status: 'completed',
      totalRows: 1,
      processedRows: 1,
      createdRows: 1,
      errorRows: 0,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:01Z',
      completedAt: '2026-01-01T00:00:01Z',
    };
    const api = {
      previewContactImport: vi.fn().mockResolvedValue(preview),
      queueContactImport: vi.fn().mockResolvedValue(completed),
      contactImportErrorsUrl: vi.fn().mockReturnValue('/api/import/errors'),
      listContacts: vi.fn().mockResolvedValue({ items: [contact], nextCursor: null }),
    };
    TestBed.configureTestingModule({
      providers: [
        ContactsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      ],
    });
    const store = TestBed.inject(ContactsStore);
    const file = new File(['First name,Last name\nAda,Lovelace'], 'contacts.csv', {
      type: 'text/csv',
    });

    await store.previewImport(file);
    await store.queueImport({ firstName: 'First name', lastName: 'Last name' });

    expect(api.previewContactImport).toHaveBeenCalledWith('workspace-1', file);
    expect(api.queueContactImport).toHaveBeenCalledWith('workspace-1', preview.id, {
      firstName: 'First name',
      lastName: 'Last name',
    });
    expect(store.importStatus()).toEqual(completed);
    expect(store.contacts()).toEqual([contact]);
  });
});
