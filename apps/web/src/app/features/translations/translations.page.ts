import type { ElementRef, OnInit } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { FormField, form, required } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import type { ContentTranslation, TranslationStatus } from '../../core/api/api.types';
import type { AppMessageKey } from '../../core/i18n/app-message-key';
import { I18nService } from '../../core/i18n/i18n.service';
import { focusAfterNextRender } from '../../shared/a11y/focus-after-render';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { placeholderMismatches } from './translation-placeholders';
import { TranslationConflictError, TranslationsStore } from './translations.store';

interface TranslationEditorModel {
  translatedText: string;
  status: 'draft' | 'published';
}

interface TranslationCreateModel {
  sourceLocale: SupportedLocale;
  locale: SupportedLocale;
  namespace: string;
  key: string;
  sourceText: string;
  description: string;
  translatedText: string;
  status: 'draft' | 'published';
}

@Component({
  selector: 'app-translations-page',
  imports: [
    ErrorPanelComponent,
    FormField,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  providers: [TranslationsStore],
  templateUrl: './translations.page.html',
  styleUrl: './translations.page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TranslationsPage implements OnInit {
  readonly i18n = inject(I18nService);
  readonly store = inject(TranslationsStore);
  readonly selected = signal<ContentTranslation | null>(null);
  readonly createOpen = signal(false);
  readonly conflict = signal(false);
  readonly saved = signal(false);
  readonly targetLocales = computed(() => this.store.supportedLocales());
  readonly editor = viewChild<ElementRef<HTMLElement>>('editor');
  readonly editorModel = signal<TranslationEditorModel>({ translatedText: '', status: 'draft' });
  readonly editorForm = form(this.editorModel, (schema) => required(schema.translatedText));
  readonly createModel = signal<TranslationCreateModel>({
    sourceLocale: this.i18n.locale(),
    locale: this.i18n.supportedLocales.find((locale) => locale !== 'en') ?? 'en',
    namespace: 'crm',
    key: '',
    sourceText: '',
    description: '',
    translatedText: '',
    status: 'draft',
  });
  readonly createTargetLocales = computed(() =>
    this.store.supportedLocales().filter((locale) => locale !== this.createModel().sourceLocale),
  );
  readonly createForm = form(this.createModel, (schema) => {
    required(schema.sourceLocale);
    required(schema.locale);
    required(schema.namespace);
    required(schema.key);
    required(schema.sourceText);
    required(schema.translatedText);
  });
  readonly placeholderErrors = computed(() => {
    const item = this.selected();
    return item ? placeholderMismatches(item.placeholders, this.editorModel().translatedText) : [];
  });
  readonly asPlaceholder = (name: string) => '{' + name + '}';

  ngOnInit(): void {
    void this.initialize();
  }

  edit(item: ContentTranslation, trigger?: HTMLButtonElement): void {
    if (trigger) this.editorTrigger = trigger;
    this.selected.set(item);
    this.conflict.set(false);
    this.saved.set(false);
    this.editorModel.set({
      translatedText: item.translatedText,
      status: item.status === 'published' ? 'published' : 'draft',
    });
    focusAfterNextRender(this.injector, () => this.editor()?.nativeElement);
  }

  closeEditor(restoreFocus = true): void {
    this.selected.set(null);
    this.conflict.set(false);
    this.saved.set(false);
    if (restoreFocus) focusAfterNextRender(this.injector, () => this.editorTrigger);
  }

  resetEditor(): void {
    const item = this.selected();
    if (item) this.edit(item);
  }

  async save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    const item = this.selected();
    this.saved.set(false);
    if (!item || this.editorForm().invalid() || this.placeholderErrors().length > 0) {
      this.editorForm().markAsTouched();
      return;
    }
    try {
      const value = this.editorModel();
      const updated = await this.store.save(item, value.translatedText.trim(), value.status);
      this.selected.set(updated);
      this.conflict.set(false);
      this.saved.set(true);
    } catch (error) {
      if (error instanceof TranslationConflictError) {
        this.conflict.set(true);
        if (error.latest) {
          this.selected.set(error.latest);
          this.editorModel.set({
            translatedText: error.latest.translatedText,
            status: error.latest.status === 'published' ? 'published' : 'draft',
          });
        }
      }
    }
  }

  applyFilters(event: SubmitEvent): void {
    event.preventDefault();
    this.closeEditor(false);
    void this.store.load();
  }

  async createResource(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (this.createForm().invalid()) {
      this.createForm().markAsTouched();
      return;
    }
    const value = this.createModel();
    if (value.sourceLocale === value.locale) return;
    try {
      const created = await this.store.create({
        sourceLocale: value.sourceLocale,
        locale: value.locale,
        namespace: value.namespace.trim(),
        key: value.key.trim(),
        sourceText: value.sourceText.trim(),
        description: value.description.trim(),
        translatedText: value.translatedText.trim(),
        status: value.status,
      });
      this.store.locale.set(value.locale);
      this.createOpen.set(false);
      this.createModel.set({
        sourceLocale: value.sourceLocale,
        locale: value.locale,
        namespace: value.namespace.trim(),
        key: '',
        sourceText: '',
        description: '',
        translatedText: '',
        status: 'draft',
      });
      this.edit(created);
    } catch {
      // Store exposes a persistent localized error panel.
    }
  }

  changeLocale(locale: SupportedLocale): void {
    this.store.locale.set(locale);
    this.closeEditor(false);
    void this.store.load();
  }

  changeCreateSourceLocale(event: Event): void {
    const sourceLocale = (event.target as HTMLSelectElement).value as SupportedLocale;
    this.createModel.update((value) => ({
      ...value,
      sourceLocale,
      locale:
        value.locale === sourceLocale
          ? (this.store.supportedLocales().find((locale) => locale !== sourceLocale) ??
            sourceLocale)
          : value.locale,
    }));
  }

  changeNamespace(namespace: string): void {
    this.store.namespace.set(namespace);
  }

  changeStatus(status: TranslationStatus | ''): void {
    this.store.status.set(status);
  }

  changeQuery(event: Event): void {
    this.store.query.set((event.target as HTMLInputElement).value);
  }

  setEditorStatus(status: 'draft' | 'published'): void {
    this.editorModel.update((value) => ({ ...value, status }));
  }

  localeName(locale: SupportedLocale): string {
    return this.i18n.languageName(locale);
  }

  statusLabel(status: TranslationStatus): string {
    return this.i18n.t(('translations.' + status) as AppMessageKey);
  }

  sameItem(left: ContentTranslation | null, right: ContentTranslation): boolean {
    return (
      left?.locale === right.locale && left.namespace === right.namespace && left.key === right.key
    );
  }

  private editorTrigger: HTMLButtonElement | null = null;
  private readonly injector = inject(Injector);

  private async initialize(): Promise<void> {
    await this.store.load();
    const sourceLocale = this.store.defaultLocale();
    const locale =
      this.store.supportedLocales().find((candidate) => candidate !== sourceLocale) ?? sourceLocale;
    this.createModel.update((value) => ({ ...value, sourceLocale, locale }));
  }
}
