import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import { ApiError } from '../../core/api/api-error';
import type {
  Activity,
  CreateActivity,
  DealLineItem,
  DealLineItemInput,
  DealParticipant,
  DealParticipantInput,
  DealRecord,
  DealUpdateInput,
  PipelineRecord,
  StageHistoryPage,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class DealDetailsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly deal = signal<DealRecord | null>(null);
  readonly lineItems = signal<readonly DealLineItem[]>([]);
  readonly participants = signal<readonly DealParticipant[]>([]);
  readonly history = signal<StageHistoryPage['items']>([]);
  readonly activities = signal<readonly Activity[]>([]);
  readonly pipelines = signal<readonly PipelineRecord[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  readonly conflict = signal(false);
  private loadSequence = 0;

  async load(dealId: string): Promise<DealRecord | null> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return null;
    const sequence = ++this.loadSequence;
    this.saving.set(false);
    this.loading.set(true);
    this.error.set(null);
    try {
      const [deal, lineItems, participants, history, activities, pipelines] = await Promise.all([
        this.api.getDeal(workspaceId, dealId),
        this.api.listDealLineItems(workspaceId, dealId),
        this.api.listDealParticipants(workspaceId, dealId),
        this.api.listDealHistory(workspaceId, dealId),
        this.api.listActivities(workspaceId, 'deal', dealId),
        this.api.listPipelines(workspaceId),
      ]);
      if (sequence !== this.loadSequence || this.workspace.id() !== workspaceId) return null;
      this.deal.set(deal.body);
      this.lineItems.set(lineItems);
      this.participants.set(participants);
      this.history.set(history.items);
      this.activities.set(activities);
      this.pipelines.set(pipelines);
      this.conflict.set(false);
      return deal.body;
    } catch (error) {
      if (sequence === this.loadSequence && this.workspace.id() === workspaceId) {
        this.error.set(error);
      }
      return null;
    } finally {
      if (sequence === this.loadSequence && this.workspace.id() === workspaceId) {
        this.loading.set(false);
      }
    }
  }

  stageName(stageId: string | undefined): string {
    if (!stageId) return '—';
    for (const pipeline of this.pipelines()) {
      const stage = pipeline.stages.find((item) => item.id === stageId);
      if (stage) return stage.displayName;
    }
    return stageId;
  }

  async save(body: DealUpdateInput): Promise<void> {
    const workspaceId = this.workspace.id();
    const deal = this.deal();
    if (!workspaceId || !deal) return;
    const sequence = this.loadSequence;
    const updated = await this.mutate(workspaceId, deal.id, sequence, () =>
      this.api.updateDeal(workspaceId, deal.id, deal.version, body),
    );
    if (updated) this.deal.set(updated.body);
  }

  async setOutcome(status: 'open' | 'won' | 'lost', lostReason: string | null): Promise<void> {
    const workspaceId = this.workspace.id();
    const deal = this.deal();
    if (!workspaceId || !deal) return;
    const sequence = this.loadSequence;
    const updated = await this.mutate(workspaceId, deal.id, sequence, async () => {
      const forecastCategory =
        status === 'won' ? 'commit' : status === 'lost' ? 'omitted' : 'pipeline';
      return this.api.setDealOutcome(workspaceId, deal.id, deal.version, {
        status,
        lostReason,
        forecastCategory,
      });
    });
    if (updated) this.deal.set(updated.body);
  }

  async addLineItem(body: DealLineItemInput): Promise<void> {
    const workspaceId = this.workspace.id();
    const deal = this.deal();
    if (!workspaceId || !deal) return;
    const sequence = this.loadSequence;
    const result = await this.mutate(workspaceId, deal.id, sequence, () =>
      this.api.createDealLineItem(workspaceId, deal.id, deal.version, body),
    );
    if (result) {
      this.lineItems.update((items) => [...items, result.body.item]);
      this.deal.update((current) =>
        current ? { ...current, version: result.body.version } : current,
      );
    }
  }

  async addParticipant(body: DealParticipantInput): Promise<void> {
    const workspaceId = this.workspace.id();
    const deal = this.deal();
    if (!workspaceId || !deal) return;
    const sequence = this.loadSequence;
    const result = await this.mutate(workspaceId, deal.id, sequence, () =>
      this.api.upsertDealParticipant(workspaceId, deal.id, deal.version, body),
    );
    if (result) {
      this.participants.update((items) => [
        ...items.filter((item) => item.contactId !== result.body.participant.contactId),
        result.body.participant,
      ]);
      this.deal.update((current) =>
        current ? { ...current, version: result.body.version } : current,
      );
    }
  }

  async addActivity(body: CreateActivity): Promise<void> {
    const workspaceId = this.workspace.id();
    const deal = this.deal();
    if (!workspaceId || !deal) return;
    const sequence = this.loadSequence;
    const activity = await this.mutate(workspaceId, deal.id, sequence, () =>
      this.api.createActivity(workspaceId, body),
    );
    if (activity) {
      this.activities.update((items) => [activity, ...items]);
    }
  }

  async completeActivity(activity: Activity): Promise<void> {
    const workspaceId = this.workspace.id();
    const deal = this.deal();
    if (!workspaceId || !deal) return;
    const sequence = this.loadSequence;
    const completed = await this.mutate(workspaceId, deal.id, sequence, () =>
      this.api.completeActivity(workspaceId, activity),
    );
    if (completed) {
      this.activities.update((items) =>
        items.map((item) => (item.id === completed.id ? completed : item)),
      );
    }
  }

  setVersion(version: number): void {
    this.deal.update((deal) => (deal ? { ...deal, version } : deal));
  }

  private async mutate<T>(
    workspaceId: string,
    dealId: string,
    sequence: number,
    operation: () => Promise<T>,
  ): Promise<T | null> {
    this.saving.set(true);
    this.error.set(null);
    this.conflict.set(false);
    try {
      const result = await operation();
      return this.isCurrent(workspaceId, dealId, sequence) ? result : null;
    } catch (error) {
      if (this.isCurrent(workspaceId, dealId, sequence)) {
        this.error.set(error);
        if (error instanceof ApiError && error.status === 412) this.conflict.set(true);
      }
      throw error;
    } finally {
      if (this.isCurrent(workspaceId, dealId, sequence)) this.saving.set(false);
    }
  }

  private isCurrent(workspaceId: string, dealId: string, sequence: number): boolean {
    return (
      this.workspace.id() === workspaceId &&
      this.loadSequence === sequence &&
      this.deal()?.id === dealId
    );
  }
}
