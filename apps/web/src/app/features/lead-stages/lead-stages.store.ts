import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { LeadStage, LeadStageInput } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';

@Injectable()
export class LeadStagesStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly toasts = inject(ToastService);

  readonly stages = signal<readonly LeadStage[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.stages.set(await this.api.listLeadStages(workspaceId));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async create(input: LeadStageInput): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      const created = await this.api.createLeadStage(workspaceId, input);
      this.stages.update((items) => [...items, created].sort((a, b) => a.position - b.position));
      this.toasts.show({ messageKey: 'leadStages.created', messageParams: {} });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async update(stage: LeadStage, input: LeadStageInput): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      const updated = await this.api.updateLeadStage(workspaceId, stage, input);
      this.stages.update((items) =>
        items.map((candidate) => (candidate.id === updated.id ? updated : candidate)),
      );
      this.toasts.show({ messageKey: 'leadStages.updated', messageParams: {} });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async remove(stage: LeadStage): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || stage.systemKey || this.saving()) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.api.deleteLeadStage(workspaceId, stage);
      this.stages.update((items) => items.filter((candidate) => candidate.id !== stage.id));
      this.toasts.show({ messageKey: 'leadStages.deleted', messageParams: {} });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async move(stage: LeadStage, direction: -1 | 1): Promise<void> {
    const workspaceId = this.workspace.id();
    const current = [...this.stages()];
    const index = current.findIndex((candidate) => candidate.id === stage.id);
    const destination = index + direction;
    if (!workspaceId || index < 0 || destination < 0 || destination >= current.length) return;
    [current[index], current[destination]] = [current[destination], current[index]];
    this.saving.set(true);
    this.error.set(null);
    try {
      const reordered = await this.api.reorderLeadStages(workspaceId, {
        stages: current.map((candidate) => ({ id: candidate.id, version: candidate.version })),
      });
      this.stages.set(reordered);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.saving.set(false);
    }
  }
}
