import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../api/api-client.service';
import type { SessionView } from '../api/api.types';
import { I18nService } from '../i18n/i18n.service';
import { AuthStore } from './auth.store';

const session: SessionView = {
  user: {
    id: '01900000-0000-7000-8000-000000000001',
    email: 'user@example.test',
    displayName: 'Test user',
    preferredLocale: 'ru',
  },
  workspaces: [
    {
      id: '01900000-0000-7000-8000-000000000002',
      name: 'Test workspace',
      role: 'owner',
      roleId: '01900000-0000-7000-8000-000000000003',
      roleName: 'Owner',
      permissions: ['records.read', 'roles.write'],
      defaultLocale: 'en',
      timezone: 'UTC',
    },
  ],
};

describe('AuthStore session probe', () => {
  it('treats an anonymous 200 probe as normal state', async () => {
    const api = { probeSession: vi.fn().mockResolvedValue({ authenticated: false }) };
    const i18n = { applyPreference: vi.fn(), setLocale: vi.fn() };
    TestBed.configureTestingModule({
      providers: [
        AuthStore,
        { provide: ApiClient, useValue: api },
        { provide: I18nService, useValue: i18n },
      ],
    });

    const store = TestBed.inject(AuthStore);

    await expect(store.ensureSession()).resolves.toBe(false);
    expect(store.authenticated()).toBe(false);
    expect(i18n.applyPreference).not.toHaveBeenCalled();
  });

  it('accepts an authenticated probe and applies the user locale', async () => {
    const api = {
      probeSession: vi.fn().mockResolvedValue({ authenticated: true, session }),
    };
    const i18n = { applyPreference: vi.fn().mockResolvedValue(undefined), setLocale: vi.fn() };
    TestBed.configureTestingModule({
      providers: [
        AuthStore,
        { provide: ApiClient, useValue: api },
        { provide: I18nService, useValue: i18n },
      ],
    });

    const store = TestBed.inject(AuthStore);

    await expect(store.ensureSession()).resolves.toBe(true);
    expect(store.session()).toEqual(session);
    expect(i18n.applyPreference).toHaveBeenCalledWith('ru', 'en');
  });

  it('deduplicates concurrent browser session probes', async () => {
    let resolveProbe!: (value: { authenticated: boolean }) => void;
    const probe = new Promise<{ authenticated: boolean }>((resolve) => {
      resolveProbe = resolve;
    });
    const api = { probeSession: vi.fn().mockReturnValue(probe) };
    const i18n = { applyPreference: vi.fn(), setLocale: vi.fn() };
    TestBed.configureTestingModule({
      providers: [
        AuthStore,
        { provide: ApiClient, useValue: api },
        { provide: I18nService, useValue: i18n },
      ],
    });
    const store = TestBed.inject(AuthStore);

    const first = store.ensureSession();
    const second = store.ensureSession();
    resolveProbe({ authenticated: false });

    await expect(Promise.all([first, second])).resolves.toEqual([false, false]);
    expect(api.probeSession).toHaveBeenCalledOnce();
  });
});
