import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import { SecurityStore } from './security.store';

describe('SecurityStore', () => {
  it('exposes one-time recovery codes after MFA confirmation', async () => {
    const api = {
      confirmMFASetup: vi.fn().mockResolvedValue({
        recoveryCodes: ['alpha-1', 'bravo-2'],
        sessionsRevoked: true,
      }),
    };
    TestBed.configureTestingModule({
      providers: [SecurityStore, { provide: ApiClient, useValue: api }],
    });
    const store = TestBed.inject(SecurityStore);

    await store.confirmMFASetup('123456');

    expect(api.confirmMFASetup).toHaveBeenCalledWith({ code: '123456' });
    expect(store.status()).toEqual({ enabled: true });
    expect(store.recoveryCodes()).toEqual(['alpha-1', 'bravo-2']);
    expect(store.sessionsRevoked()).toBe(true);
  });
});
