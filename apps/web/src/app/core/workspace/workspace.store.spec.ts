import { TestBed } from '@angular/core/testing';

import { AuthStore } from '../auth/auth.store';
import { I18nService } from '../i18n/i18n.service';
import { WorkspaceStore } from './workspace.store';

describe('WorkspaceStore', () => {
  it('exposes restoration readiness to tenant-scoped route guards', async () => {
    const { store } = configureStore();
    let releaseRestore!: () => void;
    const restore = new Promise<void>((resolve) => {
      releaseRestore = resolve;
    });
    (
      store as unknown as {
        restorePromise: Promise<void>;
      }
    ).restorePromise = restore;

    let ready = false;
    const wait = store.whenReady().then(() => (ready = true));
    await Promise.resolve();
    expect(ready).toBe(false);

    releaseRestore();
    await wait;
    expect(ready).toBe(true);
  });

  it('serializes an explicit selection after the initial preference restore', async () => {
    const { store } = configureStore();
    let releaseRestore!: () => void;
    const restore = new Promise<void>((resolve) => {
      releaseRestore = resolve;
    });
    (
      store as unknown as {
        restorePromise: Promise<void>;
      }
    ).restorePromise = restore;

    const selection = store.select('01900000-0000-7000-8000-000000000002');
    await Promise.resolve();
    expect(store.active()?.name).toBe('First workspace');

    releaseRestore();
    await selection;
    expect(store.active()?.name).toBe('Second workspace');
  });

  it('does not finish a workspace switch before its durable preference is stored', async () => {
    const { store } = configureStore();
    let releasePersist!: () => void;
    let markPersistStarted!: () => void;
    const persistStarted = new Promise<void>((resolve) => {
      markPersistStarted = resolve;
    });
    const persist = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          releasePersist = resolve;
          markPersistStarted();
        }),
    );
    (
      store as unknown as {
        persist: (workspaceId: string) => Promise<void>;
      }
    ).persist = persist;

    let settled = false;
    const selection = store
      .select('01900000-0000-7000-8000-000000000002')
      .then(() => (settled = true));
    await persistStarted;

    expect(store.active()?.name).toBe('Second workspace');
    expect(settled).toBe(false);
    expect(persist).toHaveBeenCalledWith('01900000-0000-7000-8000-000000000002');

    releasePersist();
    await selection;
    expect(settled).toBe(true);
  });
});

function configureStore(): { store: WorkspaceStore } {
  const session = {
    workspaces: [
      {
        id: '01900000-0000-7000-8000-000000000001',
        name: 'First workspace',
        role: 'owner',
        roleId: '01900000-0000-7000-8000-000000000011',
        roleName: 'Owner',
        permissions: ['records.read', 'roles.write'] as const,
        defaultLocale: 'en',
        timezone: 'UTC',
      },
      {
        id: '01900000-0000-7000-8000-000000000002',
        name: 'Second workspace',
        role: 'owner',
        roleId: '01900000-0000-7000-8000-000000000012',
        roleName: 'Owner',
        permissions: ['records.read', 'roles.write'] as const,
        defaultLocale: 'ru',
        timezone: 'UTC',
      },
    ],
  };
  const auth = {
    session: () => session,
    user: () => ({ preferredLocale: 'en' }),
  };
  const i18n = {
    applyPreference: vi.fn().mockResolvedValue(undefined),
    setTimeZone: vi.fn(),
  };
  TestBed.configureTestingModule({
    providers: [
      WorkspaceStore,
      { provide: AuthStore, useValue: auth },
      { provide: I18nService, useValue: i18n },
    ],
  });
  return { store: TestBed.inject(WorkspaceStore) };
}
