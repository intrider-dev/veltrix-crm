import { describe, expect, it } from 'vitest';

import { defaultCountryForLocale } from './phone-input.component';

describe('international phone input defaults', () => {
  it('prefers the explicit browser region', () => {
    expect(defaultCountryForLocale('ru', 'ru-KZ')).toBe('kz');
    expect(defaultCountryForLocale('en', 'en-GB')).toBe('gb');
  });

  it('falls back to the active product language', () => {
    expect(defaultCountryForLocale('ru')).toBe('ru');
    expect(defaultCountryForLocale('en')).toBe('us');
    expect(defaultCountryForLocale('en', 'not_a_locale')).toBe('us');
  });
});
