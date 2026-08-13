import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type { ContentTranslation } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { TranslationConflictError, TranslationsStore } from './translations.store';

describe('TranslationsStore', () => {
  const item: ContentTranslation = {
    namespace: 'email',
    key: 'follow-up.subject',
    sourceLocale: 'en',
    sourceText: 'Hello {name}',
    description: 'Follow-up subject',
    placeholders: ['name'],
    resourceVersion: 1,
    locale: 'ru',
    translatedText: '',
    status: 'missing',
    version: 0,
    updatedAt: '2026-07-21T00:00:00Z',
  };
  let api: {
    listTranslations: ReturnType<typeof vi.fn>;
    translationCoverage: ReturnType<typeof vi.fn>;
    localizationSettings: ReturnType<typeof vi.fn>;
    putTranslation: ReturnType<typeof vi.fn>;
  };
  let store: TranslationsStore;

  beforeEach(() => {
    api = {
      listTranslations: vi.fn().mockResolvedValue({ items: [item], nextCursor: null }),
      translationCoverage: vi
        .fn()
        .mockResolvedValue([{ namespace: 'email', total: 4, published: 2, draft: 1, missing: 1 }]),
      localizationSettings: vi.fn().mockResolvedValue({
        body: {
          defaultLocale: 'en',
          supportedLocales: ['en', 'ru'],
          timezone: 'UTC',
          version: 1,
        },
        etag: '"1"',
      }),
      putTranslation: vi.fn(),
    };
    TestBed.configureTestingModule({
      providers: [
        TranslationsStore,
        { provide: ApiClient, useValue: api },
        {
          provide: I18nService,
          useValue: {
            locale: () => 'en',
            supportedLocales: ['en', 'ru'],
            supportedLocale: (locale: string) =>
              locale.toLowerCase() === 'en' || locale.toLowerCase() === 'ru'
                ? locale.toLowerCase()
                : null,
          },
        },
        { provide: WorkspaceStore, useValue: { id: () => 'workspace-1' } },
      ],
    });
    store = TestBed.inject(TranslationsStore);
  });

  it('loads a bounded page and computes published coverage', async () => {
    await store.load();

    expect(api.listTranslations).toHaveBeenCalledWith(
      'workspace-1',
      expect.objectContaining({ locale: 'ru', limit: 50 }),
    );
    expect(store.items()).toEqual([item]);
    expect(store.completion()).toBe(50);
    expect(store.totals()).toEqual({ total: 4, published: 2, draft: 1, missing: 1 });
  });

  it('passes the current version and refreshes the latest record on conflict', async () => {
    const latest = {
      ...item,
      translatedText: 'Привет {name}',
      status: 'draft' as const,
      version: 2,
    };
    api.putTranslation.mockRejectedValue(new ApiError(412, null));
    api.listTranslations.mockResolvedValue({ items: [latest], nextCursor: null });

    await expect(store.save(item, 'Здравствуйте {name}', 'published')).rejects.toEqual(
      new TranslationConflictError(latest),
    );
    expect(api.putTranslation).toHaveBeenCalledWith(
      'workspace-1',
      'ru',
      'email',
      'follow-up.subject',
      0,
      expect.objectContaining({ translatedText: 'Здравствуйте {name}', version: 0 }),
    );
  });

  it('creates the source resource and its first tenant translation atomically', async () => {
    const created = {
      ...item,
      translatedText: 'Здравствуйте {name}',
      status: 'draft' as const,
      version: 1,
    };
    api.putTranslation.mockResolvedValue({ body: created, etag: '"1"' });
    api.translationCoverage.mockResolvedValue([
      { namespace: 'email', total: 5, published: 2, draft: 2, missing: 1 },
    ]);

    await expect(
      store.create({
        sourceLocale: 'en',
        locale: 'ru',
        namespace: 'email',
        key: 'follow-up.subject',
        sourceText: 'Hello {name}',
        description: 'Follow-up subject',
        translatedText: 'Здравствуйте {name}',
        status: 'draft',
      }),
    ).resolves.toEqual(created);

    expect(api.putTranslation).toHaveBeenCalledWith(
      'workspace-1',
      'ru',
      'email',
      'follow-up.subject',
      0,
      expect.objectContaining({ version: 0, status: 'draft' }),
    );
    expect(store.items()[0]).toEqual(created);
  });
});
