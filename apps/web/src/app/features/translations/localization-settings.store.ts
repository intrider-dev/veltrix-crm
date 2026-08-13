import { inject, Injectable, signal } from '@angular/core';
import type { SupportedLocale } from '@veltrix-crm/product-config';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type { WorkspaceLocaleSettings } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class LocalizationSettingsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly settings = signal<WorkspaceLocaleSettings | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly conflict = signal(false);
  readonly saved = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<WorkspaceLocaleSettings | null> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return null;
    this.loading.set(true);
    this.error.set(null);
    this.conflict.set(false);
    try {
      const response = await this.api.localizationSettings(workspaceId);
      this.settings.set(response.body);
      return response.body;
    } catch (error) {
      this.error.set(error);
      return null;
    } finally {
      this.loading.set(false);
    }
  }

  async save(
    defaultLocale: SupportedLocale,
    supportedLocales: readonly SupportedLocale[],
  ): Promise<WorkspaceLocaleSettings | null> {
    const workspaceId = this.workspace.id();
    const current = this.settings();
    if (!workspaceId || !current) return null;
    this.saving.set(true);
    this.saved.set(false);
    this.conflict.set(false);
    this.error.set(null);
    try {
      const response = await this.api.updateLocalizationSettings(workspaceId, current.version, {
        defaultLocale,
        supportedLocales: [...supportedLocales],
      });
      this.settings.set(response.body);
      this.saved.set(true);
      return response.body;
    } catch (error) {
      if (error instanceof ApiError && error.status === 412) {
        this.conflict.set(true);
      } else {
        this.error.set(error);
      }
      return null;
    } finally {
      this.saving.set(false);
    }
  }
}
