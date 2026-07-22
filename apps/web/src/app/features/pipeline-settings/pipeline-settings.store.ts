import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  PipelineInput,
  PipelineRecord,
  PipelineStageInput,
  PipelineStageRecord,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class PipelineSettingsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly pipelines = signal<readonly PipelineRecord[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.pipelines.set(await this.api.listPipelines(workspaceId));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async createPipeline(input: PipelineInput): Promise<boolean> {
    return this.mutate(async (workspaceId) => {
      const created = await this.api.createPipeline(workspaceId, input);
      this.pipelines.update((items) => [...items, created]);
    });
  }

  async updatePipeline(pipeline: PipelineRecord, input: PipelineInput): Promise<boolean> {
    return this.mutate(async (workspaceId) => {
      const updated = await this.api.updatePipeline(workspaceId, pipeline, input);
      this.replacePipeline(updated);
    });
  }

  async deletePipeline(pipeline: PipelineRecord): Promise<boolean> {
    return this.mutate(async (workspaceId) => {
      await this.api.deletePipeline(workspaceId, pipeline);
      this.pipelines.update((items) => items.filter((item) => item.id !== pipeline.id));
    });
  }

  async createStage(pipeline: PipelineRecord, input: PipelineStageInput): Promise<boolean> {
    return this.mutate(async (workspaceId) => {
      const created = await this.api.createPipelineStage(workspaceId, pipeline.id, input);
      this.pipelines.update((items) =>
        items.map((item) =>
          item.id === pipeline.id ? { ...item, stages: [...item.stages, created] } : item,
        ),
      );
    });
  }

  async updateStage(stage: PipelineStageRecord, input: PipelineStageInput): Promise<boolean> {
    return this.mutate(async (workspaceId) => {
      const updated = await this.api.updatePipelineStage(workspaceId, stage, input);
      this.pipelines.update((items) =>
        items.map((pipeline) => ({
          ...pipeline,
          stages: pipeline.stages.map((item) => (item.id === updated.id ? updated : item)),
        })),
      );
    });
  }

  async deleteStage(stage: PipelineStageRecord): Promise<boolean> {
    return this.mutate(async (workspaceId) => {
      await this.api.deletePipelineStage(workspaceId, stage);
      this.pipelines.update((items) =>
        items.map((pipeline) => ({
          ...pipeline,
          stages: pipeline.stages.filter((item) => item.id !== stage.id),
        })),
      );
    });
  }

  async moveStage(
    pipeline: PipelineRecord,
    stage: PipelineStageRecord,
    direction: -1 | 1,
  ): Promise<void> {
    const index = pipeline.stages.findIndex((item) => item.id === stage.id);
    const destination = index + direction;
    if (index < 0 || destination < 0 || destination >= pipeline.stages.length) return;
    const reordered = [...pipeline.stages];
    [reordered[index], reordered[destination]] = [reordered[destination], reordered[index]];
    await this.mutate(async (workspaceId) => {
      const stages = await this.api.reorderPipelineStages(workspaceId, pipeline.id, {
        stages: reordered.map((item) => ({ id: item.id, version: item.version })),
      });
      this.pipelines.update((items) =>
        items.map((item) => (item.id === pipeline.id ? { ...item, stages } : item)),
      );
    });
  }

  private replacePipeline(updated: PipelineRecord): void {
    this.pipelines.update((items) =>
      items.map((item) => (item.id === updated.id ? updated : item)),
    );
  }

  private async mutate(operation: (workspaceId: string) => Promise<void>): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.saving()) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      await operation(workspaceId);
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }
}
