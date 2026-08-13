import { computed, inject, Injectable, signal } from '@angular/core';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type {
  ContentTranslation,
  PutContentTranslation,
  TranslationCoverage,
  TranslationStatus,
} from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

export class TranslationConflictError extends Error {
  constructor(readonly latest: ContentTranslation | null) {
    super('translation.conflict');
  }
}

@Injectable()
export class TranslationsStore {
  private readonly api = inject(ApiClient);
  private readonly i18n = inject(I18nService);
  private readonly workspace = inject(WorkspaceStore);
  private requestVersion = 0;
  private settingsLoaded = false;

  readonly locale = signal(
    this.i18n.supportedLocales.find((locale) => locale !== 'en') ?? this.i18n.supportedLocales[0],
  );
  readonly namespace = signal('');
  readonly status = signal<TranslationStatus | ''>('');
  readonly query = signal('');
  readonly items = signal<readonly ContentTranslation[]>([]);
  readonly supportedLocales = signal<readonly SupportedLocale[]>(this.i18n.supportedLocales);
  readonly defaultLocale = signal<SupportedLocale>(this.i18n.supportedLocales[0]);
  readonly coverage = signal<readonly TranslationCoverage[]>([]);
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  readonly namespaces = computed(() =>
    [
      ...new Set([
        ...this.coverage().map((item) => item.namespace),
        ...this.items().map((item) => item.namespace),
      ]),
    ].sort(),
  );
  readonly totals = computed(() =>
    this.coverage().reduce(
      (result, item) => ({
        total: result.total + item.total,
        published: result.published + item.published,
        draft: result.draft + item.draft,
        missing: result.missing + item.missing,
      }),
      { total: 0, published: 0, draft: 0, missing: 0 },
    ),
  );
  readonly completion = computed(() => {
    const totals = this.totals();
    return totals.total === 0 ? 100 : Math.round((totals.published / totals.total) * 100);
  });

  async load(reset = true): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const requestVersion = ++this.requestVersion;
    this.loading.set(true);
    this.error.set(null);
    try {
      if (reset && !this.settingsLoaded) {
        const settings = await this.api.localizationSettings(workspaceId);
        if (requestVersion !== this.requestVersion) return;
        const allowed = settings.body.supportedLocales
          .map((locale) => this.i18n.supportedLocale(locale))
          .filter((locale): locale is SupportedLocale => locale !== null);
        const supported = allowed.length ? allowed : this.i18n.supportedLocales;
        const fallbackDefault = supported[0] ?? this.i18n.supportedLocales[0];
        const requestedDefault = this.i18n.supportedLocale(settings.body.defaultLocale);
        const defaultLocale =
          requestedDefault && supported.includes(requestedDefault)
            ? requestedDefault
            : fallbackDefault;
        this.supportedLocales.set(supported);
        this.defaultLocale.set(defaultLocale);
        if (!supported.includes(this.locale())) {
          this.locale.set(supported.find((locale) => locale !== defaultLocale) ?? defaultLocale);
        }
        this.settingsLoaded = true;
      }
      const [page, coverage] = await Promise.all([
        this.api.listTranslations(workspaceId, {
          locale: this.locale(),
          namespace: this.namespace() || undefined,
          status: this.status() || undefined,
          query: this.query().trim() || undefined,
          cursor: reset ? undefined : (this.nextCursor() ?? undefined),
          limit: 50,
        }),
        reset
          ? this.api.translationCoverage(workspaceId, this.locale())
          : Promise.resolve(this.coverage()),
      ]);
      if (requestVersion !== this.requestVersion) return;
      this.items.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
      if (reset) this.coverage.set(coverage);
    } catch (error) {
      if (requestVersion === this.requestVersion) this.error.set(error);
    } finally {
      if (requestVersion === this.requestVersion) this.loading.set(false);
    }
  }

  async create(input: {
    readonly sourceLocale: SupportedLocale;
    readonly locale: SupportedLocale;
    readonly namespace: string;
    readonly key: string;
    readonly sourceText: string;
    readonly description: string;
    readonly translatedText: string;
    readonly status: 'draft' | 'published';
  }): Promise<ContentTranslation> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace.unavailable');
    this.saving.set(true);
    this.error.set(null);
    try {
      const previousLocale = this.locale();
      const response = await this.api.putTranslation(
        workspaceId,
        input.locale,
        input.namespace,
        input.key,
        0,
        {
          sourceLocale: input.sourceLocale,
          sourceText: input.sourceText,
          description: input.description,
          translatedText: input.translatedText,
          status: input.status,
          version: 0,
        },
      );
      this.locale.set(input.locale);
      if (previousLocale === input.locale) {
        this.items.update((items) => [response.body, ...items]);
      } else {
        this.items.set([response.body]);
        this.nextCursor.set(null);
      }
      this.coverage.set(await this.api.translationCoverage(workspaceId, input.locale));
      return response.body;
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async save(
    item: ContentTranslation,
    translatedText: string,
    status: 'draft' | 'published',
  ): Promise<ContentTranslation> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace.unavailable');
    const body: PutContentTranslation = {
      sourceLocale: item.sourceLocale,
      sourceText: item.sourceText,
      description: item.description,
      translatedText,
      status,
      version: item.version,
    };
    this.saving.set(true);
    this.error.set(null);
    try {
      const response = await this.api.putTranslation(
        workspaceId,
        item.locale,
        item.namespace,
        item.key,
        item.version,
        body,
      );
      this.items.update((items) =>
        items.map((candidate) =>
          sameTranslation(candidate, response.body) ? response.body : candidate,
        ),
      );
      this.coverage.set(await this.api.translationCoverage(workspaceId, this.locale()));
      return response.body;
    } catch (error) {
      if (error instanceof ApiError && error.status === 412) {
        throw new TranslationConflictError(await this.fetchLatest(item));
      }
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  private async fetchLatest(item: ContentTranslation): Promise<ContentTranslation | null> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return null;
    const page = await this.api.listTranslations(workspaceId, {
      locale: item.locale,
      namespace: item.namespace,
      query: item.key,
      limit: 50,
    });
    const latest =
      page.items.find(
        (candidate) => candidate.namespace === item.namespace && candidate.key === item.key,
      ) ?? null;
    if (latest) {
      this.items.update((items) =>
        items.map((candidate) => (sameTranslation(candidate, latest) ? latest : candidate)),
      );
    }
    return latest;
  }
}

function sameTranslation(left: ContentTranslation, right: ContentTranslation): boolean {
  return (
    left.locale === right.locale && left.namespace === right.namespace && left.key === right.key
  );
}
