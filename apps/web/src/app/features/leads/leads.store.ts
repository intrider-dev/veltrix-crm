import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Lead, LeadInput, LeadStage, LeadStatus } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class LeadsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private requestSequence = 0;

  readonly leads = signal<readonly Lead[]>([]);
  readonly stages = signal<readonly LeadStage[]>([]);
  readonly query = signal('');
  readonly status = signal<LeadStatus | ''>('');
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly movingLeadIds = signal<ReadonlySet<string>>(new Set());
  readonly error = signal<unknown>(null);

  async loadStages(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    try {
      this.stages.set(await this.api.listLeadStages(workspaceId));
    } catch (error) {
      this.error.set(error);
    }
  }

  async load(reset = true): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || (!reset && !this.nextCursor())) return;
    const request = ++this.requestSequence;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listLeads(workspaceId, {
        query: this.query() || undefined,
        status: this.status() || undefined,
        cursor: reset ? undefined : (this.nextCursor() ?? undefined),
        limit: 50,
      });
      if (request !== this.requestSequence) return;
      this.leads.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      if (request === this.requestSequence) this.error.set(error);
    } finally {
      if (request === this.requestSequence) this.loading.set(false);
    }
  }

  async create(body: LeadInput): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const created = await this.api.createLead(workspaceId, body);
      this.leads.update((items) => [created, ...items]);
    } catch (error) {
      this.error.set(error);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async changeStage(lead: Lead, stage: LeadStage): Promise<void> {
    const workspaceId = this.workspace.id();
    if (
      !workspaceId ||
      lead.stageId === stage.id ||
      stage.category === 'converted' ||
      this.movingLeadIds().has(lead.id)
    )
      return;
    const previous = lead;
    this.movingLeadIds.update((ids) => new Set(ids).add(lead.id));
    this.replace({ ...lead, stageId: stage.id, status: stage.category });
    try {
      this.replace(await this.api.moveLeadStage(workspaceId, lead, stage.id));
    } catch (error) {
      const current = this.leads().find((candidate) => candidate.id === lead.id);
      if (current?.version === previous.version) this.replace(previous);
      this.error.set(error);
    } finally {
      this.movingLeadIds.update((ids) => {
        const next = new Set(ids);
        next.delete(lead.id);
        return next;
      });
    }
  }

  isMoving(leadId: string): boolean {
    return this.movingLeadIds().has(leadId);
  }

  setVersion(leadId: string, version: number): void {
    this.leads.update((items) =>
      items.map((item) => (item.id === leadId ? { ...item, version } : item)),
    );
  }

  async convert(lead: Lead): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || lead.status === 'converted') return;
    this.saving.set(true);
    this.error.set(null);
    try {
      const result = await this.api.convertLead(workspaceId, lead);
      this.replace(result.lead);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.saving.set(false);
    }
  }

  private replace(lead: Lead): void {
    this.leads.update((items) => items.map((item) => (item.id === lead.id ? lead : item)));
  }
}
