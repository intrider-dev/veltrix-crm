import { TestBed } from '@angular/core/testing';

import { I18nService } from './i18n.service';

describe('I18nService', () => {
  beforeEach(() => {
    const catalogs: Record<string, Record<string, string>> = {
      '/i18n/en/common.json': {
        'product.welcome': 'Welcome to {productName}',
        'resultCount.one': '{count} result',
        'resultCount.few': '{count} results',
        'resultCount.many': '{count} results',
        'resultCount.other': '{count} results',
      },
      '/i18n/ru/common.json': {
        'product.welcome': 'Добро пожаловать в {productName}',
        'resultCount.one': '{count} результат',
        'resultCount.few': '{count} результата',
        'resultCount.many': '{count} результатов',
        'resultCount.other': '{count} результата',
      },
    };
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string | URL | Request) => {
        const path =
          typeof input === 'string'
            ? input
            : input instanceof URL
              ? input.pathname
              : new URL(input.url).pathname;
        return Promise.resolve(
          new Response(JSON.stringify(catalogs[path] ?? {}), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }),
    );
  });

  afterEach(() => vi.unstubAllGlobals());

  it('switches a loaded runtime catalog without recreating the service', async () => {
    const service = TestBed.inject(I18nService);
    await service.initialize();
    expect(service.t('common.product.welcome', { productName: 'CRM' })).toBe('Welcome to CRM');

    await service.setLocale('ru');

    expect(service.locale()).toBe('ru');
    expect(document.documentElement.lang).toBe('ru');
    expect(service.t('common.product.welcome', { productName: 'CRM' })).toBe(
      'Добро пожаловать в CRM',
    );
  });

  it('applies user, workspace, deployment precedence in that order', async () => {
    const service = TestBed.inject(I18nService);
    await service.initialize();
    await service.applyPreference(null, 'ru', 'en');
    expect(service.locale()).toBe('ru');
    await service.applyPreference('en', 'ru', 'ru');
    expect(service.locale()).toBe('en');
    await service.applyPreference('RU', 'en', 'en');
    expect(service.locale()).toBe('ru');
  });

  it('uses native Russian plural categories', async () => {
    const service = TestBed.inject(I18nService);
    await service.initialize();
    await service.setLocale('ru');
    expect(service.plural('common.resultCount', 1)).toBe('1 результат');
    expect(service.plural('common.resultCount', 5)).toBe('5 результатов');
  });

  it('formats instants in the validated workspace timezone', async () => {
    const service = TestBed.inject(I18nService);
    await service.initialize();
    service.setTimeZone('UTC');
    const utc = service.date('2026-01-01T00:30:00Z', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
    });
    service.setTimeZone('America/Los_Angeles');
    const pacific = service.date('2026-01-01T00:30:00Z', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
    });
    expect(utc).not.toBe(pacific);
  });

  it('retries a catalog after a transient fetch failure', async () => {
    const service = TestBed.inject(I18nService);
    await service.initialize();
    const eagerFetch = vi.mocked(fetch);
    let attempts = 0;
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string | URL | Request) => {
        const path =
          typeof input === 'string'
            ? input
            : input instanceof URL
              ? input.pathname
              : new URL(input.url).pathname;
        if (path === '/i18n/en/retry.json') {
          attempts += 1;
          if (attempts === 1) return Promise.reject(new TypeError('Failed to fetch'));
          return Promise.resolve(
            new Response(JSON.stringify({ ready: 'Ready' }), {
              status: 200,
              headers: { 'Content-Type': 'application/json' },
            }),
          );
        }
        return eagerFetch(input);
      }),
    );

    await expect(service.loadNamespaces(['retry'])).rejects.toThrow('Failed to fetch');
    await expect(service.loadNamespaces(['retry'])).resolves.toBeUndefined();
    expect(attempts).toBe(2);
  });

  it('does not blank the SPA when one eager catalog stays unavailable', async () => {
    const originalFetch = vi.mocked(fetch);
    let problemAttempts = 0;
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string | URL | Request) => {
        const path =
          typeof input === 'string'
            ? input
            : input instanceof URL
              ? input.pathname
              : new URL(input.url).pathname;
        if (path === '/i18n/en/problems.json') {
          problemAttempts += 1;
          return Promise.reject(new TypeError('Failed to fetch'));
        }
        return originalFetch(input);
      }),
    );

    const service = TestBed.inject(I18nService);
    await expect(service.initialize()).resolves.toBeUndefined();

    expect(problemAttempts).toBe(2);
    expect(service.t('common.product.welcome', { productName: 'CRM' })).toBe('Welcome to CRM');
    expect(service.t('problems.problem.generic', { requestId: 'request' })).toBe(
      'problems.problem.generic',
    );
  });
});
