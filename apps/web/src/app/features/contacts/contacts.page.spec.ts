import { signal } from '@angular/core';
import type { ComponentFixture } from '@angular/core/testing';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';

import { ApiClient } from '../../core/api/api-client.service';
import type { SavedView } from '../../core/api/api.types';
import { Permissions } from '../../core/auth/permissions';
import { DraftService } from '../../core/drafts/draft.service';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ContactsPage } from './contacts.page';

describe('ContactsPage', () => {
  it('selects a newly created saved view after adding its option', async () => {
    const savedView: SavedView = {
      id: '018f0000-0000-7000-8000-000000000010',
      ownerId: '018f0000-0000-7000-8000-000000000020',
      entityType: 'contact',
      name: 'Imported contacts',
      definition: {
        filters: [{ field: 'displayName', operator: 'contains', value: 'Imported' }],
        sort: [{ field: 'updatedAt', direction: 'desc' }],
        columns: ['displayName', 'email', 'companyId', 'status'],
      },
      isShared: false,
      version: 1,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    const api = {
      listContacts: vi.fn().mockResolvedValue({ items: [], nextCursor: null }),
      listSavedViews: vi.fn().mockResolvedValue([]),
      listTags: vi.fn().mockResolvedValue([]),
      listCustomFields: vi.fn().mockResolvedValue([]),
      listMembers: vi.fn().mockResolvedValue([]),
      createSavedView: vi.fn().mockResolvedValue(savedView),
    };
    const fixture = await render(api);
    const page = fixture.componentInstance;

    page.store.query.set('Imported');
    page.savedViewName.set(savedView.name);
    await page.saveCurrentView(new SubmitEvent('submit'));
    fixture.detectChanges();

    const element = fixture.nativeElement as HTMLElement;
    const picker = element.querySelector('.saved-view-picker select') as HTMLSelectElement;
    expect(Array.from(picker.options).map((option) => option.value)).toContain(savedView.id);
    expect(picker.value).toBe(savedView.id);
  });
});

async function render(api: object): Promise<ComponentFixture<ContactsPage>> {
  await TestBed.configureTestingModule({
    imports: [ContactsPage],
    providers: [
      { provide: ApiClient, useValue: api },
      { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      { provide: Permissions, useValue: { allows: () => true } },
      {
        provide: I18nService,
        useValue: {
          t: (key: string) => key,
          plural: (key: string) => key,
          problem: (code: string) => code,
          date: (value: string) => value,
        },
      },
      {
        provide: DraftService,
        useValue: {
          load: vi.fn().mockResolvedValue(null),
          save: vi.fn().mockResolvedValue(undefined),
          remove: vi.fn().mockResolvedValue(undefined),
          submitWithDraft: vi.fn(),
        },
      },
      { provide: Router, useValue: { navigate: vi.fn().mockResolvedValue(true) } },
    ],
  }).compileComponents();
  const fixture = TestBed.createComponent(ContactsPage);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture;
}
