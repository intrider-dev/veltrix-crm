import { describe, expect, it } from 'vitest';

import { resolveDarkTheme } from './appearance.store';

describe('resolveDarkTheme', () => {
  it('keeps an explicit light preference light regardless of the operating system', () => {
    expect(resolveDarkTheme('light', true)).toBe(false);
    expect(resolveDarkTheme('light', false)).toBe(false);
  });

  it('keeps an explicit dark preference dark regardless of the operating system', () => {
    expect(resolveDarkTheme('dark', true)).toBe(true);
    expect(resolveDarkTheme('dark', false)).toBe(true);
  });

  it('follows the operating system only for the system preference', () => {
    expect(resolveDarkTheme('system', true)).toBe(true);
    expect(resolveDarkTheme('system', false)).toBe(false);
  });
});
