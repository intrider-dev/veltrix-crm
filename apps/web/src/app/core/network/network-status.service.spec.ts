import { TestBed } from '@angular/core/testing';

import { NetworkStatusService } from './network-status.service';

describe('NetworkStatusService', () => {
  it('reacts to browser offline and online events', () => {
    const service = TestBed.inject(NetworkStatusService);
    window.dispatchEvent(new Event('offline'));
    expect(service.online()).toBe(false);
    window.dispatchEvent(new Event('online'));
    expect(service.online()).toBe(true);
  });
});
