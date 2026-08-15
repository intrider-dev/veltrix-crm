import { DOCUMENT } from '@angular/common';
import { computed, effect, inject, Injectable, signal } from '@angular/core';

import { openAppDatabase } from '../storage/app-database';

export type ThemePreference = 'light' | 'dark' | 'system';
export type DensityPreference = 'comfortable' | 'compact';

@Injectable({ providedIn: 'root' })
export class AppearanceStore {
  private readonly document = inject(DOCUMENT);
  private readonly media = matchMedia('(prefers-color-scheme: dark)');
  private readonly themeState = signal<ThemePreference>('system');
  private readonly densityState = signal<DensityPreference>('comfortable');
  private readonly systemDark = signal(this.media.matches);

  readonly theme = this.themeState.asReadonly();
  readonly density = this.densityState.asReadonly();
  readonly dark = computed(() => resolveDarkTheme(this.themeState(), this.systemDark()));

  constructor() {
    this.media.addEventListener('change', (event) => this.systemDark.set(event.matches));
    effect(() => {
      const dark = this.dark();
      this.document.documentElement.dataset['theme'] = dark ? 'dark' : 'light';
      this.document.documentElement.dataset['density'] = this.densityState();
      this.document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
    });
    void this.restore();
  }

  setTheme(theme: ThemePreference): void {
    this.themeState.set(theme);
    void this.persist();
  }

  setDensity(density: DensityPreference): void {
    this.densityState.set(density);
    void this.persist();
  }

  private async restore(): Promise<void> {
    if (!('indexedDB' in globalThis)) return;
    const database = await openAppDatabase();
    const value = await new Promise<
      { theme?: ThemePreference; density?: DensityPreference } | undefined
    >((resolve, reject) => {
      const request = database
        .transaction('preferences', 'readonly')
        .objectStore('preferences')
        .get('appearance');
      request.onsuccess = () =>
        resolve(
          request.result as { theme?: ThemePreference; density?: DensityPreference } | undefined,
        );
      request.onerror = () =>
        reject(request.error ?? new Error('Unable to read appearance preferences'));
    }).catch(() => undefined);
    database.close();
    if (value?.theme && ['light', 'dark', 'system'].includes(value.theme))
      this.themeState.set(value.theme);
    if (value?.density && ['comfortable', 'compact'].includes(value.density))
      this.densityState.set(value.density);
  }

  private async persist(): Promise<void> {
    if (!('indexedDB' in globalThis)) return;
    const database = await openAppDatabase();
    await new Promise<void>((resolve, reject) => {
      const transaction = database.transaction('preferences', 'readwrite');
      transaction
        .objectStore('preferences')
        .put({ theme: this.themeState(), density: this.densityState() }, 'appearance');
      transaction.oncomplete = () => resolve();
      transaction.onerror = () =>
        reject(transaction.error ?? new Error('Unable to save appearance preferences'));
    }).catch(() => undefined);
    database.close();
  }
}

export function resolveDarkTheme(theme: ThemePreference, systemDark: boolean): boolean {
  return theme === 'dark' || (theme === 'system' && systemDark);
}
