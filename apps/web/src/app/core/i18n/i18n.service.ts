import { DOCUMENT } from '@angular/common';
import { inject, Injectable, signal } from '@angular/core';
import { productConfig, type SupportedLocale } from '@veltrix-crm/product-config';

import type { AppMessageKey } from './app-message-key';
import type { MessageParams } from './message-key.generated';

type Catalog = Readonly<Record<string, string>>;
type LocaleCatalogs = Readonly<Record<string, Catalog>>;

const eagerNamespaces = ['common', 'auth', 'problems', 'pwa', 'settings', 'web'] as const;

@Injectable({ providedIn: 'root' })
export class I18nService {
  private readonly document = inject(DOCUMENT);
  private readonly activeLocale = signal<SupportedLocale>(productConfig.defaultLocale);
  private readonly activeTimeZone = signal(
    Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
  );
  private readonly activeCatalogs = signal<LocaleCatalogs>({});
  private readonly fallbackCatalogs = signal<LocaleCatalogs>({});
  private readonly namespaces = new Set<string>();
  private readonly requests = new Map<string, Promise<Catalog>>();

  readonly locale = this.activeLocale.asReadonly();
  readonly supportedLocales = productConfig.supportedLocales;

  async initialize(): Promise<void> {
    // One unavailable catalog must not leave the entire SPA as a blank custom
    // element. Load initial namespaces independently, retry transient network
    // failures once, and let the stable message key act as the final fallback.
    await Promise.all(eagerNamespaces.map((namespace) => this.loadInitialNamespace(namespace)));
    this.applyDocumentLocale(this.activeLocale());
  }

  async loadNamespaces(namespaces: readonly string[]): Promise<void> {
    namespaces.forEach((namespace) => this.namespaces.add(namespace));
    const locale = this.activeLocale();
    const [active, fallback] = await Promise.all([
      this.loadCatalogSet(locale, namespaces),
      locale === 'en' ? Promise.resolve({}) : this.loadCatalogSet('en', namespaces),
    ]);
    this.activeCatalogs.update((current) => ({ ...current, ...active }));
    if (locale === 'en') {
      this.fallbackCatalogs.update((current) => ({ ...current, ...active }));
    } else {
      this.fallbackCatalogs.update((current) => ({ ...current, ...fallback }));
    }
  }

  async setLocale(locale: string): Promise<void> {
    const supportedLocale = this.supportedLocale(locale);
    if (!supportedLocale || supportedLocale === this.activeLocale()) return;
    const namespaces = [...this.namespaces];
    const [active, fallback] = await Promise.all([
      this.loadCatalogSet(supportedLocale, namespaces),
      supportedLocale === 'en' ? Promise.resolve({}) : this.loadCatalogSet('en', namespaces),
    ]);
    this.activeCatalogs.set(active);
    this.fallbackCatalogs.set(supportedLocale === 'en' ? active : fallback);
    this.activeLocale.set(supportedLocale);
    this.applyDocumentLocale(supportedLocale);
  }

  async applyPreference(
    userLocale?: string | null,
    workspaceLocale?: string | null,
    deploymentLocale: string = productConfig.defaultLocale,
  ): Promise<void> {
    const locale = [userLocale, workspaceLocale, deploymentLocale]
      .map((candidate) => this.supportedLocale(candidate))
      .find((candidate): candidate is SupportedLocale => candidate !== null);
    await this.setLocale(locale ?? productConfig.defaultLocale);
  }

  supportedLocale(locale: string | null | undefined): SupportedLocale | null {
    if (!locale) return null;
    const normalized = locale.toLowerCase();
    return (
      this.supportedLocales.find((supported) => supported.toLowerCase() === normalized) ?? null
    );
  }

  t(key: AppMessageKey, params: MessageParams = {}): string {
    const [namespace, ...parts] = key.split('.');
    const catalogKey = parts.join('.');
    const template =
      this.activeCatalogs()[namespace]?.[catalogKey] ??
      this.fallbackCatalogs()[namespace]?.[catalogKey] ??
      key;
    return template.replace(/\{([a-zA-Z][\w]*)\}/g, (match, name: string) => {
      const value = params[name];
      return value === undefined ? match : String(value);
    });
  }

