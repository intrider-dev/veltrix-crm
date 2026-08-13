import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { ApiKeyItem, ApiKeyScope } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class ApiKeysStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly keys = signal<readonly ApiKeyItem[]>([]);
  readonly revealedToken = signal<string | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  async load(): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.keys.set(await this.api.listApiKeys(id));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }
  async create(name: string, scopes: readonly ApiKeyScope[]): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    this.revealedToken.set(null);
    try {
      const result = await this.api.createApiKey(id, name, scopes);
      this.keys.update((items) => [result.apiKey, ...items]);
      this.revealedToken.set(result.token);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }
  async revoke(key: ApiKeyItem): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    try {
      await this.api.revokeApiKey(id, key.id);
      this.keys.update((items) =>
        items.map((item) =>
          item.id === key.id ? { ...item, revokedAt: new Date().toISOString() } : item,
        ),
      );
    } catch (error) {
      this.error.set(error);
    }
  }
}
