import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  ViewEncapsulation,
  computed,
  effect,
  inject,
  input,
  model,
  output,
  viewChild,
} from '@angular/core';
import { transformedValue, type FormValueControl } from '@angular/forms/signals';
import IntlTelInput, { intlTelInput as intlTelInputCore } from '@intl-tel-input/angular';
import type { Iso2, UiTranslations } from 'intl-tel-input';
import enTranslations from 'intl-tel-input/locale/en';
import ruTranslations from 'intl-tel-input/locale/ru';

import { I18nService } from '../../core/i18n/i18n.service';
import { runAfterNextRender } from '../a11y/focus-after-render';

interface PhoneEditorValue {
  readonly number: string;
  readonly valid: boolean;
}

interface PhoneLocaleConfiguration {
  readonly locale: 'en' | 'ru';
  readonly translations: UiTranslations;
}

let phoneInputSequence = 0;

const loadPhoneUtils = () => import('intl-tel-input/utils');
const supportedCountries = new Set<Iso2>(
  intlTelInputCore.getAllCountries().map((country) => country.iso2),
);

export function defaultCountryForLocale(locale: string, browserLocale = ''): Iso2 {
  try {
    const browserRegion = new Intl.Locale(browserLocale).region?.toLowerCase() as Iso2 | undefined;
    if (browserRegion && supportedCountries.has(browserRegion)) return browserRegion;
  } catch {
    // A malformed browser locale is untrusted client state; use the product-language fallback.
  }
  return locale.toLowerCase().startsWith('ru') ? 'ru' : 'us';
}

@Component({
  selector: 'app-phone-input',
  imports: [IntlTelInput],
  host: { class: 'veltrix-phone-field' },
  template: `
    <label class="phone-label" [for]="inputId">
      {{ label() }}
      @if (required()) {
        <span aria-hidden="true">*</span>
      }
    </label>
    @for (configuration of localeConfiguration(); track configuration.locale) {
      <intl-tel-input
        #phoneInput
        [initialValue]="editorValue().number"
        [inputAttributes]="inputAttributes()"
        [disabled]="disabled()"
        [readonly]="readOnly()"
        countrySelectorMode="AUTO"
        [countrySearch]="true"
        [formatAsYouType]="true"
        [uiTranslations]="configuration.translations"
        [countryNameLocale]="configuration.locale"
        [initialCountry]="initialCountry()"
        [loadUtils]="loadPhoneUtils"
        numberDisplayFormat="NATIONAL"
        placeholderNumberPolicy="AGGRESSIVE"
        [separateDialCode]="true"
        [showFlags]="true"
        [strictMode]="true"
        [strictRejectAnimation]="true"
        [dropdownParent]="dropdownParent"
        (numberChange)="numberChanged($event)"
        (validityChange)="validityChanged($event)"
        (blur)="touch.emit()"
      />
    }
    <p class="phone-error" [id]="errorId" [attr.aria-live]="showError() ? 'polite' : null">
      @if (showError()) {
        {{ errorMessage() }}
      }
    </p>
  `,
  styleUrl: './phone-input.component.scss',
  encapsulation: ViewEncapsulation.None,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PhoneInputComponent implements FormValueControl<string> {
  readonly value = model.required<string>();
  readonly label = input.required<string>();
  readonly disabled = input(false);
  readonly readOnly = input(false, { alias: 'readonly' });
  readonly invalid = input(false);
  readonly touched = input(false);
  readonly required = input(false);
  readonly name = input('phone');
  readonly touch = output<void>();
  readonly validityChange = output<boolean>();

  protected readonly i18n = inject(I18nService);
  protected readonly dropdownParent = inject(DOCUMENT).body;
  protected readonly inputId = `phone-input-${++phoneInputSequence}`;
  protected readonly errorId = `${this.inputId}-error`;
  protected readonly loadPhoneUtils = loadPhoneUtils;
  private readonly phoneInput = viewChild<IntlTelInput>('phoneInput');
  private readonly injector = inject(Injector);
  private lastInternalNumber = '';

  protected readonly localeConfiguration = computed<readonly PhoneLocaleConfiguration[]>(() => {
    const locale = this.i18n.locale() === 'ru' ? 'ru' : 'en';
    return [
      {
        locale,
        translations: locale === 'ru' ? ruTranslations : enTranslations,
      },
    ];
  });

  protected readonly initialCountry = computed(() =>
    defaultCountryForLocale(this.i18n.locale(), globalThis.navigator?.language ?? ''),
  );

  protected readonly editorValue = transformedValue(this.value, {
    parse: (editor: PhoneEditorValue) => {
      const number = editor.number.trim();
      if (!number) return { value: '' };
      if (!editor.valid) {
        return {
          value: number,
          error: {
            kind: 'phone',
            message: this.i18n.t('common.validation.phone'),
          },
        };
      }
      return { value: number };
    },
    format: (number): PhoneEditorValue => ({ number, valid: number.length === 0 }),
  });

  protected readonly showError = computed(() => {
    const editor = this.editorValue();
    return this.touched() && (this.invalid() || (editor.number.length > 0 && !editor.valid));
  });
  protected readonly errorMessage = computed(() =>
    this.required() && !this.value()
      ? this.i18n.t('auth.validation.required')
      : this.i18n.t('common.validation.phone'),
  );
  protected readonly inputAttributes = computed<Record<string, string>>(() => ({
    id: this.inputId,
    name: this.name(),
    inputmode: 'tel',
    autocomplete: 'tel',
    autocapitalize: 'off',
    spellcheck: 'false',
    'aria-label': this.label(),
    'aria-invalid': String(this.invalid()),
    ...(this.showError() ? { 'aria-describedby': this.errorId } : {}),
    ...(this.required() ? { required: '' } : {}),
  }));

  protected numberChanged(number: string): void {
    this.lastInternalNumber = number;
    const valid =
      number.length === 0 || (this.phoneInput()?.getInstance()?.isValidNumber() ?? false);
    this.validityChange.emit(valid);
    this.editorValue.set({ number, valid });
  }

  protected validityChanged(valid: boolean): void {
    const current = this.editorValue();
    const normalizedValidity = current.number.length === 0 || valid;
    this.editorValue.set({ number: current.number, valid: normalizedValidity });
    this.validityChange.emit(normalizedValidity);
  }

  focus(options?: FocusOptions): void {
    this.phoneInput()?.getInput().focus(options);
  }

  private readonly synchronizeExternalValue = effect(() => {
    const requestedNumber = this.editorValue().number;
    if (requestedNumber === this.lastInternalNumber) return;
    const phoneInput = this.phoneInput();
    if (!phoneInput) return;
    runAfterNextRender(this.injector, () => {
      const instance = phoneInput.getInstance();
      if (!instance) return;
      void instance.promise.then(() => {
        if (!instance.isActive()) return;
        const latestNumber = this.editorValue().number;
        if (latestNumber !== requestedNumber) return;
        const currentNumber = instance.getNumber() ?? '';
        const visibleNumber = phoneInput.getInput().value;
        if (
          currentNumber !== latestNumber &&
          (latestNumber.length > 0 || visibleNumber.length > 0)
        ) {
          instance.setNumber(latestNumber);
        }
      });
    });
  });
}
