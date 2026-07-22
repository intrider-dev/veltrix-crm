import { computed, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type { CreateDeal, Deal, PipelineRecord } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

export type DealViewMode = 'list' | 'kanban' | 'gantt';

const viewPreferenceKey = 'veltrix.deals.view';

@Injectable()
export class DealsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  readonly pipelines = signal<readonly PipelineRecord[]>([]);
  readonly activePipelineId = signal<string | null>(null);
  readonly deals = signal<readonly Deal[]>([]);
  readonly listDeals = signal<readonly Deal[]>([]);
  readonly listNextCursor = signal<string | null>(null);
  readonly nextCursorByStage = signal<Readonly<Record<string, string | null>>>({});
  readonly viewMode = signal<DealViewMode>(readInitialViewMode());
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  readonly conflict = signal(false);
  readonly activePipeline = computed(
    () =>
      this.pipelines().find((pipeline) => pipeline.id === this.activePipelineId()) ??
      this.pipelines()[0] ??
      null,
  );

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const pipelines = await this.api.listPipelines(workspaceId);
      this.pipelines.set(pipelines);
      const selected =
        pipelines.find((item) => item.id === this.activePipelineId()) ?? pipelines[0];
      this.activePipelineId.set(selected?.id ?? null);
      if (!selected) {
        this.deals.set([]);
        return;
      }
      await this.loadActiveView(workspaceId, selected);
      this.conflict.set(false);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async setViewMode(mode: DealViewMode): Promise<void> {
    if (this.viewMode() === mode) return;
    this.viewMode.set(mode);
    try {
      localStorage.setItem(viewPreferenceKey, mode);
    } catch {
      // A blocked preference store must not make the CRM unusable.
    }
    const workspaceId = this.workspace.id();
    const pipeline = this.activePipeline();
    if (!workspaceId || !pipeline) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      await this.loadActiveView(workspaceId, pipeline);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async loadMoreList(): Promise<void> {
    const workspaceId = this.workspace.id();
    const pipelineId = this.activePipelineId();
    const cursor = this.listNextCursor();
    if (!workspaceId || !pipelineId || !cursor || this.loading()) return;
    this.loading.set(true);
    try {
      const page = await this.api.listDeals(workspaceId, pipelineId, undefined, cursor, 50);
      const existing = new Set(this.listDeals().map((deal) => deal.id));
      this.listDeals.update((deals) => [
        ...deals,
        ...page.items.filter((deal) => !existing.has(deal.id)),
      ]);
      this.listNextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async loadMore(stageId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    const pipelineId = this.activePipelineId();
    const cursor = this.nextCursorByStage()[stageId];
    if (!workspaceId || !pipelineId || !cursor || this.loading()) return;
    this.loading.set(true);
    try {
      const page = await this.api.listDeals(workspaceId, pipelineId, stageId, cursor);
      const existing = new Set(this.deals().map((deal) => deal.id));
      this.deals.update((deals) => [
        ...deals,
        ...page.items.filter((deal) => !existing.has(deal.id)),
      ]);
      this.nextCursorByStage.update((cursors) => ({
        ...cursors,
        [stageId]: page.nextCursor ?? null,
      }));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async selectPipeline(pipelineId: string): Promise<void> {
    this.activePipelineId.set(pipelineId);
    const workspaceId = this.workspace.id();
    const pipeline = this.activePipeline();
    if (!workspaceId || !pipeline) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      await this.loadActiveView(workspaceId, pipeline);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  dealsFor(stageId: string): readonly Deal[] {
    return this.deals()
      .filter((deal) => deal.stageId === stageId)
      .sort((left, right) => left.position - right.position);
  }

  async move(dealId: string, stageId: string, position: number): Promise<void> {
    const workspaceId = this.workspace.id();
    const before = this.deals();
    const beforeList = this.listDeals();
    const deal =
      before.find((item) => item.id === dealId) ?? beforeList.find((item) => item.id === dealId);
    if (!workspaceId || !deal || (deal.stageId === stageId && deal.position === position)) return;
    this.saving.set(true);
    this.error.set(null);
    this.conflict.set(false);
    this.deals.update((deals) =>
      deals.map((item) => (item.id === dealId ? { ...item, stageId, position } : item)),
    );
    this.listDeals.update((deals) =>
      deals.map((item) => (item.id === dealId ? { ...item, stageId, position } : item)),
    );
    try {
      const updated = await this.api.moveDeal(workspaceId, dealId, deal.version, stageId, position);
      this.deals.update((deals) => deals.map((item) => (item.id === dealId ? updated : item)));
      this.listDeals.update((deals) => deals.map((item) => (item.id === dealId ? updated : item)));
    } catch (error) {
      this.deals.set(before);
      this.listDeals.set(beforeList);
      this.error.set(error);
      if (error instanceof ApiError && error.status === 412) {
        this.conflict.set(true);
        await this.load();
        this.conflict.set(true);
      }
    } finally {
      this.saving.set(false);
    }
  }

  async create(body: CreateDeal): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.saving.set(true);
    try {
      const deal = await this.api.createDeal(workspaceId, body);
      this.deals.update((deals) => [...deals, deal]);
      this.listDeals.update((deals) => [deal, ...deals.filter((item) => item.id !== deal.id)]);
    } finally {
      this.saving.set(false);
    }
  }

  private async loadActiveView(workspaceId: string, pipeline: PipelineRecord): Promise<void> {
    if (this.viewMode() === 'kanban') {
      const pages = await Promise.all(
        pipeline.stages.map((stage) => this.api.listDeals(workspaceId, pipeline.id, stage.id)),
      );
      this.deals.set(pages.flatMap((page) => page.items));
      this.nextCursorByStage.set(
        Object.fromEntries(
          pipeline.stages.map((stage, index) => [stage.id, pages[index]?.nextCursor ?? null]),
        ),
      );
      return;
    }
    const page = await this.api.listDeals(workspaceId, pipeline.id, undefined, undefined, 50);
    this.listDeals.set(page.items);
    this.listNextCursor.set(page.nextCursor ?? null);
  }
}

function readInitialViewMode(): DealViewMode {
  try {
    const value = localStorage.getItem(viewPreferenceKey);
    if (value === 'list' || value === 'kanban' || value === 'gantt') return value;
  } catch {
    // Private browsing can deny storage access; Kanban is a safe default.
  }
  return 'kanban';
}
