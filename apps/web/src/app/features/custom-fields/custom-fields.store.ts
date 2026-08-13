import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { CustomFieldDefinition, CustomFieldDefinitionInput } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class CustomFieldsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly definitions = signal<readonly CustomFieldDefinition[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.definitions.set(await this.api.listCustomFields(id));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }
  async create(body: CustomFieldDefinitionInput): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const created = await this.api.createCustomField(id, body);
      this.definitions.update((items) => [...items, created]);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }
  async remove(definition: CustomFieldDefinition): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    try {
      await this.api.deleteCustomField(id, definition);
      this.definitions.update((items) => items.filter((item) => item.id !== definition.id));
    } catch (error) {
      this.error.set(error);
    }
  }
}
