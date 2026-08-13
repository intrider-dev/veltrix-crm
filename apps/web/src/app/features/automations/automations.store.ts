import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { AutomationRule, AutomationRuleInput } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class AutomationsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly rules = signal<readonly AutomationRule[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.rules.set(await this.api.listAutomations(id));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async create(body: AutomationRuleInput): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const created = await this.api.createAutomation(id, body);
      this.rules.update((items) => [created, ...items]);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async toggle(rule: AutomationRule): Promise<void> {
    const id = this.workspace.id();
    if (!id) return;
    const optimistic = { ...rule, enabled: !rule.enabled };
    this.replace(optimistic);
    try {
      this.replace(await this.api.setAutomationEnabled(id, rule, !rule.enabled));
    } catch (error) {
      this.replace(rule);
      this.error.set(error);
    }
  }

  private replace(rule: AutomationRule): void {
    this.rules.update((items) => items.map((item) => (item.id === rule.id ? rule : item)));
  }
}