  plural(key: 'common.resultCount', count: number): string {
    const category = new Intl.PluralRules(this.activeLocale()).select(count);
    const supportedCategory = category === 'zero' || category === 'two' ? 'other' : category;
    return this.t(`common.resultCount.${supportedCategory}` as AppMessageKey, { count });
  }

  date(
    value: string | Date,
    options: Intl.DateTimeFormatOptions = { dateStyle: 'medium' },
  ): string {
    return new Intl.DateTimeFormat(this.activeLocale(), {
      ...options,
      timeZone: options.timeZone ?? this.activeTimeZone(),
    }).format(new Date(value));
  }

  setTimeZone(timeZone: string | null | undefined): void {
    if (!timeZone) return;
    try {
      new Intl.DateTimeFormat('en', { timeZone }).format(0);
      this.activeTimeZone.set(timeZone);
    } catch {
      // Ignore corrupt server preferences and retain the last validated zone.
    }
  }

  money(minorUnits: number, currency: string): string {
    return new Intl.NumberFormat(this.activeLocale(), { style: 'currency', currency }).format(
      minorUnits / 100,
    );
  }

  languageName(locale: string): string {
    const name =
      new Intl.DisplayNames([this.activeLocale()], { type: 'language' }).of(locale) ?? locale;
    return name.charAt(0).toLocaleUpperCase(this.activeLocale()) + name.slice(1);
  }

  problem(code: string, requestId = ''): string {
    const candidates: AppMessageKey[] = [
      `auth.problem.${code}` as AppMessageKey,
      `contacts.problem.${code}` as AppMessageKey,
      `problems.problem.${code}` as AppMessageKey,
    ];
    for (const candidate of candidates) {
      const translated = this.t(candidate, { requestId });
      if (translated !== candidate) return translated;
    }
    return this.t('problems.problem.generic', { requestId });
  }

  private async loadCatalogSet(
    locale: SupportedLocale,
    namespaces: readonly string[],
  ): Promise<Record<string, Catalog>> {
    const entries = await Promise.all(
      namespaces.map(
        async (namespace) => [namespace, await this.loadCatalog(locale, namespace)] as const,
      ),
    );
    return Object.fromEntries(entries);
  }

  private async loadInitialNamespace(namespace: string): Promise<void> {
    try {
      await this.loadNamespaces([namespace]);
      return;
    } catch {
      await new Promise<void>((resolve) => setTimeout(resolve, 150));
    }
    try {
      await this.loadNamespaces([namespace]);
    } catch {
      // Bootstrap remains usable with stable message keys. The rejected fetch
      // is removed from `requests`, so a later feature load can recover it.
    }
  }

  private loadCatalog(locale: SupportedLocale, namespace: string): Promise<Catalog> {
    const cacheKey = `${locale}:${namespace}`;
    const existing = this.requests.get(cacheKey);
    if (existing) return existing;
    const request: Promise<Catalog> = fetch(`/i18n/${locale}/${namespace}.json`, {
      cache: 'no-cache',
      headers: { Accept: 'application/json' },
    })
      .then(async (response) => {
        if (!response.ok)
          throw new Error(`Translation catalog ${cacheKey} returned ${response.status}`);
        return (await response.json()) as Catalog;
      })
      .catch((error: unknown) => {
        // A transient offline/navigation failure must not poison this locale
        // for the rest of the SPA session; a later route retry can fetch it.
        if (this.requests.get(cacheKey) === request) this.requests.delete(cacheKey);
        throw error;
      });
    this.requests.set(cacheKey, request);
    return request;
  }

  private applyDocumentLocale(locale: SupportedLocale): void {
    this.document.documentElement.lang = locale;
    const language = new Intl.Locale(locale).language;
    this.document.documentElement.dir = ['ar', 'fa', 'he', 'ur'].includes(language) ? 'rtl' : 'ltr';
  }
}
