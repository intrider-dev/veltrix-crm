import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { LocalizationSettingsStore } from './localization-settings.store';

describe('LocalizationSettingsStore', () => {
  const initial = {
    defaultLocale: 'en',
    supportedLocales: ['en', 'ru'],
    version: 3,
    updatedAt: '2026-07-22T00:00:00Z',
  } as const;
  let api: {
    localizationSettings: ReturnType<typeof vi.fn>;
    updateLocalizationSettings: ReturnType<typeof vi.fn>;
  };
  let store: LocalizationSettingsStore;

  beforeEach(() => {
    api = {
      localizationSettings: vi.fn().mockResolvedValue({ body: initial, etag: '"3"' }),
      updateLocalizationSettings: vi.fn(),
    };
    TestBed.configureTestingModule({
      providers: [
        LocalizationSettingsStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: () => 'workspace-1' } },
      ],
    });
    store = TestBed.inject(LocalizationSettingsStore);
  });

  it('loads the workspace policy', async () => {
    await expect(store.load()).resolves.toEqual(initial);
    expect(api.localizationSettings).toHaveBeenCalledWith('workspace-1');
  });

  it('uses the current optimistic version when saving', async () => {
    const updated = { ...initial, defaultLocale: 'ru', version: 4 };
    api.updateLocalizationSettings.mockResolvedValue({ body: updated, etag: '"4"' });
    await store.load();

    await expect(store.save('ru', ['en', 'ru'])).resolves.toEqual(updated);

    expect(api.updateLocalizationSettings).toHaveBeenCalledWith('workspace-1', 3, {
      defaultLocale: 'ru',
      supportedLocales: ['en', 'ru'],
    });
    expect(store.saved()).toBe(true);
  });
});
